package domain

import (
	"fmt"
	"time"
)

type Area struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Color     string     `json:"color,omitempty"`
	Icon      string     `json:"icon,omitempty"`
	OrderNum  int        `json:"orderNum"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

func (a *Area) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("%w: area ID cannot be empty", ErrValidation)
	}
	if a.Name == "" {
		return fmt.Errorf("%w: area name cannot be empty", ErrValidation)
	}
	return nil
}

func (a *Area) SoftDelete(now time.Time, projects []*Project, tasks []*Task) {
	a.DeletedAt = &now
	a.UpdatedAt = now

	for _, p := range projects {
		if p.AreaID != nil && *p.AreaID == a.ID {
			p.SoftDelete(now, tasks)
		}
	}

	for _, t := range tasks {
		if t.AreaID != nil && *t.AreaID == a.ID {
			t.SoftDelete(now)
		}
	}
}

func (a *Area) Restore(now time.Time, projects []*Project, tasks []*Task) {
	a.DeletedAt = nil
	a.UpdatedAt = now

	for _, p := range projects {
		if p.AreaID != nil && *p.AreaID == a.ID {
			p.Restore(now, tasks)
		}
	}

	for _, t := range tasks {
		if t.AreaID != nil && *t.AreaID == a.ID {
			t.Restore(now)
		}
	}
}
