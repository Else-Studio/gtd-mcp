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

		areaID, _ := cmd.Flags().GetString("area-id")
		areaName, _ := cmd.Flags().GetString("area")

		if areaName != "" && areaID == "" {
			// Find or create area
			now := time.Now()
			areas, _ := appCtx.areaRepo.List()
			var found *domain.Area
			for _, a := range areas {
				if a.Name == areaName && a.DeletedAt == nil {
					found = a
					break
				}
			}
			if found == nil {
				found = &domain.Area{
					ID:        uuid.New().String(),
					Name:      areaName,
					CreatedAt: now,
					UpdatedAt: now,
				}
				appCtx.areaRepo.Save(found)
				appCtx.syncEngine.SyncArea(context.Background(), found)
			}
			areaID = found.ID
		}

		if areaID != "" {
			project.AreaID = &areaID
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

		areaID, _ := cmd.Flags().GetString("area-id")
		areaName, _ := cmd.Flags().GetString("area")

		if areaName != "" && areaID == "" {
			now := time.Now()
			areas, _ := appCtx.areaRepo.List()
			var found *domain.Area
			for _, a := range areas {
				if a.Name == areaName && a.DeletedAt == nil {
					found = a
					break
				}
			}
			if found == nil {
				found = &domain.Area{
					ID:        uuid.New().String(),
					Name:      areaName,
					CreatedAt: now,
					UpdatedAt: now,
				}
				appCtx.areaRepo.Save(found)
				appCtx.syncEngine.SyncArea(context.Background(), found)
			}
			areaID = found.ID
		}

		if cmd.Flags().Changed("area-id") || areaID != "" {
			if areaID == "" {
				project.AreaID = nil
			} else {
				project.AreaID = &areaID
			}
			project.UpdatedAt = time.Now()
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

		tasks, err := appCtx.taskRepo.List()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		now := time.Now()
		project.SoftDelete(now, tasks)

		if err := appCtx.projectRepo.Save(project); err != nil {
			return fmt.Errorf("save project: %w", err)
		}

		if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
			return fmt.Errorf("sync project: %w", err)
		}

		// Persist cascade soft-deletes on child tasks.
		for _, t := range tasks {
			if t.ProjectID != nil && *t.ProjectID == project.ID {
				if err := appCtx.taskRepo.Save(t); err != nil {
					return fmt.Errorf("save cascaded task: %w", err)
				}
				if err := appCtx.syncEngine.SyncTask(context.Background(), t, now); err != nil {
					return fmt.Errorf("sync cascaded task: %w", err)
				}
			}
		}

		printSuccess(project)
		return nil
	},
}

var projectRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a project",
	Long: `Restores a soft-deleted project by ID, and cascades the restoration to child tasks.`,
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

		tasks, err := appCtx.taskRepo.List()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		now := time.Now()
		project.Restore(now, tasks)

		if err := appCtx.projectRepo.Save(project); err != nil {
			return fmt.Errorf("save project: %w", err)
		}
		if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
			return fmt.Errorf("sync project: %w", err)
		}

		for _, t := range tasks {
			if t.ProjectID != nil && *t.ProjectID == project.ID {
				appCtx.taskRepo.Save(t)
				appCtx.syncEngine.SyncTask(context.Background(), t, now)
			}
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
	projectAddCmd.Flags().String("area-id", "", "Area ID to associate with the project")
	projectAddCmd.Flags().String("area", "", "Area Name to associate with the project (creates if doesn't exist)")

	projectUpdateCmd.Flags().String("status", "", "Status of the project")
	projectUpdateCmd.Flags().String("area-id", "", "Area ID to associate with the project")
	projectUpdateCmd.Flags().String("area", "", "Area Name to associate with the project (creates if doesn't exist)")

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectRestoreCmd)
	projectCmd.AddCommand(projectListCmd)

	rootCmd.AddCommand(projectCmd)
}
