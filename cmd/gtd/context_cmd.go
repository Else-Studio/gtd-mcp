package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "List known contexts",
	Long: `Contexts are free-form @labels on tasks (not first-class entities).
Use list to see distinct contexts currently present on non-deleted tasks.`,
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List distinct contexts used on tasks",
	Long: `Lists distinct context strings found on non-deleted tasks (e.g. @computer, @phone).
Defaults to JSON. When --plain is specified, prints a VALUE column table.
Contexts appear when assigned to tasks; there is no separate create command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		items, err := appCtx.ListContexts()
		if err != nil {
			return err
		}
		printSuccess(items)
		return nil
	},
}

// ListContexts returns sorted distinct contexts from the entity catalog.
func (c *appContext) ListContexts() ([]string, error) {
	catalog, err := c.taskQuery.GetEntityCatalog(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get catalog: %w", err)
	}
	out := append([]string(nil), catalog.Contexts...)
	sort.Strings(out)
	return out, nil
}

func init() {
	contextCmd.AddCommand(contextListCmd)
	rootCmd.AddCommand(contextCmd)
}
