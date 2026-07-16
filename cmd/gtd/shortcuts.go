package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Shortcut for 'gtd task list inbox'",
	Long: `Lists all tasks with 'inbox' status.
Unprocessed tasks that need clarification. Default returns a JSON list of IDs. Supports --plain for a task table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		ids, err := appCtx.taskQuery.ListInboxTasks(context.Background())
		if err != nil {
			return fmt.Errorf("list inbox: %w", err)
		}

		printSuccess(resolveTasks(appCtx, ids))
		return nil
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Shortcut for 'gtd task list next'",
	Long: `Lists all active, actionable tasks with 'next' status.
These are ready to be worked on immediately. Accepts the same filters as
task list next (--area, --area-id, --project, --project-id, --context,
--assigned-to). Default returns a JSON list of IDs. Supports --plain for a task table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		filter := buildTaskQueryFilter(cmd, appCtx)
		ids, err := appCtx.taskQuery.ListNextTasks(context.Background(), filter)
		if err != nil {
			return fmt.Errorf("list next: %w", err)
		}

		printSuccess(resolveTasks(appCtx, ids))
		return nil
	},
}

var stalledCmd = &cobra.Command{
	Use:   "stalled",
	Short: "List stalled projects",
	Long: `Lists all active projects that have zero tasks marked 'next'.
Used during Reflect/Weekly Review to identify outcomes requiring a new action step. Default returns a JSON list of project IDs. Supports --plain for a project table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		ids, err := appCtx.taskQuery.ListStalledProjects(context.Background())
		if err != nil {
			return fmt.Errorf("list stalled: %w", err)
		}

		printSuccess(resolveProjects(appCtx, ids))
		return nil
	},
}

var agendaCmd = &cobra.Command{
	Use:   "agenda",
	Short: "List tasks for the agenda (due or starting now or before)",
	Long: `Retrieves the immediate focus agenda ("What's Important Now").
Returns active (non-reference) tasks whose start time has passed, or whose due
date is today or earlier. Date-only dues use calendar-day comparison (due today
is included all day); timed dues use full timestamps. Soft-deleted, done, and
archived tasks are excluded. Default returns a JSON list of task IDs. Supports
--plain for a task table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		filter := buildTaskQueryFilter(cmd, appCtx)
		ids, err := appCtx.taskQuery.ListAgendaTasks(context.Background(), time.Now(), filter)
		if err != nil {
			return fmt.Errorf("list agenda: %w", err)
		}

		printSuccess(resolveTasks(appCtx, ids))
		return nil
	},
}

// addCmd is a root shortcut for quick capture (same as `gtd task add`).
// Flags and RunE must stay in sync with taskAddCmd.
var addCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Shortcut for 'gtd task add'",
	Long: `Quick-capture a task via the NLP quick-add parser.
Same behavior as gtd task add — see that command for token syntax.

Example:
  gtd add "Email Bob about proposal %Bob @computer /due:tomorrow"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return taskAddCmd.RunE(cmd, args)
	},
}

func init() {
	for _, c := range []*cobra.Command{nextCmd, agendaCmd} {
		c.Flags().String("area-id", "", "Filter by Area ID")
		c.Flags().String("area", "", "Filter by Area Name")
		c.Flags().String("project-id", "", "Filter by Project ID")
		c.Flags().String("project", "", "Filter by Project Title")
		c.Flags().String("context", "", "Filter by Context")
		c.Flags().String("assigned-to", "", "Filter by Assigned To")
	}

	// Same optional bind flags as task add (cmd.Flags() is per-command).
	addCmd.Flags().String("project-id", "", "Project ID")
	addCmd.Flags().String("area-id", "", "Area ID")
	addCmd.Flags().String("area", "", "Area name (sets area, clears project)")
	addCmd.Flags().String("assigned-to", "", "Assigned To")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(inboxCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(stalledCmd)
	rootCmd.AddCommand(agendaCmd)
}
