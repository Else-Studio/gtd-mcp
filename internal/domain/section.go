package domain

import (
	"fmt"
	"time"
)

type Section struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"projectId"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	OrderNum    int        `json:"orderNum"`
	IsCollapsed bool       `json:"isCollapsed"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

func (s *Section) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("%w: section ID cannot be empty", ErrValidation)
	}
	if s.ProjectID == "" {
		return fmt.Errorf("%w: section projectId cannot be empty", ErrValidation)
	}
	if s.Title == "" {
		return fmt.Errorf("%w: section title cannot be empty", ErrValidation)
	}
	return nil
}

func (s *Section) SoftDelete(now time.Time) {
	s.DeletedAt = &now
	s.UpdatedAt = now
}

func (s *Section) Restore(now time.Time) {
	s.DeletedAt = nil
	s.UpdatedAt = now
}
