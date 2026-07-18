package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gtd/internal/domain"
)

// CreateProjectOptions holds title and optional area binding for project add.
type CreateProjectOptions struct {
	Title    string
	AreaID   string
	AreaName string // find-or-create area when set and AreaID empty
}

// CreateProject creates an active project, optionally binding (or creating) an area.
func (c *appContext) CreateProject(opts CreateProjectOptions) (*domain.Project, error) {
	project := &domain.Project{
		ID:     uuid.New().String(),
		Title:  opts.Title,
		Status: domain.ProjectStatusActive,
	}

	areaID := opts.AreaID
	if opts.AreaName != "" && areaID == "" {
		resolved, err := c.findOrCreateAreaByName(opts.AreaName)
		if err != nil {
			return nil, err
		}
		areaID = resolved.ID
	}

	if areaID != "" {
		project.AreaID = &areaID
	}

	if err := c.PersistProject(project); err != nil {
		return nil, fmt.Errorf("persist project: %w", err)
	}
	return project, nil
}

// UpdateProjectOptions holds status and area updates for project update.
// AreaID.Set or non-empty resolved area applies area change (including clear).
type UpdateProjectOptions struct {
	Status       string // empty = no change
	AreaID       optionalString
	AreaName     string // if set and AreaID.Value empty, find-or-create then set
	AreaFlagUsed bool   // true when CLI Changed("area-id") or resolved areaID != ""
}

// UpdateProject loads a project, applies status/area updates, and persists.
func (c *appContext) UpdateProject(id string, opts UpdateProjectOptions) (*domain.Project, error) {
	project, err := c.projectRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	if opts.Status != "" {
		project.UpdateStatus(domain.ProjectStatus(opts.Status), time.Now())
	}

	areaID := ""
	if opts.AreaID.Set {
		areaID = opts.AreaID.Value
	}
	if opts.AreaName != "" && areaID == "" {
		resolved, err := c.findOrCreateAreaByName(opts.AreaName)
		if err != nil {
			return nil, err
		}
		areaID = resolved.ID
	}

	// Match prior CLI: Changed("area-id") || areaID != ""
	if opts.AreaFlagUsed || areaID != "" {
		if areaID == "" {
			project.AreaID = nil
		} else {
			project.AreaID = &areaID
		}
		project.UpdatedAt = time.Now()
	}

	if err := c.PersistProject(project); err != nil {
		return nil, fmt.Errorf("persist project: %w", err)
	}
	return project, nil
}

// findOrCreateAreaByName returns an active area with the given name, creating it if needed.
func (c *appContext) findOrCreateAreaByName(areaName string) (*domain.Area, error) {
	now := time.Now()
	areas, _ := c.areaRepo.List()
	var found *domain.Area
	for _, a := range areas {
		if a.Name == areaName && a.DeletedAt == nil {
			found = a
			break
		}
	}
	if found == nil {
		found = &domain.Area{
			ID:        uuid.New().String(),
			Name:      areaName,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := c.PersistArea(found); err != nil {
			return nil, fmt.Errorf("persist new area: %w", err)
		}
	}
	return found, nil
}

// DeleteProject soft-deletes a project and cascades soft-delete to child tasks.
func (c *appContext) DeleteProject(id string) (*domain.Project, error) {
	project, err := c.projectRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	tasks, err := c.taskRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	now := time.Now()
	project.SoftDelete(now, tasks)

	if err := c.PersistProject(project); err != nil {
		return nil, fmt.Errorf("persist project: %w", err)
	}

	// Persist cascade soft-deletes on child tasks.
	for _, t := range tasks {
		if t.ProjectID != nil && *t.ProjectID == project.ID {
			if err := c.PersistTask(t, now); err != nil {
				return nil, fmt.Errorf("persist cascaded task: %w", err)
			}
		}
	}
	return project, nil
}

// RestoreProject restores a soft-deleted project and cascades restore to child tasks.
func (c *appContext) RestoreProject(id string) (*domain.Project, error) {
	project, err := c.projectRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	tasks, err := c.taskRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	now := time.Now()
	if err := c.restoreProjectWithCascade(project, tasks, now); err != nil {
		return nil, err
	}
	return project, nil
}

// restoreProjectWithCascade restores a project and persists every child task
// that belongs to it. Fail-closed: any Persist* error aborts the cascade.
func (c *appContext) restoreProjectWithCascade(project *domain.Project, tasks []*domain.Task, now time.Time) error {
	project.Restore(now, tasks)

	if err := c.PersistProject(project); err != nil {
		return fmt.Errorf("persist project: %w", err)
	}

	for _, t := range tasks {
		if t.ProjectID != nil && *t.ProjectID == project.ID {
			if err := c.PersistTask(t, now); err != nil {
				return fmt.Errorf("persist cascaded task: %w", err)
			}
		}
	}
	return nil
}

// ListActiveProjectIDs returns IDs of non-deleted projects.
func (c *appContext) ListActiveProjectIDs() ([]string, error) {
	projects, err := c.projectRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	ids := []string{}
	for _, p := range projects {
		if p.DeletedAt == nil {
			ids = append(ids, p.ID)
		}
	}
	return ids, nil
}
