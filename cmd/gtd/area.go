package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var areaCmd = &cobra.Command{
	Use:   "area",
	Short: "Manage areas",
	Long: `Manage ongoing Areas of Focus (e.g. Finances, Work, Health).
Areas represent permanent categories of work and responsibility.`,
}

var areaAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new area",
	Long: `Creates a new Area of Focus with the specified name. Returns the JSON area representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area := &domain.Area{
			ID:   uuid.New().String(),
			Name: name,
		}

		if err := appCtx.areaRepo.Save(area); err != nil {
			return fmt.Errorf("save area: %w", err)
		}

		if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
			return fmt.Errorf("sync area: %w", err)
		}

		printSuccess(area)
		return nil
	},
}

var areaDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an area",
	Long: `Soft-deletes an Area of Focus by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.areaRepo.Get(id)
		if err != nil {
			return fmt.Errorf("area not found: %w", err)
		}

		projects, err := appCtx.projectRepo.List()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		tasks, err := appCtx.taskRepo.List()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		now := time.Now()
		area.SoftDelete(now, projects, tasks)

		if err := appCtx.areaRepo.Save(area); err != nil {
			return fmt.Errorf("save area: %w", err)
		}

		if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
			return fmt.Errorf("sync area: %w", err)
		}

		// Persist cascade soft-deletes on child projects and tasks.
		cascadedProjects := map[string]bool{}
		for _, p := range projects {
			if p.AreaID != nil && *p.AreaID == area.ID {
				cascadedProjects[p.ID] = true
				if err := appCtx.projectRepo.Save(p); err != nil {
					return fmt.Errorf("save cascaded project: %w", err)
				}
				if err := appCtx.syncEngine.SyncProject(context.Background(), p); err != nil {
					return fmt.Errorf("sync cascaded project: %w", err)
				}
			}
		}
		for _, t := range tasks {
			underArea := t.AreaID != nil && *t.AreaID == area.ID
			underCascadedProject := t.ProjectID != nil && cascadedProjects[*t.ProjectID]
			if !underArea && !underCascadedProject {
				continue
			}
			if err := appCtx.taskRepo.Save(t); err != nil {
				return fmt.Errorf("save cascaded task: %w", err)
			}
			if err := appCtx.syncEngine.SyncTask(context.Background(), t, now); err != nil {
				return fmt.Errorf("sync cascaded task: %w", err)
			}
		}

		printSuccess(area)
		return nil
	},
}

var areaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List areas",
	Long: `Lists all active Area of Focus IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		areas, err := appCtx.areaRepo.List()
		if err != nil {
			return fmt.Errorf("list areas: %w", err)
		}

		activeAreas := make([]*domain.Area, 0)
		for _, a := range areas {
			if a.DeletedAt == nil {
				activeAreas = append(activeAreas, a)
			}
		}

		printSuccess(activeAreas)
		return nil
	},
}

var areaUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an area",
	Long: `Updates an Area of Focus by ID. Use --name to change its name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.areaRepo.Get(id)
		if err != nil {
			return fmt.Errorf("area not found: %w", err)
		}

		name, _ := cmd.Flags().GetString("name")
		if name != "" {
			area.Name = name
			area.UpdatedAt = time.Now()
		}

		if err := appCtx.areaRepo.Save(area); err != nil {
			return fmt.Errorf("save area: %w", err)
		}

		if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
			return fmt.Errorf("sync area: %w", err)
		}

		printSuccess(area)
		return nil
	},
}

var areaRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore an area",
	Long: `Restores a soft-deleted Area of Focus by ID, and cascades the restoration to child projects and tasks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.areaRepo.Get(id)
		if err != nil {
			return fmt.Errorf("area not found: %w", err)
		}

		projects, err := appCtx.projectRepo.List()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		tasks, err := appCtx.taskRepo.List()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		now := time.Now()
		area.Restore(now, projects, tasks)

		if err := appCtx.areaRepo.Save(area); err != nil {
			return fmt.Errorf("save area: %w", err)
		}
		if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
			return fmt.Errorf("sync area: %w", err)
		}

		cascadedProjects := map[string]bool{}
		for _, p := range projects {
			if p.AreaID != nil && *p.AreaID == area.ID {
				cascadedProjects[p.ID] = true
				if err := appCtx.projectRepo.Save(p); err != nil {
					return fmt.Errorf("save cascaded project: %w", err)
				}
				if err := appCtx.syncEngine.SyncProject(context.Background(), p); err != nil {
					return fmt.Errorf("sync cascaded project: %w", err)
				}
			}
		}
		for _, t := range tasks {
			underArea := t.AreaID != nil && *t.AreaID == area.ID
			underCascadedProject := t.ProjectID != nil && cascadedProjects[*t.ProjectID]
			if !underArea && !underCascadedProject {
				continue
			}
			if err := appCtx.taskRepo.Save(t); err != nil {
				return fmt.Errorf("save cascaded task: %w", err)
			}
			if err := appCtx.syncEngine.SyncTask(context.Background(), t, now); err != nil {
				return fmt.Errorf("sync cascaded task: %w", err)
			}
		}

		printSuccess(area)
		return nil
	},
}

func init() {
	areaUpdateCmd.Flags().String("name", "", "New name for the area")

	areaCmd.AddCommand(areaAddCmd)
	areaCmd.AddCommand(areaUpdateCmd)
	areaCmd.AddCommand(areaDeleteCmd)
	areaCmd.AddCommand(areaRestoreCmd)
	areaCmd.AddCommand(areaListCmd)

	rootCmd.AddCommand(areaCmd)
}
