package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gtd/internal/domain"
	"gtd/internal/parser"
	"gtd/internal/persistence/sqlite"
)

// parseLabeledList splits a comma-separated flag value into a normalized slice.
// Empty raw (after trim) returns an empty slice (clear). Items are trimmed;
// missing labelPrefix is prepended (e.g. "@" for contexts, "#" for tags).
func parseLabeledList(raw, labelPrefix string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if labelPrefix != "" && !strings.HasPrefix(p, labelPrefix) {
			p = labelPrefix + p
		}
		out = append(out, p)
	}
	return out
}

// CreateTaskOptions holds NLP text and optional flag overrides for task add.
type CreateTaskOptions struct {
	Text       string
	ProjectID  string // flag override if non-empty
	AreaID     string
	AreaName   string // resolve existing area; validation error if missing
	AssignedTo string
}

// CreateTaskResult carries the created task plus parser feedback for decorateTask.
type CreateTaskResult struct {
	Task                *domain.Task
	InvalidDateCommands []string
}

// CreateTask parses NLP text, applies flag overrides, persists the task, and
// auto-creates project/area when the parser requests them by title/name.
func (c *appContext) CreateTask(opts CreateTaskOptions) (*CreateTaskResult, error) {
	catalog, err := c.taskQuery.GetEntityCatalog(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get catalog: %w", err)
	}

	now := time.Now()
	parsed, err := parser.Parse(opts.Text, catalog, parser.ParseOptions{FallbackTitle: "Screenshot"}, now)
	if err != nil {
		return nil, fmt.Errorf("parse task: %w", err)
	}

	task := &domain.Task{
		ID:          uuid.New().String(),
		Title:       parsed.Title,
		Status:      domain.TaskStatusInbox,
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

	// Apply NLP status via domain helpers so /done and /archived set completedAt
	// (and /reference clears schedule fields) before the file write.
	if parsed.Status != nil {
		if *parsed.Status == domain.TaskStatusReference {
			task.SetReference()
			task.UpdateStatus(domain.TaskStatusReference, now)
		} else {
			task.UpdateStatus(*parsed.Status, now)
		}
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
		if err := rejectArchivedProjectByTitle(c, *parsed.ProjectTitle); err != nil {
			return nil, err
		}
		project := &domain.Project{
			ID:        uuid.New().String(),
			Title:     *parsed.ProjectTitle,
			Status:    domain.ProjectStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := c.PersistProject(project); err != nil {
			return nil, fmt.Errorf("persist new project: %w", err)
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
		if err := c.PersistArea(area); err != nil {
			return nil, fmt.Errorf("persist new area: %w", err)
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

	if opts.ProjectID != "" {
		task.ProjectID = &opts.ProjectID
		task.AreaID = nil
	}
	// Explicit area flag: set area and clear project (last container wins).
	if opts.AreaID != "" {
		task.AreaID = &opts.AreaID
		task.ProjectID = nil
	} else if opts.AreaName != "" {
		areas, _ := c.areaRepo.List()
		var foundID string
		for _, a := range areas {
			if a.Name == opts.AreaName && a.DeletedAt == nil {
				foundID = a.ID
				break
			}
		}
		if foundID == "" {
			return nil, fmt.Errorf("%w: area %q not found", domain.ErrValidation, opts.AreaName)
		}
		task.AreaID = &foundID
		task.ProjectID = nil
	}
	if opts.AssignedTo != "" {
		task.AssignedTo = opts.AssignedTo
	}

	if err := rejectArchivedProject(c, task.ProjectID); err != nil {
		return nil, err
	}

	if err := c.PersistTask(task, now); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}

	return &CreateTaskResult{
		Task:                task,
		InvalidDateCommands: parsed.InvalidDateCommands,
	}, nil
}

// UpdateTaskOptions holds optional NLP text and flag-driven field updates.
// optionalString fields: Set false = leave alone; Set true = apply Value (empty clears).
// Contexts/Tags: Set true with empty Value clears the slice; non-empty replaces
// the whole list (comma-separated values).
type UpdateTaskOptions struct {
	Text        string
	Status      string // empty = no status flag
	ProjectID   optionalString
	AreaID      optionalString
	AreaName    optionalString
	AssignedTo  optionalString
	StartOffset optionalString
	Recurrence  optionalString
	Contexts    optionalString
	Tags        optionalString
}

// UpdateTaskResult is the updated task plus presentation side-data for the CLI.
type UpdateTaskResult struct {
	Task                 *domain.Task
	InvalidDateCommands  []string
	ProjectStalled       bool
	NextActionCandidates []domain.Task
}

// UpdateTask loads a task, applies NLP and flag updates, persists, may spawn the
// next recurring occurrence, and reports project-stall telemetry.
func (c *appContext) UpdateTask(id string, opts UpdateTaskOptions) (*UpdateTaskResult, error) {
	task, err := c.taskRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Capture status before any mutation for recurrence side-effects.
	previousStatus := task.Status
	now := time.Now()
	var invalidDateCommands []string

	if opts.Text != "" {
		catalog, err := c.taskQuery.GetEntityCatalog(context.Background())
		if err != nil {
			return nil, fmt.Errorf("get catalog: %w", err)
		}
		parsed, err := parser.Parse(opts.Text, catalog, parser.ParseOptions{}, now)
		if err == nil || err.Error() == "empty-title" {
			if parsed != nil {
				invalidDateCommands = parsed.InvalidDateCommands
			}
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
				if err := rejectArchivedProjectByTitle(c, *parsed.ProjectTitle); err != nil {
					return nil, err
				}
				project := &domain.Project{
					ID:        uuid.New().String(),
					Title:     *parsed.ProjectTitle,
					Status:    domain.ProjectStatusActive,
					CreatedAt: now,
					UpdatedAt: now,
				}
				if err := c.PersistProject(project); err != nil {
					return nil, fmt.Errorf("persist new project: %w", err)
				}
				task.ProjectID = &project.ID
				task.AreaID = nil
			} else if parsed.ProjectID != nil {
				task.ProjectID = parsed.ProjectID
				task.AreaID = nil
			}

			// Explicit area token without a project token in this parse: move
			// to the area (clear project). If both project and area tokens
			// appear, project wins (handled above; skip area when project set).
			projectInParse := parsed.ProjectTitle != nil || parsed.ProjectID != nil
			if !projectInParse {
				if parsed.AreaName != nil {
					area := &domain.Area{
						ID:        uuid.New().String(),
						Name:      *parsed.AreaName,
						CreatedAt: now,
						UpdatedAt: now,
					}
					if err := c.PersistArea(area); err != nil {
						return nil, fmt.Errorf("persist new area: %w", err)
					}
					task.AreaID = &area.ID
					task.ProjectID = nil
				} else if parsed.AreaID != nil {
					task.AreaID = parsed.AreaID
					task.ProjectID = nil
				}
			}
			// Only change assignee when % is present; omit means leave as-is.
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
			return nil, fmt.Errorf("parse task: %w", err)
		}
	}

	if opts.ProjectID.Set {
		if opts.ProjectID.Value != "" {
			projID := opts.ProjectID.Value
			task.ProjectID = &projID
			task.AreaID = nil
		} else {
			task.ProjectID = nil
		}
		task.UpdatedAt = time.Now()
	}
	// Explicit --area / --area-id: last container wins — set area, clear project.
	if opts.AreaID.Set || opts.AreaName.Set {
		areaID := ""
		if opts.AreaID.Set {
			areaID = opts.AreaID.Value
		}
		if areaID == "" {
			if opts.AreaName.Set && opts.AreaName.Value != "" {
				areaName := opts.AreaName.Value
				areas, _ := c.areaRepo.List()
				for _, a := range areas {
					if a.Name == areaName && a.DeletedAt == nil {
						areaID = a.ID
						break
					}
				}
				if areaID == "" {
					return nil, fmt.Errorf("%w: area %q not found", domain.ErrValidation, areaName)
				}
			}
		}
		if areaID != "" {
			task.AreaID = &areaID
			task.ProjectID = nil
		} else {
			task.AreaID = nil
		}
		task.UpdatedAt = time.Now()
	}
	if opts.AssignedTo.Set {
		task.AssignedTo = opts.AssignedTo.Value
		task.UpdatedAt = time.Now()
	}
	if opts.StartOffset.Set {
		if opts.StartOffset.Value == "" {
			task.UpdateRelativeStartOffset(nil)
		} else {
			offset, err := parseStartOffset(opts.StartOffset.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid --start-offset: %w", err)
			}
			task.UpdateRelativeStartOffset(offset)
		}
	}
	if opts.Recurrence.Set {
		if opts.Recurrence.Value == "" {
			task.Recurrence = nil
		} else {
			var rule domain.RecurrenceRule
			if err := json.Unmarshal([]byte(opts.Recurrence.Value), &rule); err != nil {
				return nil, fmt.Errorf("invalid --recurrence JSON: %w", err)
			}
			task.Recurrence = &rule
		}
		task.UpdatedAt = time.Now()
	}
	if opts.Contexts.Set {
		task.Contexts = parseLabeledList(opts.Contexts.Value, "@")
		task.UpdatedAt = time.Now()
	}
	if opts.Tags.Set {
		task.Tags = parseLabeledList(opts.Tags.Value, "#")
		task.UpdatedAt = time.Now()
	}

	if opts.Status != "" {
		if opts.Status == string(domain.TaskStatusReference) {
			task.SetReference()
			task.UpdateStatus(domain.TaskStatusReference, time.Now())
		} else {
			task.UpdateStatus(domain.TaskStatus(opts.Status), time.Now())
		}
	}

	if err := rejectArchivedProject(c, task.ProjectID); err != nil {
		return nil, err
	}

	now = time.Now()
	if err := c.PersistTask(task, now); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}

	// Recurring task automation: spawn only on transition into done/archived
	// (business rule §6), not on every re-save of an already completed instance.
	justCompleted := (task.Status == domain.TaskStatusDone || task.Status == domain.TaskStatusArchived) &&
		previousStatus != domain.TaskStatusDone &&
		previousStatus != domain.TaskStatusArchived
	if justCompleted && task.Recurrence != nil {
		nextTask := task.DuplicateRecurringTask(uuid.New().String(), now, previousStatus)
		if nextTask != nil {
			if err := c.PersistTask(nextTask, now); err != nil {
				return nil, fmt.Errorf("persist next recurring occurrence: %w", err)
			}
		}
	}

	// Project stall logic
	projectStalled := false
	var candidates []domain.Task

	if (task.Status == domain.TaskStatusDone || task.Status == domain.TaskStatusArchived) && task.ProjectID != nil {
		ctx := context.Background()
		isActive, err := c.taskQuery.IsProjectActive(ctx, *task.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("check project active: %w", err)
		}
		if isActive {
			count, err := c.taskQuery.CountProjectNextTasks(ctx, *task.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("count project next tasks: %w", err)
			}
			if count == 0 {
				projectStalled = true
				candidates, err = c.taskQuery.GetProjectCandidates(ctx, *task.ProjectID)
				if err != nil {
					return nil, fmt.Errorf("get project candidates: %w", err)
				}
			}
		}
	}

	return &UpdateTaskResult{
		Task:                 task,
		InvalidDateCommands:  invalidDateCommands,
		ProjectStalled:       projectStalled,
		NextActionCandidates: candidates,
	}, nil
}

// DeleteTask soft-deletes a task by ID and returns the updated entity for printing.
func (c *appContext) DeleteTask(id string) (*domain.Task, error) {
	task, err := c.taskRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	now := time.Now()
	task.SoftDelete(now)

	if err := c.PersistTask(task, now); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}
	return task, nil
}

// RestoreTask clears soft-delete on a task and persists it.
func (c *appContext) RestoreTask(id string) (*domain.Task, error) {
	task, err := c.taskRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	now := time.Now()
	task.Restore(now)

	if err := c.PersistTask(task, now); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}
	return task, nil
}

