package main

import (
	"github.com/spf13/cobra"
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
	Long:  `Creates a new Area of Focus with the specified name. Returns the JSON area representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.CreateArea(args[0])
		if err != nil {
			return err
		}
		printSuccess(area)
		return nil
	},
}

var areaDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an area",
	Long:  `Soft-deletes an Area of Focus by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.DeleteArea(args[0])
		if err != nil {
			return err
		}
		printSuccess(area)
		return nil
	},
}

var areaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List areas",
	Long:  `Lists all active Area of Focus IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		activeAreas, err := appCtx.ListActiveAreas()
		if err != nil {
			return err
		}
		printSuccess(activeAreas)
		return nil
	},
}

var areaUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an area",
	Long:  `Updates an Area of Focus by ID. Use --name to change its name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		name, _ := cmd.Flags().GetString("name")
		area, err := appCtx.UpdateArea(args[0], UpdateAreaOptions{Name: name})
		if err != nil {
			return err
		}
		printSuccess(area)
		return nil
	},
}

var areaRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore an area",
	Long:  `Restores a soft-deleted Area of Focus by ID, and cascades the restoration to child projects and tasks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		area, err := appCtx.RestoreArea(args[0])
		if err != nil {
			return err
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
