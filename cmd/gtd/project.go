package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long: `Manage outcomes requiring multiple steps (projects).
Supports creating, updating, listing, and soft-deleting projects. Defaults to JSON, but --plain overrides with a readable project table.`,
}

var projectAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new project",
	Long: `Creates a new active project with the specified title.
The project is initialized in the 'active' status. Returns the JSON project representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		project := &domain.Project{
			ID:     uuid.New().String(),
			Title:  title,
			Status: domain.ProjectStatusActive,
		}

		if err := appCtx.projectRepo.Save(project); err != nil {
			return fmt.Errorf("save project: %w", err)
		}

		if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
			return fmt.Errorf("sync project: %w", err)
		}

		printSuccess(project)
		return nil
	},
}


var projectUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a project",
	Long: `Updates project metadata by ID.
The --status flag allows changing the status of the project (e.g. active, someday, completed, archived).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		project, err := appCtx.projectRepo.Get(id)
		if err != nil {
			return fmt.Errorf("project not found: %w", err)
		}

		status, _ := cmd.Flags().GetString("status")
		if status != "" {
			project.UpdateStatus(domain.ProjectStatus(status), time.Now())
		}

		if err := appCtx.projectRepo.Save(project); err != nil {
			return fmt.Errorf("save project: %w", err)
		}

		if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
			return fmt.Errorf("sync project: %w", err)
		}

		printSuccess(project)
		return nil
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a project",
	Long: `Soft-deletes a project by ID.
Soft-deleted projects are hidden from normal list views.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		project, err := appCtx.projectRepo.Get(id)
		if err != nil {
			return fmt.Errorf("project not found: %w", err)
		}

		now := time.Now()
		project.SoftDelete(now, nil, nil)

		if err := appCtx.projectRepo.Save(project); err != nil {
			return fmt.Errorf("save project: %w", err)
		}

		if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
			return fmt.Errorf("sync project: %w", err)
		}

		printSuccess(project)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `Lists all active project IDs.
Returns a JSON list of IDs. When --plain is specified, fetches and outputs a detailed ASCII project table (ID, Title, Status, Area).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		// For stage 7, listing files via fs repo is simplest and sufficient if we don't have sqlite list method.
		projects, err := appCtx.projectRepo.List()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}

		ids := []string{}
		for _, p := range projects {
			if p.DeletedAt == nil {
				ids = append(ids, p.ID)
			}
		}

		printSuccess(resolveProjects(appCtx, ids))
		return nil
	},
}

func init() {
	projectUpdateCmd.Flags().String("status", "", "Status of the project")

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectListCmd)

	rootCmd.AddCommand(projectCmd)
}
