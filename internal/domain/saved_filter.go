package domain

import (
	"fmt"
	"time"
)

type SavedFilter struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Icon      string     `json:"icon,omitempty"`
	View      string     `json:"view"`
	Criteria  string     `json:"criteria"`
	SortBy    string     `json:"sortBy,omitempty"`
	SortOrder string     `json:"sortOrder,omitempty"`
	GroupBy   string     `json:"groupBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

func (f *SavedFilter) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("%w: saved filter ID cannot be empty", ErrValidation)
	}
	if f.Name == "" {
		return fmt.Errorf("%w: saved filter name cannot be empty", ErrValidation)
	}
	if f.View == "" {
		return fmt.Errorf("%w: saved filter view cannot be empty", ErrValidation)
	}
	if f.Criteria == "" {
		return fmt.Errorf("%w: saved filter criteria cannot be empty", ErrValidation)
	}
	return nil
}

func (f *SavedFilter) SoftDelete(now time.Time) {
	f.DeletedAt = &now
	f.UpdatedAt = now
}

func (f *SavedFilter) Restore(now time.Time) {
	f.DeletedAt = nil
	f.UpdatedAt = now
}