// TaskListFilter is a cobra-free task list filter (IDs or names resolved inside ListTaskIDs).
type TaskListFilter struct {
	AreaID       string
	AreaName     string
	ProjectID    string
	ProjectTitle string
	Context      string
	AssignedTo   string
}

// ListTaskIDs returns active task IDs, optionally filtered by status and TaskListFilter.
// Empty status lists all active tasks; non-empty uses ListTasksByStatus.
func (c *appContext) ListTaskIDs(status string, f TaskListFilter) ([]string, error) {
	filter := c.resolveTaskListFilter(f)
	ctx := context.Background()
	var ids []string
	var err error
	if status == "" {
		ids, err = c.taskQuery.ListActiveTasks(ctx, filter)
	} else {
		ids, err = c.taskQuery.ListTasksByStatus(ctx, status, filter)
	}
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return ids, nil
}

func (c *appContext) resolveTaskListFilter(f TaskListFilter) *sqlite.TaskQueryFilter {
	filter := &sqlite.TaskQueryFilter{}

	if f.AreaID != "" {
		filter.AreaID = f.AreaID
	} else if f.AreaName != "" {
		areas, _ := c.areaRepo.List()
		for _, a := range areas {
			if a.Name == f.AreaName {
				filter.AreaID = a.ID
				break
			}
		}
	}

	if f.ProjectID != "" {
		filter.ProjectID = f.ProjectID
	} else if f.ProjectTitle != "" {
		projects, _ := c.projectRepo.List()
		for _, p := range projects {
			if p.Title == f.ProjectTitle {
				filter.ProjectID = p.ID
				break
			}
		}
	}

	if f.Context != "" {
		filter.Context = f.Context
	}
	if f.AssignedTo != "" {
		filter.AssignedTo = f.AssignedTo
	}
	return filter
}

