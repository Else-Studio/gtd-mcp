package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long: `Manage tasks within the GTD framework.
Allows adding, updating, listing, deleting, and restoring tasks. Default output is JSON, but '--plain' overrides this to output a readable ASCII table.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add [text...]",
	Short: "Add a new task",
	Long: `Add a new task using the quick-add NLP parser.
All words after add (and any flags) are joined into one string and parsed — shell
quotes around the whole task are optional for plain capture.

Unquoted entity tokens are a single word only; use quotes for multi-word names:
  +Project       Binds task to a project (one word; use +"Kitchen Sink" for multi-word)
  !Area          Binds task to an Area of Focus if no project is assigned (!"Work Home")
  @Context       Physical context (@computer; @"deep work")
  #Tag           Classification tags (#urgent; #"home office")
  %Person        Delegate for waiting tasks (%Bob; %"Jane Doe")
  /due:<date>    Sets due date (e.g. today, tomorrow, monday, or YYYY-MM-DD)
  /start:<date>  Sets relative or absolute start date
  /next          Sets status directly to 'next' action
  /someday       Sets status directly to 'someday' action
  /waiting       Sets status directly to 'waiting' action
  /reference     Sets status directly to 'reference' note
  /done          Sets status to done (sets completedAt)

Shell note: still quote when the shell would eat characters (especially # comments,
$ variables, * globs) or when NLP needs quoted multi-word entities.

Examples:
  gtd add Call the plumber about the leak
  gtd add Email Bob about proposal %Bob @computer /due:tomorrow
  gtd task add Email Bob about proposal %Bob @computer +"Work Migration" /due:tomorrow
  gtd add "Fix login #urgent"   # quotes protect # from the shell

Returns a JSON task object containing fields like id, title, status, contexts, tags, and warnings. In plain mode, prints a single-row task table.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		opts := CreateTaskOptions{Text: strings.Join(args, " ")}
		opts.ProjectID, _ = cmd.Flags().GetString("project-id")
		opts.AreaID, _ = cmd.Flags().GetString("area-id")
		opts.AreaName, _ = cmd.Flags().GetString("area")
		opts.AssignedTo, _ = cmd.Flags().GetString("assigned-to")

		result, err := appCtx.CreateTask(opts)
		if err != nil {
			return err
		}
		printSuccess(decorateTask(result.Task, result.InvalidDateCommands))
		return nil
	},
}

var taskUpdateCmd = &cobra.Command{
	Use:   "update <id> [text]",
	Short: "Update a task",
	Long: `Updates a task's properties.
If [text] is provided, it is parsed via NLP to update the task's title, project, area, contexts, tags, or dates.
The --status flag can be used to explicitly change the status (e.g. 'next', 'done', 'waiting', 'someday').

Special Behavior (Project Stall Telemetry):
If updating a task status to 'done' or 'archived' leaves its associated project with zero active 'next' tasks, the JSON output will return 'project_stalled: true' alongside next-action candidates.

Example:
  gtd task update c1a67a07-... "+Work Migration /due:2026-07-20" --status next`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		var text string
		if len(args) > 1 {
			text = args[1]
		}

		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		opts := UpdateTaskOptions{Text: text}
		opts.Status, _ = cmd.Flags().GetString("status")
		if cmd.Flags().Changed("project-id") {
			v, _ := cmd.Flags().GetString("project-id")
			opts.ProjectID = optionalString{Set: true, Value: v}
		}
		if cmd.Flags().Changed("area-id") {
			v, _ := cmd.Flags().GetString("area-id")
			opts.AreaID = optionalString{Set: true, Value: v}
		}
		if cmd.Flags().Changed("area") {
			v, _ := cmd.Flags().GetString("area")
			opts.AreaName = optionalString{Set: true, Value: v}
		}
		if cmd.Flags().Changed("assigned-to") {
			v, _ := cmd.Flags().GetString("assigned-to")
			opts.AssignedTo = optionalString{Set: true, Value: v}
		}
		if cmd.Flags().Changed("start-offset") {
			v, _ := cmd.Flags().GetString("start-offset")
			opts.StartOffset = optionalString{Set: true, Value: v}
		}
		if cmd.Flags().Changed("recurrence") {
			v, _ := cmd.Flags().GetString("recurrence")
			opts.Recurrence = optionalString{Set: true, Value: v}
		}

		result, err := appCtx.UpdateTask(id, opts)
		if err != nil {
			return err
		}

		var respData map[string]interface{}
		if result.ProjectStalled {
			respData = map[string]interface{}{
				"task":                   decorateTask(result.Task, result.InvalidDateCommands),
				"project_stalled":        true,
				"next_action_candidates": result.NextActionCandidates,
			}
		} else {
			respData = map[string]interface{}{
				"task": decorateTask(result.Task, result.InvalidDateCommands),
			}
		}
		printSuccess(respData)
		return nil
	},
}

