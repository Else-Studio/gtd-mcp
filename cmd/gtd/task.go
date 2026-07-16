package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
	"gtd/internal/parser"
	"gtd/internal/persistence/sqlite"
)



var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long: `Manage tasks within the GTD framework.
Allows adding, updating, listing, deleting, and restoring tasks. Default output is JSON, but '--plain' overrides this to output a readable ASCII table.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Add a new task",
	Long: `Add a new task using the quick-add NLP parser.
The parser processes the text string to instantly extract metadata:
  +ProjectName   Binds task to a project (case-insensitive, greedy match)
  !AreaName      Binds task to an Area of Focus (only if no project is assigned)
  @Context       Adds physical context tags (e.g. @computer, @phone)
  #Tag           Adds general classification tags (e.g. #urgent)
  %Person        Assigns a delegate for waiting tasks
  /due:<date>    Sets due date (e.g. today, tomorrow, monday, or YYYY-MM-DD)
  /start:<date>  Sets relative or absolute start date
  /next          Sets status directly to 'next' action
  /someday       Sets status directly to 'someday' action
  /waiting       Sets status directly to 'waiting' action
  /reference     Sets status directly to 'reference' note

Example:
  gtd task add "Email Bob about proposal %Bob @computer +Work Migration /due:tomorrow"

