package domain

import (
	"fmt"
	"time"
)

type Project struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Status       ProjectStatus `json:"status"`
	Color        string        `json:"color,omitempty"`
	OrderNum     int           `json:"orderNum"`
	TagIDs       []string      `json:"tagIds,omitempty"`
	SupportNotes string        `json:"supportNotes,omitempty"`
	Attachments  []Attachment  `json:"attachments,omitempty"`
	DueDate      *time.Time    `json:"dueDate,omitempty"`
	ReviewAt     *time.Time    `json:"reviewAt,omitempty"`
	AreaID       *string       `json:"areaId,omitempty"`
	AreaTitle    string        `json:"areaTitle,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	DeletedAt    *time.Time    `json:"deletedAt,omitempty"`
}

func (p *Project) SoftDelete(now time.Time, tasks []*Task) {
	p.DeletedAt = &now
	p.UpdatedAt = now

	for _, t := range tasks {
		if t.ProjectID != nil && *t.ProjectID == p.ID {
			t.SoftDelete(now)
		}
	}
}

func (p *Project) Restore(now time.Time, tasks []*Task) {
	p.DeletedAt = nil
	p.UpdatedAt = now

	for _, t := range tasks {
		if t.ProjectID != nil && *t.ProjectID == p.ID {
			t.Restore(now)
		}
	}
}

func (p *Project) UpdateStatus(status ProjectStatus, now time.Time) {
	p.Status = status
	p.UpdatedAt = now
}

func (p *Project) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("%w: project ID cannot be empty", ErrValidation)
	}
	if p.Title == "" {
		return fmt.Errorf("%w: project title cannot be empty", ErrValidation)
	}
	switch p.Status {
	case ProjectStatusActive, ProjectStatusSomeday, ProjectStatusWaiting, ProjectStatusArchived:
		// Valid
	default:
		return fmt.Errorf("%w: invalid project status", ErrValidation)
	}
	return nil
}