// decorateTask merges coherence warnings and parser invalid-date feedback for JSON output.
func decorateTask(t *domain.Task, invalidDateCommands []string) interface{} {
	warnings := domain.ValidateTaskCoherence(t)
	warnings = append(warnings, invalidDateCommands...)
	if len(warnings) == 0 && len(invalidDateCommands) == 0 {
		return t
	}
	out := &TaskOutput{Task: t, Warnings: warnings}
	if len(invalidDateCommands) > 0 {
		out.InvalidDateCommands = invalidDateCommands
	}
	return out
}

// rejectArchivedProject blocks task create/update when the container project is archived.
func rejectArchivedProject(appCtx *appContext, projectID *string) error {
	if projectID == nil || *projectID == "" {
		return nil
	}
	project, err := appCtx.projectRepo.Get(*projectID)
	if err != nil {
		return nil // missing project is not this rule's concern
	}
	if project.Status == domain.ProjectStatusArchived {
		return fmt.Errorf("%w: cannot create or update tasks under archived project %q", domain.ErrValidation, project.Title)
	}
	return nil
}

// rejectArchivedProjectByTitle blocks auto-creating a twin project when an archived
// project with the same title already exists (catalog omits archived, so NLP would
// otherwise spawn a new active project with the same name).
func rejectArchivedProjectByTitle(appCtx *appContext, title string) error {
	projects, err := appCtx.projectRepo.List()
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects {
		if p.DeletedAt != nil {
			continue
		}
		if p.Title == title && p.Status == domain.ProjectStatusArchived {
			return fmt.Errorf("%w: cannot create or update tasks under archived project %q", domain.ErrValidation, title)
		}
	}
	return nil
}

// parseStartOffset accepts either JSON ({"amount":-1,"unit":"day"}) or a human
// form like "-1 day" / "-30 minute".
func parseStartOffset(s string) (*domain.RelativeOffset, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var offset domain.RelativeOffset
	if err := json.Unmarshal([]byte(s), &offset); err == nil && offset.Unit != "" {
		return &offset, nil
	}
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected JSON or \"<amount> <unit>\" (e.g. \"-1 day\"), got %q", s)
	}
	amount, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid amount %q: %w", parts[0], err)
	}
	unit := strings.ToLower(parts[1])
	// Normalize plural units (days → day, minutes → minute).
	if strings.HasSuffix(unit, "s") && unit != "s" {
		unit = strings.TrimSuffix(unit, "s")
	}
	return &domain.RelativeOffset{Amount: amount, Unit: unit}, nil
}

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a task",
	Long: `Soft-deletes a task by ID.
Soft-deleted tasks are excluded from normal list commands but remain in history.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		task, err := appCtx.DeleteTask(args[0])
		if err != nil {
			return err
		}
		printSuccess(task)
		return nil
	},
}

var taskRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a task",
	Long:  `Restores a soft-deleted task by ID, placing it back into active rotation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		task, err := appCtx.RestoreTask(args[0])
		if err != nil {
			return err
		}
		printSuccess(task)
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list [status]",
	Short: "List tasks",
	Long: `Lists active tasks.
Optional [status] filter can be applied (e.g. inbox, next, waiting, someday, reference, done, archived).
By default, returns a JSON list of task IDs. When --plain is specified, returns a detailed tabwriter table resolving task details (ID, Title, Status, Due Date, Project, Warnings).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		status := ""
		if len(args) > 0 {
			status = args[0]
		}
		ids, err := appCtx.ListTaskIDs(status, taskListFilterFromCmd(cmd))
		if err != nil {
			return err
		}
		printSuccess(resolveTasks(appCtx, ids))
		return nil
	},
}

var taskDuplicateCmd = &cobra.Command{
	Use:   "duplicate <id>",
	Short: "Duplicate a task",
	Long: `Deep-copies a task with a new unique ID.
