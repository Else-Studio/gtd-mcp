package main

import (
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "List known tags",
	Long: `Tags are free-form #labels on tasks (not first-class entities).
Use list to see distinct tags currently present on non-deleted tasks.`,
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List distinct tags used on tasks",
	Long: `Lists distinct tag strings found on non-deleted tasks (e.g. #weekend).
Defaults to JSON. When --plain is specified, prints a VALUE column table.
Tags appear when assigned to tasks; there is no separate create command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.Close()

		items, err := appCtx.ListTags()
		if err != nil {
			return err
		}
		printSuccess(items)
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd)
	rootCmd.AddCommand(tagCmd)
}
