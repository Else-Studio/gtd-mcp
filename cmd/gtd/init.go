package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gtd/internal/persistence/sqlite"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstraps the workspace",
	Long: `Initializes the local GTD workspace.
Creates the directory structures for tasks, projects, areas, sections, people, and saved filters under the GTD workspace (defaults to ~/.gtd or GTD_DIR environment variable).
Also initializes an empty config.yml file and the sqlite database index.db.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gtdDir, err := getWorkspaceDir()
		if err != nil {
			return err
		}

		dirs := []string{
			"tasks",
			"projects",
			"areas",
			"people",
		}

		for _, d := range dirs {
			dirPath := filepath.Join(gtdDir, d)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", d, err)
			}
		}

		configFile := filepath.Join(gtdDir, "config.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
		}

		dbFile := filepath.Join(gtdDir, "index.db")
		dsn := fmt.Sprintf("file:%s?_journal=WAL", filepath.ToSlash(dbFile))
		db, err := sqlite.NewDB(dsn)
		if err != nil {
			return fmt.Errorf("failed to initialize sqlite db: %w", err)
		}
		defer db.Close()

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