Returns a JSON task object containing fields like id, title, status, contexts, tags, and warnings. In plain mode, prints a single-row task table.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		catalog, err := appCtx.taskQuery.GetEntityCatalog(context.Background())
		if err != nil {
			return fmt.Errorf("get catalog: %w", err)
		}

		now := time.Now()
		parsed, err := parser.Parse(text, catalog, parser.ParseOptions{FallbackTitle: "Screenshot"}, now)
		if err != nil {
			return fmt.Errorf("parse task: %w", err)
		}

		status := domain.TaskStatusInbox
		if parsed.Status != nil {
			status = *parsed.Status
		}

		task := &domain.Task{
			ID:          uuid.New().String(),
			Title:       parsed.Title,
			Status:      status,
			Contexts:    parsed.Contexts,
			Tags:        parsed.Tags,
			ProjectID:   parsed.ProjectID,
			AreaID:      parsed.AreaID,
			DueDate:     parsed.DueDate,
			StartTime:   parsed.StartTime,
			ReviewAt:    parsed.ReviewAt,
			Attachments: parsed.Attachments,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		
		if parsed.AssignedTo != nil {
			task.AssignedTo = *parsed.AssignedTo
		}
		if parsed.Description != nil {
			task.Description = *parsed.Description
		}
		if parsed.EnergyLevel != nil {
			task.EnergyLevel = *parsed.EnergyLevel
		}
		if parsed.Priority != nil {
			task.Priority = *parsed.Priority
		}
		if parsed.Recurrence != nil {
			task.Recurrence = parsed.Recurrence
		}

		if parsed.ProjectTitle != nil {
			if err := rejectArchivedProjectByTitle(appCtx, *parsed.ProjectTitle); err != nil {
				return err
			}
			project := &domain.Project{
				ID:        uuid.New().String(),
				Title:     *parsed.ProjectTitle,
				Status:    domain.ProjectStatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := appCtx.projectRepo.Save(project); err != nil {
				return fmt.Errorf("save new project: %w", err)
			}
			if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
				return fmt.Errorf("sync new project: %w", err)
			}
			task.ProjectID = &project.ID
			task.AreaID = nil
		} else if parsed.AreaName != nil {
			area := &domain.Area{
				ID:        uuid.New().String(),
				Name:      *parsed.AreaName,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := appCtx.areaRepo.Save(area); err != nil {
				return fmt.Errorf("save new area: %w", err)
			}
			if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
				return fmt.Errorf("sync new area: %w", err)
			}
			task.AreaID = &area.ID
			task.ProjectID = nil
		}

		if parsed.AssignedTo != nil {
			task.AssignedTo = *parsed.AssignedTo
		}
		if parsed.Description != nil {
			task.Description = *parsed.Description
		}
		if parsed.EnergyLevel != nil {
			task.EnergyLevel = *parsed.EnergyLevel
		}

		if projID, _ := cmd.Flags().GetString("project-id"); projID != "" {
			task.ProjectID = &projID
			task.AreaID = nil
		}
		if areaID, _ := cmd.Flags().GetString("area-id"); areaID != "" {
			task.AreaID = &areaID
			if task.ProjectID != nil {
				task.AreaID = nil
			}
		}
		if assignedTo, _ := cmd.Flags().GetString("assigned-to"); assignedTo != "" {
			task.AssignedTo = assignedTo
		}

		if err := rejectArchivedProject(appCtx, task.ProjectID); err != nil {
			return err
		}

		if err := appCtx.taskRepo.Save(task); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		if err := appCtx.syncEngine.SyncTask(context.Background(), task, time.Now()); err != nil {
			return fmt.Errorf("sync task: %w", err)
		}

		printSuccess(task)
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
	Args:  cobra.MinimumNArgs(1),
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

		task, err := appCtx.taskRepo.Get(id)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		// Capture status before any mutation for recurrence side-effects.
		previousStatus := task.Status
		now := time.Now()

		if text != "" {
			catalog, err := appCtx.taskQuery.GetEntityCatalog(context.Background())
			if err != nil {
				return fmt.Errorf("get catalog: %w", err)
			}
			parsed, err := parser.Parse(text, catalog, parser.ParseOptions{}, now)
			if err == nil || err.Error() == "empty-title" {
				if err == nil && parsed.Title != "" {
					task.Title = parsed.Title
				}
				if parsed.Status != nil {
					if *parsed.Status == domain.TaskStatusReference {
						task.SetReference()
						task.UpdateStatus(domain.TaskStatusReference, now)
					} else {
						task.UpdateStatus(*parsed.Status, now)
					}
				}
				if len(parsed.Contexts) > 0 {
					task.Contexts = parsed.Contexts
				}
				if len(parsed.Tags) > 0 {
					task.Tags = parsed.Tags
				}
				if parsed.ProjectTitle != nil {
					if err := rejectArchivedProjectByTitle(appCtx, *parsed.ProjectTitle); err != nil {
						return err
					}
					project := &domain.Project{
						ID:        uuid.New().String(),
						Title:     *parsed.ProjectTitle,
						Status:    domain.ProjectStatusActive,
						CreatedAt: now,
						UpdatedAt: now,
					}
					if err := appCtx.projectRepo.Save(project); err != nil {
						return fmt.Errorf("save new project: %w", err)
					}
					if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
						return fmt.Errorf("sync new project: %w", err)
					}
					task.ProjectID = &project.ID
					task.AreaID = nil
				} else if parsed.ProjectID != nil {
					task.ProjectID = parsed.ProjectID
					task.AreaID = nil
				}

				if parsed.AreaName != nil {
					area := &domain.Area{
						ID:        uuid.New().String(),
						Name:      *parsed.AreaName,
						CreatedAt: now,
						UpdatedAt: now,
					}
					if err := appCtx.areaRepo.Save(area); err != nil {
						return fmt.Errorf("save new area: %w", err)
					}
					if err := appCtx.syncEngine.SyncArea(context.Background(), area); err != nil {
						return fmt.Errorf("sync new area: %w", err)
					}
					task.AreaID = &area.ID
					if task.ProjectID != nil {
						task.AreaID = nil
					}
				} else if parsed.AreaID != nil {
					task.AreaID = parsed.AreaID
					if task.ProjectID != nil {
						task.AreaID = nil
					}
				}
				if parsed.AssignedTo != nil {
					task.AssignedTo = *parsed.AssignedTo
				} else {
					task.AssignedTo = ""
				}
				if parsed.Description != nil {
					task.Description = *parsed.Description
				}
				if parsed.EnergyLevel != nil {
					task.EnergyLevel = *parsed.EnergyLevel
				}
				if parsed.Priority != nil {
					task.Priority = *parsed.Priority
				}
				if parsed.Recurrence != nil {
					task.Recurrence = parsed.Recurrence
				}
				// Use domain helpers so relative start offsets recompute with due-date changes.
				if parsed.DueDate != nil {
					task.UpdateDueDate(parsed.DueDate)
				}
				if parsed.StartTime != nil {
					task.UpdateStartTime(parsed.StartTime)
				}
				if parsed.ReviewAt != nil {
					task.ReviewAt = parsed.ReviewAt
				}
				if len(parsed.Attachments) > 0 {
					task.Attachments = append(task.Attachments, parsed.Attachments...)
				}
			} else {
				return fmt.Errorf("parse task: %w", err)
			}
		}

		if cmd.Flags().Changed("project-id") {
			if projID, _ := cmd.Flags().GetString("project-id"); projID != "" {
				task.ProjectID = &projID
				task.AreaID = nil
			} else {
				task.ProjectID = nil
			}
			task.UpdatedAt = time.Now()
		}
		if cmd.Flags().Changed("area-id") {
			if areaID, _ := cmd.Flags().GetString("area-id"); areaID != "" {
				task.AreaID = &areaID
				if task.ProjectID != nil {
					task.AreaID = nil
				}
			} else {
				task.AreaID = nil
			}
			task.UpdatedAt = time.Now()
		}
		if cmd.Flags().Changed("assigned-to") {
			task.AssignedTo, _ = cmd.Flags().GetString("assigned-to")
			task.UpdatedAt = time.Now()
		}
		if cmd.Flags().Changed("start-offset") {
			offsetStr, _ := cmd.Flags().GetString("start-offset")
			if offsetStr == "" {
				task.UpdateRelativeStartOffset(nil)
			} else {
				offset, err := parseStartOffset(offsetStr)
				if err != nil {
					return fmt.Errorf("invalid --start-offset: %w", err)
				}
				task.UpdateRelativeStartOffset(offset)
			}
		}
		if cmd.Flags().Changed("recurrence") {
			recStr, _ := cmd.Flags().GetString("recurrence")
			if recStr == "" {
				task.Recurrence = nil
			} else {
				var rule domain.RecurrenceRule
				if err := json.Unmarshal([]byte(recStr), &rule); err != nil {
					return fmt.Errorf("invalid --recurrence JSON: %w", err)
				}
				task.Recurrence = &rule
			}
			task.UpdatedAt = time.Now()
		}

		status, _ := cmd.Flags().GetString("status")
		if status != "" {
			if status == string(domain.TaskStatusReference) {
				task.SetReference()
				task.UpdateStatus(domain.TaskStatusReference, time.Now())
			} else {
				task.UpdateStatus(domain.TaskStatus(status), time.Now())
			}
		}

		if err := rejectArchivedProject(appCtx, task.ProjectID); err != nil {
			return err
		}

		if err := appCtx.taskRepo.Save(task); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		now = time.Now()
		if err := appCtx.syncEngine.SyncTask(context.Background(), task, now); err != nil {
			return fmt.Errorf("sync task: %w", err)
		}

		// Recurring task automation: completing a recurring task spawns the next occurrence.
		if (task.Status == domain.TaskStatusDone || task.Status == domain.TaskStatusArchived) && task.Recurrence != nil {
			nextTask := task.DuplicateRecurringTask(uuid.New().String(), now, previousStatus)
			if nextTask != nil {
				if err := appCtx.taskRepo.Save(nextTask); err != nil {
					return fmt.Errorf("save next recurring occurrence: %w", err)
				}
				if err := appCtx.syncEngine.SyncTask(context.Background(), nextTask, now); err != nil {
					return fmt.Errorf("sync next recurring occurrence: %w", err)
				}
			}
		}

		var respData map[string]interface{}

		// Project stall logic
		projectStalled := false
		var candidates []domain.Task

		if (task.Status == domain.TaskStatusDone || task.Status == domain.TaskStatusArchived) && task.ProjectID != nil {
			ctx := context.Background()
			isActive, err := appCtx.taskQuery.IsProjectActive(ctx, *task.ProjectID)
			if err != nil {
				return fmt.Errorf("check project active: %w", err)
			}
			if isActive {
				count, err := appCtx.taskQuery.CountProjectNextTasks(ctx, *task.ProjectID)
				if err != nil {
					return fmt.Errorf("count project next tasks: %w", err)
				}
				if count == 0 {
					projectStalled = true
					candidates, err = appCtx.taskQuery.GetProjectCandidates(ctx, *task.ProjectID)
					if err != nil {
						return fmt.Errorf("get project candidates: %w", err)
					}
				}
			}
		}

		if projectStalled {
			respData = map[string]interface{}{
				"task":                   task,
				"project_stalled":        true,
				"next_action_candidates": candidates,
			}
		} else {
			respData = map[string]interface{}{
				"task": task,
			}
		}

		printSuccess(respData)
		return nil
	},
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
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		task, err := appCtx.taskRepo.Get(id)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		now := time.Now()
		task.SoftDelete(now)

		if err := appCtx.taskRepo.Save(task); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		if err := appCtx.syncEngine.SyncTask(context.Background(), task, now); err != nil {
			return fmt.Errorf("sync task: %w", err)
		}

		printSuccess(task)
		return nil
	},
}

var taskRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a task",
	Long: `Restores a soft-deleted task by ID, placing it back into active rotation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		task, err := appCtx.taskRepo.Get(id)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		task.Restore(time.Now())

		if err := appCtx.taskRepo.Save(task); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		if err := appCtx.syncEngine.SyncTask(context.Background(), task, time.Now()); err != nil {
			return fmt.Errorf("sync task: %w", err)
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
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		ids := []string{}
		ctx := context.Background()

		filter := buildTaskQueryFilter(cmd, appCtx)

		if len(args) == 0 {
			ids, err = appCtx.taskQuery.ListActiveTasks(ctx, filter)
		} else {
			ids, err = appCtx.taskQuery.ListTasksByStatus(ctx, args[0], filter)
		}

		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
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
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		orig, err := appCtx.taskRepo.Get(id)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		now := time.Now()
		clone := *orig
		clone.ID = uuid.New().String()
		clone.CreatedAt = now
		clone.UpdatedAt = now
		clone.DeletedAt = nil

		// Deep-copy slices so clone and original do not share backing arrays.
		if orig.Tags != nil {
			clone.Tags = append([]string(nil), orig.Tags...)
		}
		if orig.Contexts != nil {
			clone.Contexts = append([]string(nil), orig.Contexts...)
		}
		if orig.Attachments != nil {
			clone.Attachments = append([]domain.Attachment(nil), orig.Attachments...)
		}
		if orig.Recurrence != nil {
			r := *orig.Recurrence
			clone.Recurrence = &r
		}
		if orig.RelativeStartOffset != nil {
			o := *orig.RelativeStartOffset
			clone.RelativeStartOffset = &o
		}
		if orig.DueDate != nil {
			d := *orig.DueDate
			clone.DueDate = &d
		}
		if orig.StartTime != nil {
			s := *orig.StartTime
			clone.StartTime = &s
		}
		if orig.ReviewAt != nil {
			r := *orig.ReviewAt
			clone.ReviewAt = &r
		}
		if orig.ProjectID != nil {
			p := *orig.ProjectID
			clone.ProjectID = &p
		}
		if orig.AreaID != nil {
			a := *orig.AreaID
			clone.AreaID = &a
		}

		clone.UpdateStatus(domain.TaskStatusNext, now)
		clone.CompletedAt = nil

		if err := appCtx.taskRepo.Save(&clone); err != nil {
			return fmt.Errorf("save duplicated task: %w", err)
		}
		if err := appCtx.syncEngine.SyncTask(context.Background(), &clone, now); err != nil {
			return fmt.Errorf("sync duplicated task: %w", err)
		}

		printSuccess(map[string]interface{}{
			"task": &clone,
		})
		return nil
	},
}

var taskPromoteCmd = &cobra.Command{
	Use:   "promote <id> <project_title>",
	Short: "Promote a task to a project",
	Long: `Creates a new project with the given title and links the task to it.