Resets status to 'next' and clears completedAt so the clone is actionable.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		clone, err := appCtx.DuplicateTask(args[0])
		if err != nil {
			return err
		}
		printSuccess(map[string]interface{}{
			"task": clone,
		})
		return nil
	},
}

var taskPromoteCmd = &cobra.Command{
	Use:   "promote <id> <project_title>",
	Short: "Promote a task to a project",
	Long: `Creates a new project with the given title and links the task to it.
Clears the task's area association (container exclusivity). The task is preserved as the first step.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		result, err := appCtx.PromoteTask(args[0], args[1])
		if err != nil {
			return err
		}
		printSuccess(map[string]interface{}{
			"project_id": result.ProjectID,
			"task":       result.Task,
		})
		return nil
	},
}

// taskListFilterFromCmd maps list/shortcut flags onto TaskListFilter.
func taskListFilterFromCmd(cmd *cobra.Command) TaskListFilter {
	f := TaskListFilter{}
	f.AreaID, _ = cmd.Flags().GetString("area-id")
	f.AreaName, _ = cmd.Flags().GetString("area")
	f.ProjectID, _ = cmd.Flags().GetString("project-id")
	f.ProjectTitle, _ = cmd.Flags().GetString("project")
	f.Context, _ = cmd.Flags().GetString("context")
	f.AssignedTo, _ = cmd.Flags().GetString("assigned-to")
	return f
}

func init() {
	taskAddCmd.Flags().String("project-id", "", "Project ID")
	taskAddCmd.Flags().String("area-id", "", "Area ID")
	taskAddCmd.Flags().String("area", "", "Area name (sets area, clears project)")
	taskAddCmd.Flags().String("assigned-to", "", "Assigned To")

	taskUpdateCmd.Flags().String("status", "", "Status of the task")
	taskUpdateCmd.Flags().String("project-id", "", "Project ID")
	taskUpdateCmd.Flags().String("area-id", "", "Area ID (sets area, clears project)")
	taskUpdateCmd.Flags().String("area", "", "Area name (sets area, clears project)")
	taskUpdateCmd.Flags().String("assigned-to", "", "Assigned To")
	taskUpdateCmd.Flags().String("start-offset", "", "Relative start offset (JSON or \"-1 day\")")
	taskUpdateCmd.Flags().String("recurrence", "", "Recurrence rule JSON (e.g. {\"rule\":\"daily\"})")

	taskListCmd.Flags().String("area-id", "", "Filter by Area ID")
	taskListCmd.Flags().String("area", "", "Filter by Area Name")
	taskListCmd.Flags().String("project-id", "", "Filter by Project ID")
	taskListCmd.Flags().String("project", "", "Filter by Project Title")
	taskListCmd.Flags().String("context", "", "Filter by Context")
	taskListCmd.Flags().String("assigned-to", "", "Filter by Assigned To")

	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskUpdateCmd)
	taskCmd.AddCommand(taskDeleteCmd)
	taskCmd.AddCommand(taskRestoreCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskDuplicateCmd)
	taskCmd.AddCommand(taskPromoteCmd)

	rootCmd.AddCommand(taskCmd)
}
