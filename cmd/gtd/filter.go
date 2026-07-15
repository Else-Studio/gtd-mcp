package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var filterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Manage saved filters",
	Long: `Manage saved custom view filters.
Filters persist query criteria (e.g. status next contexts @computer) for fast execution.`,
}


var filterAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new saved filter",
	Long: `Creates a new saved filter with the specified name.
Requires specifying --view (e.g. list, board) and --criteria (query filter criteria) flags. Returns the JSON filter representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		view, _ := cmd.Flags().GetString("view")
		criteria, _ := cmd.Flags().GetString("criteria")

		if view == "" || criteria == "" {
			return fmt.Errorf("--view and --criteria are required")
		}

		filter := &domain.SavedFilter{
			ID:       uuid.New().String(),
			Name:     name,
			View:     view,
			Criteria: criteria,
		}

		if err := appCtx.filterRepo.Save(filter); err != nil {
			return fmt.Errorf("save filter: %w", err)
		}

		if err := appCtx.syncEngine.SyncSavedFilter(context.Background(), filter); err != nil {
			return fmt.Errorf("sync filter: %w", err)
		}

		printSuccess(filter)
		return nil
	},
}

var filterDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved filter",
	Long: `Soft-deletes a saved custom filter by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		filter, err := appCtx.filterRepo.Get(id)
		if err != nil {
			return fmt.Errorf("filter not found: %w", err)
		}

		now := time.Now()
		filter.SoftDelete(now)

		if err := appCtx.filterRepo.Save(filter); err != nil {
			return fmt.Errorf("save filter: %w", err)
		}

		if err := appCtx.syncEngine.SyncSavedFilter(context.Background(), filter); err != nil {
			return fmt.Errorf("sync filter: %w", err)
		}

		printSuccess(filter)
		return nil
	},
}

var filterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved filters",
	Long: `Lists all active saved filter IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		filters, err := appCtx.filterRepo.List()
		if err != nil {
			return fmt.Errorf("list filters: %w", err)
		}

		activeFilters := make([]*domain.SavedFilter, 0)
		for _, f := range filters {
			if f.DeletedAt == nil {
				activeFilters = append(activeFilters, f)
			}
		}

		printSuccess(activeFilters)
		return nil
	},
}

func init() {
	filterAddCmd.Flags().String("view", "", "View (e.g., list, board)")
	filterAddCmd.Flags().String("criteria", "", "Filter criteria query")

	filterCmd.AddCommand(filterAddCmd)
	filterCmd.AddCommand(filterDeleteCmd)
	filterCmd.AddCommand(filterListCmd)

	rootCmd.AddCommand(filterCmd)
}
