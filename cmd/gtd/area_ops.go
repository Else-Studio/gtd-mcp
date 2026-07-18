package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gtd/internal/domain"
)

// CreateArea creates a new Area of Focus with the given name.
func (c *appContext) CreateArea(name string) (*domain.Area, error) {
	area := &domain.Area{
		ID:   uuid.New().String(),
		Name: name,
	}

	if err := c.PersistArea(area); err != nil {
		return nil, fmt.Errorf("persist area: %w", err)
	}
	return area, nil
}

// UpdateAreaOptions holds fields that may be changed on an area.
type UpdateAreaOptions struct {
	Name string // empty = no change
}

// UpdateArea loads an area, applies name change if set, and persists.
func (c *appContext) UpdateArea(id string, opts UpdateAreaOptions) (*domain.Area, error) {
	area, err := c.areaRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("area not found: %w", err)
	}

	if opts.Name != "" {
		area.Name = opts.Name
		area.UpdatedAt = time.Now()
	}

	if err := c.PersistArea(area); err != nil {
		return nil, fmt.Errorf("persist area: %w", err)
	}
	return area, nil
}

// DeleteArea soft-deletes an area and cascades soft-delete to child projects and tasks.
func (c *appContext) DeleteArea(id string) (*domain.Area, error) {
	area, err := c.areaRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("area not found: %w", err)
	}

	projects, err := c.projectRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	tasks, err := c.taskRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	now := time.Now()
	area.SoftDelete(now, projects, tasks)

	if err := c.PersistArea(area); err != nil {
		return nil, fmt.Errorf("persist area: %w", err)
	}

	// Persist cascade soft-deletes on child projects and tasks.
	cascadedProjects := map[string]bool{}
	for _, p := range projects {
		if p.AreaID != nil && *p.AreaID == area.ID {
			cascadedProjects[p.ID] = true
			if err := c.PersistProject(p); err != nil {
				return nil, fmt.Errorf("persist cascaded project: %w", err)
			}
		}
	}
	for _, t := range tasks {
		underArea := t.AreaID != nil && *t.AreaID == area.ID
		underCascadedProject := t.ProjectID != nil && cascadedProjects[*t.ProjectID]
		if !underArea && !underCascadedProject {
			continue
		}
		if err := c.PersistTask(t, now); err != nil {
			return nil, fmt.Errorf("persist cascaded task: %w", err)
		}
	}
	return area, nil
}

// RestoreArea restores a soft-deleted area and cascades restore to child projects and tasks.
func (c *appContext) RestoreArea(id string) (*domain.Area, error) {
	area, err := c.areaRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("area not found: %w", err)
	}

	projects, err := c.projectRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	tasks, err := c.taskRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	now := time.Now()
	area.Restore(now, projects, tasks)

	if err := c.PersistArea(area); err != nil {
		return nil, fmt.Errorf("persist area: %w", err)
	}

	cascadedProjects := map[string]bool{}
	for _, p := range projects {
		if p.AreaID != nil && *p.AreaID == area.ID {
			cascadedProjects[p.ID] = true
			if err := c.PersistProject(p); err != nil {
				return nil, fmt.Errorf("persist cascaded project: %w", err)
			}
		}
	}
	for _, t := range tasks {
		underArea := t.AreaID != nil && *t.AreaID == area.ID
		underCascadedProject := t.ProjectID != nil && cascadedProjects[*t.ProjectID]
		if !underArea && !underCascadedProject {
			continue
		}
		if err := c.PersistTask(t, now); err != nil {
			return nil, fmt.Errorf("persist cascaded task: %w", err)
		}
	}
	return area, nil
}

// ListActiveAreas returns non-deleted areas.
func (c *appContext) ListActiveAreas() ([]*domain.Area, error) {
	areas, err := c.areaRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}

	activeAreas := make([]*domain.Area, 0)
	for _, a := range areas {
		if a.DeletedAt == nil {
			activeAreas = append(activeAreas, a)
		}
	}
	return activeAreas, nil
}
