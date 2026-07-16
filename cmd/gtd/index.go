package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gtd/internal/persistence/fs"
	"gtd/internal/persistence/sqlite"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage the search index",
}

var indexRebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild the sqlite index from disk",
	Long: `Rebuilds the sqlite database index.db by scanning all active Markdown files in the workspace directories (tasks, projects, areas, sections, people, saved_filters).
Run this command at the start of a session or during a Weekly Review to ensure data consistency.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gtdDir, err := getWorkspaceDir()
		if err != nil {
			return err
		}

		dbFile := filepath.Join(gtdDir, "index.db")
		dsn := fmt.Sprintf("file:%s?_journal=WAL", filepath.ToSlash(dbFile))
		db, err := sqlite.NewDB(dsn)
		if err != nil {
			return fmt.Errorf("failed to open sqlite db: %w", err)
		}
		defer db.Close()

		taskRepo := fs.NewTaskRepository(filepath.Join(gtdDir, "tasks"))
		projectRepo := fs.NewProjectRepository(filepath.Join(gtdDir, "projects"))
		areaRepo := fs.NewAreaRepository(filepath.Join(gtdDir, "areas"))
		personRepo := fs.NewPersonRepository(filepath.Join(gtdDir, "people"))

		syncEngine := sqlite.NewSyncEngine(db, taskRepo, projectRepo, areaRepo, personRepo)
		
		ctx := context.Background()
		now := time.Now()
		
		if err := syncEngine.Sync(ctx, now); err != nil {
			return fmt.Errorf("failed to sync index: %w", err)
		}

		printSuccess(map[string]string{
			"message": "Index rebuilt successfully",
		})
		return nil
	},
}

func init() {
	indexCmd.AddCommand(indexRebuildCmd)
	rootCmd.AddCommand(indexCmd)
}
