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

func (a *Area) SoftDelete(now time.Time) {
	a.DeletedAt = &now
	a.UpdatedAt = now
}

func (a *Area) Restore(now time.Time) {
	a.DeletedAt = nil
	a.UpdatedAt = now
}
