package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var sectionCmd = &cobra.Command{
	Use:   "section",
	Short: "Manage sections",
	Long: `Manage sections within projects.
Sections represent mid-level groupings of tasks inside a project container.`,
}


var sectionAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new section",
	Long: `Creates a new project section.
Requires specifying the parent project ID via the --project flag. Returns the JSON section representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		projectId, _ := cmd.Flags().GetString("project")
		if projectId == "" {
			return fmt.Errorf("--project is required")
		}

		section := &domain.Section{
			ID:        uuid.New().String(),
			Title:     title,
			ProjectID: projectId,
		}

		if err := appCtx.sectionRepo.Save(section); err != nil {
			return fmt.Errorf("save section: %w", err)
		}

		if err := appCtx.syncEngine.SyncSection(context.Background(), section); err != nil {
			return fmt.Errorf("sync section: %w", err)
		}

		printSuccess(section)
		return nil
	},
}

var sectionDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a section",
	Long: `Soft-deletes a project section by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		section, err := appCtx.sectionRepo.Get(id)
		if err != nil {
			return fmt.Errorf("section not found: %w", err)
		}

		now := time.Now()
		section.SoftDelete(now)

		if err := appCtx.sectionRepo.Save(section); err != nil {
			return fmt.Errorf("save section: %w", err)
		}

		if err := appCtx.syncEngine.SyncSection(context.Background(), section); err != nil {
			return fmt.Errorf("sync section: %w", err)
		}

		printSuccess(section)
		return nil
	},
}

var sectionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sections",
	Long: `Lists all active project section IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		sections, err := appCtx.sectionRepo.List()
		if err != nil {
			return fmt.Errorf("list sections: %w", err)
		}

		activeSections := make([]*domain.Section, 0)
		for _, s := range sections {
			if s.DeletedAt == nil {
				activeSections = append(activeSections, s)
			}
		}

		printSuccess(activeSections)
		return nil
	},
}

func init() {
	sectionAddCmd.Flags().String("project", "", "Project ID")
	sectionCmd.AddCommand(sectionAddCmd)
	sectionCmd.AddCommand(sectionDeleteCmd)
	sectionCmd.AddCommand(sectionListCmd)

	rootCmd.AddCommand(sectionCmd)
}
