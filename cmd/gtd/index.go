package main

import (
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gtd/internal/app"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage the search index",
}

var indexRebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild the sqlite index from disk",
	Long: `Rebuilds the sqlite database index.db by scanning all active Markdown files in the workspace directories (tasks, projects, areas, people).
Run this command at the start of a session or during a Weekly Review to ensure data consistency after external file edits.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gtdDir, err := getWorkspaceDir()
		if err != nil {
			return err
		}

		appCtx, err := app.Open(gtdDir, filepath.Join(gtdDir, "index.db"))
		if err != nil {
			return err
		}
		defer appCtx.Close()

		result, err := appCtx.RebuildIndex(time.Now())
		if err != nil {
			return err
		}

		printSuccess(map[string]interface{}{
			"message":          "Index rebuilt successfully",
			"indexed":          result.Indexed,
			"skippedConflicts": result.SkippedConflicts,
			"errors":           result.Errors,
		})
		return nil
	},
}

func init() {
	indexCmd.AddCommand(indexRebuildCmd)
	rootCmd.AddCommand(indexCmd)
}
