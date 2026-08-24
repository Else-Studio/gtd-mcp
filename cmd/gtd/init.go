package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"gtd/internal/app"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstraps the workspace",
	Long: `Initializes the local GTD workspace.
Creates the directory structures for tasks, projects, areas, and people under the GTD workspace (defaults to ~/.gtd or GTD_DIR environment variable).
Also creates an empty config.yml placeholder (not loaded in V1 — effective config is GTD_DIR only) and the sqlite database index.db.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gtdDir, err := getWorkspaceDir()
		if err != nil {
			return err
		}

		if err := app.Init(gtdDir, filepath.Join(gtdDir, "index.db")); err != nil {
			return err
		}

		printSuccess(map[string]string{
			"workspace": gtdDir,
			"message":   "Workspace initialized",
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