// DuplicateTask deep-copies a task with a new ID, status next, and cleared completedAt.
func (c *appContext) DuplicateTask(id string) (*domain.Task, error) {
	orig, err := c.taskRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
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

	if err := c.PersistTask(&clone, now); err != nil {
		return nil, fmt.Errorf("persist duplicated task: %w", err)
	}
	return &clone, nil
}

// PromoteTaskResult is the project and task after promote.
type PromoteTaskResult struct {
	ProjectID string
	Task      *domain.Task
}

// PromoteTask links a task to a project with the given title (reuse same-title
// active project in the same area when possible; otherwise create one).
func (c *appContext) PromoteTask(id, projectTitle string) (*PromoteTaskResult, error) {
	task, err := c.taskRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	now := time.Now()

	// Reuse an existing active (non-archived, non-deleted) project with the same title
	// in the same area when possible.
	var project *domain.Project
	projects, err := c.projectRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects {
		if p.DeletedAt != nil || p.Status == domain.ProjectStatusArchived {
			continue
		}
		if p.Title != projectTitle {
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
			Title:     projectTitle,
			Status:    domain.ProjectStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// Inherit the task's area when creating a new project.
		if task.AreaID != nil {
			areaID := *task.AreaID
			project.AreaID = &areaID
		}
		if err := c.PersistProject(project); err != nil {
			return nil, fmt.Errorf("persist new project: %w", err)
		}
	}

	task.ProjectID = &project.ID
	task.AreaID = nil
	task.UpdatedAt = now

	if err := c.PersistTask(task, now); err != nil {
		return nil, fmt.Errorf("persist task: %w", err)
	}

	return &PromoteTaskResult{
		ProjectID: project.ID,
		Task:      task,
	}, nil
}
