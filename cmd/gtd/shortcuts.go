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
These are ready to be worked on immediately. Default returns a JSON list of IDs. Supports --plain for a task table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		ids, err := appCtx.taskQuery.ListNextTasks(context.Background())
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
	Long: `Retrieves the immediate focus agenda.
Returns active tasks whose start date has passed, or due date is today or overdue. 
Note: Due dates without a specific time are treated as starting at 23:59:59.999 local time.
Default returns a JSON list of task IDs. Supports --plain for a task table.`,
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

func init() {
	agendaCmd.Flags().String("area-id", "", "Filter by Area ID")
	agendaCmd.Flags().String("area", "", "Filter by Area Name")
	agendaCmd.Flags().String("project-id", "", "Filter by Project ID")
	agendaCmd.Flags().String("project", "", "Filter by Project Title")
	agendaCmd.Flags().String("context", "", "Filter by Context")
	agendaCmd.Flags().String("assigned-to", "", "Filter by Assigned To")

	rootCmd.AddCommand(inboxCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(stalledCmd)
	rootCmd.AddCommand(agendaCmd)
}
