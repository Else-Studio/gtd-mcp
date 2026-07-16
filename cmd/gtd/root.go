package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
	"gtd/internal/persistence/sqlite"

	"github.com/spf13/cobra"
)

var PlainOutput bool

type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
}

type JSONError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var rootCmd = &cobra.Command{
	Use:           "gtd",
	Short:         "AI-First GTD CLI",
	Long: `The AI-First GTD CLI is a command-line interface designed to implement the David Allen Getting Things Done (GTD) framework.
It is built primarily for integration with AI Agents (e.g. MCP Servers) but supports human-readable outputs.

Design Philosophy:
1. JSON-first output format by default to allow reliable programmatic parsing.
2. Global flag '--plain' to toggle tabular plain text rendering (powered by tabwriter).
3. Explicit sqlite indexing supporting high-performance querying and offline sync.
4. Integrated date coherence validation surfacing warning flags without auto-correcting user inputs.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func getWorkspaceDir() (string, error) {
	dir := os.Getenv("GTD_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve user home directory: %w", err)
		}
		dir = filepath.Join(home, ".gtd")
	}
	return dir, nil
}

type appContext struct {
	db          *sql.DB
	syncEngine  *sqlite.SyncEngine
	taskQuery   *sqlite.TaskQuery
	taskRepo    *fs.TaskRepository
	projectRepo *fs.ProjectRepository
	areaRepo    *fs.AreaRepository
	personRepo  *fs.PersonRepository
	cleanup     func()
}

func getAppContext() (*appContext, error) {
	wsDir, err := getWorkspaceDir()
	if err != nil {
		return nil, err
	}
	dbFile := filepath.Join(wsDir, "index.db")
	dsn := fmt.Sprintf("file:%s?_journal=WAL", filepath.ToSlash(dbFile))
	db, err := sqlite.NewDB(dsn)
	if err != nil {
		return nil, err
	}

	taskRepo := fs.NewTaskRepository(filepath.Join(wsDir, "tasks"))
	projectRepo := fs.NewProjectRepository(filepath.Join(wsDir, "projects"))
	areaRepo := fs.NewAreaRepository(filepath.Join(wsDir, "areas"))
	personRepo := fs.NewPersonRepository(filepath.Join(wsDir, "people"))

	syncEngine := sqlite.NewSyncEngine(db, taskRepo, projectRepo, areaRepo, personRepo)
	taskQuery := sqlite.NewTaskQuery(db)

	cleanup := func() {
		db.Close()
	}

	return &appContext{
		db:          db,
		syncEngine:  syncEngine,
		taskQuery:   taskQuery,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		areaRepo:    areaRepo,
		personRepo:  personRepo,
		cleanup:     cleanup,
	}, nil
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&PlainOutput, "plain", false, "Output plain text instead of JSON")
}

type TaskOutput struct {
	*domain.Task
	Warnings []string `json:"warnings,omitempty"`
}

func formatOutputData(data interface{}) interface{} {
	switch v := data.(type) {
	case *domain.Task:
		warnings := domain.ValidateTaskCoherence(v)
		if len(warnings) > 0 {
			return &TaskOutput{Task: v, Warnings: warnings}
		}
		return v
	case []*domain.Task:
		var out []interface{}
		for _, t := range v {
			warnings := domain.ValidateTaskCoherence(t)
			if len(warnings) > 0 {
				out = append(out, &TaskOutput{Task: t, Warnings: warnings})
			} else {
				out = append(out, t)
			}
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{})
		for k, val := range v {
			out[k] = formatOutputData(val)
		}
		return out
	case []interface{}:
		var out []interface{}
		for _, item := range v {
			out = append(out, formatOutputData(item))
		}
		return out
	default:
		return v
	}
}

func resolveTasks(appCtx *appContext, ids []string) interface{} {
	tasks := make([]*domain.Task, 0)
	for _, id := range ids {
		if t, err := appCtx.taskRepo.Get(id); err == nil {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func resolveProjects(appCtx *appContext, ids []string) interface{} {
	projects := make([]*domain.Project, 0)
	for _, id := range ids {
		if p, err := appCtx.projectRepo.Get(id); err == nil {
			projects = append(projects, p)
		}
	}
	return projects
}

func printPlainOutput(data interface{}) {
	if m, ok := data.(map[string]interface{}); ok {
		if t, ok := m["task"]; ok {
			data = t
		} else if lst, ok := m["tasks"]; ok {
			data = lst
		} else if p, ok := m["project"]; ok {
			data = p
		}
	}

	switch v := data.(type) {
	case *domain.Task, *TaskOutput:
		var t *domain.Task
		if to, ok := v.(*TaskOutput); ok {
			t = to.Task
		} else {
			t = v.(*domain.Task)
		}
		printTaskTable([]*domain.Task{t})
	case []*domain.Task:
		printTaskTable(v)
	case []interface{}:
		var tasks []*domain.Task
		for _, item := range v {
			if t, ok := item.(*domain.Task); ok {
				tasks = append(tasks, t)
			} else if to, ok := item.(*TaskOutput); ok {
				tasks = append(tasks, to.Task)
			}
		}
		if len(tasks) > 0 {
			printTaskTable(tasks)
		} else {
			fmt.Printf("%+v\n", v)
		}
	case *domain.Project:
		printProjectTable([]*domain.Project{v})
	case []*domain.Project:
		printProjectTable(v)
	default:
		fmt.Printf("%+v\n", v)
	}
}

func printTaskTable(tasks []*domain.Task) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tDUE\tPROJECT\tWARNINGS")
	for _, t := range tasks {
		dueStr := "-"
		if t.DueDate != nil {
			dueStr = t.DueDate.Format("2006-01-02")
		}
		projStr := "-"
		if t.ProjectID != nil {
			projStr = *t.ProjectID
		}
		warnings := strings.Join(domain.ValidateTaskCoherence(t), ",")
		if warnings == "" {
			warnings = "-"
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Title, t.Status, dueStr, projStr, warnings)
	}
	w.Flush()
}

func printProjectTable(projects []*domain.Project) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tAREA")
	for _, p := range projects {
		areaStr := "-"
		if p.AreaID != nil {
			areaStr = *p.AreaID
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Title, p.Status, areaStr)
	}
	w.Flush()
}

func printSuccess(data interface{}) {
	formattedData := formatOutputData(data)
	if PlainOutput {
		printPlainOutput(formattedData)
		return
	}
	resp := JSONResponse{
		Success: true,
		Data:    formattedData,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON response: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

func printError(err error) {
	if PlainOutput {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		return
	}
	
	code := "ERR_UNKNOWN"
	if errors.Is(err, domain.ErrNotFound) {
		code = "ERR_NOT_FOUND"
	} else if errors.Is(err, domain.ErrValidation) {
		code = "ERR_VALIDATION"
	}

	resp := JSONResponse{
		Success: false,
		Error: &JSONError{
			Code:    code,
			Message: err.Error(),
		},
	}
	b, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON response: %v\nOriginal error: %v\n", marshalErr, err)
		return
	}
	fmt.Println(string(b))
}