Clears the task's area association (container exclusivity). The task is preserved as the first step.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		title := args[1]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		task, err := appCtx.taskRepo.Get(id)
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}

		now := time.Now()

		// Reuse an existing active (non-archived, non-deleted) project with the same title
		// in the same area when possible.
		var project *domain.Project
		projects, err := appCtx.projectRepo.List()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		for _, p := range projects {
			if p.DeletedAt != nil || p.Status == domain.ProjectStatusArchived {
				continue
			}
			if p.Title != title {
				continue
			}
			// Same area: both nil, or both point to the same area ID.
			sameArea := (p.AreaID == nil && task.AreaID == nil) ||
				(p.AreaID != nil && task.AreaID != nil && *p.AreaID == *task.AreaID)
			if sameArea {
				project = p
				break
			}
		}

		if project == nil {
			project = &domain.Project{
				ID:        uuid.New().String(),
				Title:     title,
				Status:    domain.ProjectStatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			}
			// Inherit the task's area when creating a new project.
			if task.AreaID != nil {
				areaID := *task.AreaID
				project.AreaID = &areaID
			}
			if err := appCtx.projectRepo.Save(project); err != nil {
				return fmt.Errorf("save new project: %w", err)
			}
			if err := appCtx.syncEngine.SyncProject(context.Background(), project); err != nil {
				return fmt.Errorf("sync new project: %w", err)
			}
		}

		task.ProjectID = &project.ID
		task.AreaID = nil
		task.UpdatedAt = now

		if err := appCtx.taskRepo.Save(task); err != nil {
			return fmt.Errorf("save task: %w", err)
		}
		if err := appCtx.syncEngine.SyncTask(context.Background(), task, now); err != nil {
			return fmt.Errorf("sync task: %w", err)
		}

		printSuccess(map[string]interface{}{
			"project_id": project.ID,
			"task":       task,
		})
		return nil
	},
}

