package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
	"gtd/internal/persistence/sqlite"
)

// Context wires markdown repos (source of truth) and the SQLite read index.
// All entity writes go through Persist* (file first, then Sync). See persist.go.
//
// workspaceDir is the folder with tasks/projects/areas/people.
// indexPath is the sqlite file; it may live outside the workspace (Android
// keeps it in app-private storage). The CLI uses workspaceDir/index.db.
type Context struct {
	workspaceDir string
	indexPath    string

	db          *sql.DB
	syncEngine  *sqlite.SyncEngine
	taskQuery   *sqlite.TaskQuery
	taskRepo    domain.TaskRepository
	projectRepo domain.ProjectRepository
	areaRepo    domain.AreaRepository
	personRepo  domain.PersonRepository
}

func indexDSN(indexPath string) string {
	return indexDSNFor(runtime.GOOS, indexPath)
}

func indexDSNFor(goos, indexPath string) string {
	path := filepath.ToSlash(indexPath)
	if goos == "android" {
		// ncruces ignores mattn-style `_journal=WAL`. WAL + pooled connections
		// on gomobile/dotlk produces "cannot commit - no transaction is active".
		// DELETE journal + one conn is the phone path. Do not use EXCLUSIVE
		// locking: a discarded conn after a failed COMMIT can block the next write.
		return "file:" + path + "?_pragma=journal_mode(DELETE)&_txlock=immediate"
	}
	return "file:" + path + "?_journal=WAL"
}

// Init creates workspace subdirectories, an empty config.yml if missing, and
// opens/creates the sqlite index at indexPath (parent dir is created).
func Init(workspaceDir, indexPath string) error {
	for _, d := range []string{"tasks", "projects", "areas", "people"} {
		dirPath := filepath.Join(workspaceDir, d)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	configFile := filepath.Join(workspaceDir, "config.yml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	db, err := sqlite.NewDB(indexDSN(indexPath))
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite db: %w", err)
	}
	return db.Close()
}

// Open connects to an existing workspace and index. It does not create folders
// (call Init first). The index file is created if missing.
func Open(workspaceDir, indexPath string) (*Context, error) {
	db, err := sqlite.NewDB(indexDSN(indexPath))
	if err != nil {
		return nil, err
	}

	taskRepo := fs.NewTaskRepository(filepath.Join(workspaceDir, "tasks"))
	projectRepo := fs.NewProjectRepository(filepath.Join(workspaceDir, "projects"))
	areaRepo := fs.NewAreaRepository(filepath.Join(workspaceDir, "areas"))
	personRepo := fs.NewPersonRepository(filepath.Join(workspaceDir, "people"))

	return &Context{
		workspaceDir: workspaceDir,
		indexPath:    indexPath,
		db:           db,
		syncEngine:   sqlite.NewSyncEngine(db, taskRepo, projectRepo, areaRepo, personRepo),
		taskQuery:    sqlite.NewTaskQuery(db),
		taskRepo:     taskRepo,
		projectRepo:  projectRepo,
		areaRepo:     areaRepo,
		personRepo:   personRepo,
	}, nil
}

// Close releases the index database.
func (c *Context) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}

// SetIndexMaxOpenConns caps the index pool. Call after Open; not set in NewDB.
func (c *Context) SetIndexMaxOpenConns(n int) {
	if c == nil || c.db == nil {
		return
	}
	c.db.SetMaxOpenConns(n)
}

type RebuildResult struct {
	RebuiltAt        time.Time
	Indexed          int
	SkippedConflicts []string
	Errors           []string
}

// RebuildIndex scans workspace markdown and rebuilds the sqlite index.
func (c *Context) RebuildIndex(now time.Time) (*RebuildResult, error) {
	report, err := c.syncEngine.Sync(context.Background(), now)
	if err != nil {
		return nil, fmt.Errorf("failed to sync index: %w", err)
	}

	var indexed int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&indexed); err != nil {
		return nil, fmt.Errorf("failed to count indexed tasks: %w", err)
	}

	skipped := []string{}
	errs := []string{}
	if report != nil {
		skipped = uniqueStrings(report.SkippedConflicts)
		if report.Errors != nil {
			errs = report.Errors
		}
	}

	_, _ = c.db.Exec(`INSERT INTO index_meta (key, value) VALUES ('last_rebuilt_at', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, now.UTC().Format(time.RFC3339Nano))

	return &RebuildResult{
		RebuiltAt:        now.UTC(),
		Indexed:          indexed,
		SkippedConflicts: skipped,
		Errors:           errs,
	}, nil
}

// GetLastRebuiltAt returns the timestamp of the last index rebuild, or zero time if never rebuilt.
func (c *Context) GetLastRebuiltAt() (time.Time, error) {
	if c == nil || c.db == nil {
		return time.Time{}, nil
	}
	var val string
	err := c.db.QueryRow(`SELECT value FROM index_meta WHERE key = 'last_rebuilt_at'`).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, nil
	}
	t, parseErr := time.Parse(time.RFC3339Nano, val)
	if parseErr != nil {
		return time.Time{}, nil
	}
	return t, nil
}

// EnsureFresh checks if the index was rebuilt within maxAge. If not (or never rebuilt),
// it executes RebuildIndex(now). Returns true if a rebuild was performed.
func (c *Context) EnsureFresh(now time.Time, maxAge time.Duration) (bool, error) {
	if c == nil || c.db == nil || maxAge <= 0 {
		return false, nil
	}
	lastRebuilt, err := c.GetLastRebuiltAt()
	if err != nil {
		return false, err
	}
	if lastRebuilt.IsZero() || now.Sub(lastRebuilt) >= maxAge {
		_, err := c.RebuildIndex(now)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (c *Context) GetTask(id string) (*domain.Task, error) {
	return c.taskRepo.Get(id)
}

func (c *Context) GetProject(id string) (*domain.Project, error) {
	return c.projectRepo.Get(id)
}

func (c *Context) WorkspaceDir() string { return c.workspaceDir }
func (c *Context) IndexPath() string    { return c.indexPath }
