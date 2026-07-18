package main

import (
	"github.com/spf13/cobra"
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
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		opts := CreateProjectOptions{Title: args[0]}
		opts.AreaID, _ = cmd.Flags().GetString("area-id")
		opts.AreaName, _ = cmd.Flags().GetString("area")

		project, err := appCtx.CreateProject(opts)
		if err != nil {
			return err
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
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		opts := UpdateProjectOptions{}
		opts.Status, _ = cmd.Flags().GetString("status")
		areaID, _ := cmd.Flags().GetString("area-id")
		opts.AreaName, _ = cmd.Flags().GetString("area")
		// Mirror prior CLI: Changed("area-id") || areaID != "" after name resolve.
		// areaID from the flag is usually only non-empty when Changed; AreaName
		// resolution happens inside UpdateProject.
		if cmd.Flags().Changed("area-id") {
			opts.AreaID = optionalString{Set: true, Value: areaID}
			opts.AreaFlagUsed = true
		}

		project, err := appCtx.UpdateProject(args[0], opts)
		if err != nil {
			return err
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
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		project, err := appCtx.DeleteProject(args[0])
		if err != nil {
			return err
		}
		printSuccess(project)
		return nil
	},
}

var projectRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a project",
	Long:  `Restores a soft-deleted project by ID, and cascades the restoration to child tasks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		project, err := appCtx.RestoreProject(args[0])
		if err != nil {
			return err
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

		ids, err := appCtx.ListActiveProjectIDs()
		if err != nil {
			return err
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
