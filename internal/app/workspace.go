package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	return fmt.Sprintf("file:%s?_journal=WAL", filepath.ToSlash(indexPath))
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

// RebuildIndex scans workspace markdown and rebuilds the sqlite index.
func (c *Context) RebuildIndex(now time.Time) error {
	if err := c.syncEngine.Sync(context.Background(), now); err != nil {
		return fmt.Errorf("failed to sync index: %w", err)
	}
	return nil
}

func (c *Context) GetTask(id string) (*domain.Task, error) {
	return c.taskRepo.Get(id)
}

func (c *Context) GetProject(id string) (*domain.Project, error) {
	return c.projectRepo.Get(id)
}

func (c *Context) WorkspaceDir() string { return c.workspaceDir }
func (c *Context) IndexPath() string    { return c.indexPath }
