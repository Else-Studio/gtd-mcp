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

		now := time.Now()
		area.SoftDelete(now)

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

func init() {
	areaCmd.AddCommand(areaAddCmd)
	areaCmd.AddCommand(areaDeleteCmd)
	areaCmd.AddCommand(areaListCmd)

	rootCmd.AddCommand(areaCmd)
}
