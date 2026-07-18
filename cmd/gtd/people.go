package main

import (
	"github.com/spf13/cobra"
)

var peopleCmd = &cobra.Command{
	Use:   "people",
	Short: "Manage people",
	Long: `Manage people (delegates) associated with tasks.
People are used to track 'waiting' tasks.`,
}

var peopleAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new person",
	Long:  `Adds a new person to the workspace with the specified name. Returns the JSON person representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		person, err := appCtx.CreatePerson(args[0])
		if err != nil {
			return err
		}
		printSuccess(person)
		return nil
	},
}

var peopleDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a person",
	Long:  `Soft-deletes a person by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		person, err := appCtx.DeletePerson(args[0])
		if err != nil {
			return err
		}
		printSuccess(person)
		return nil
	},
}

var peopleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List people",
	Long:  `Lists all active person IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		activePeople, err := appCtx.ListActivePeople()
		if err != nil {
			return err
		}
		printSuccess(activePeople)
		return nil
	},
}

var peopleUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a person",
	Long:  `Updates a person's name by ID. Use --name to change their name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		name, _ := cmd.Flags().GetString("name")
		person, err := appCtx.UpdatePerson(args[0], UpdatePersonOptions{Name: name})
		if err != nil {
			return err
		}
		printSuccess(person)
		return nil
	},
}

func init() {
	peopleUpdateCmd.Flags().String("name", "", "New name for the person")

	peopleCmd.AddCommand(peopleAddCmd)
	peopleCmd.AddCommand(peopleUpdateCmd)
	peopleCmd.AddCommand(peopleDeleteCmd)
	peopleCmd.AddCommand(peopleListCmd)

	rootCmd.AddCommand(peopleCmd)
}