func buildTaskQueryFilter(cmd *cobra.Command, appCtx *appContext) *sqlite.TaskQueryFilter {
	filter := &sqlite.TaskQueryFilter{}

	if val, _ := cmd.Flags().GetString("area-id"); val != "" {
		filter.AreaID = val
	} else if name, _ := cmd.Flags().GetString("area"); name != "" {
		areas, _ := appCtx.areaRepo.List()
		for _, a := range areas {
			if a.Name == name {
				filter.AreaID = a.ID
				break
			}
		}
	}

	if val, _ := cmd.Flags().GetString("project-id"); val != "" {
		filter.ProjectID = val
	} else if title, _ := cmd.Flags().GetString("project"); title != "" {
		projects, _ := appCtx.projectRepo.List()
		for _, p := range projects {
			if p.Title == title {
				filter.ProjectID = p.ID
				break
			}
		}
	}

	if val, _ := cmd.Flags().GetString("context"); val != "" {
		filter.Context = val
	}
	if val, _ := cmd.Flags().GetString("assigned-to"); val != "" {
		filter.AssignedTo = val
	}

	return filter
}

func init() {
	taskAddCmd.Flags().String("project-id", "", "Project ID")
	taskAddCmd.Flags().String("area-id", "", "Area ID")
	taskAddCmd.Flags().String("assigned-to", "", "Assigned To")

	taskUpdateCmd.Flags().String("status", "", "Status of the task")
	taskUpdateCmd.Flags().String("project-id", "", "Project ID")
	taskUpdateCmd.Flags().String("area-id", "", "Area ID")
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
