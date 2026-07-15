package domain

import (
	"fmt"
	"time"
)

type Task struct {
	ID                  string             `json:"id"`
	Title               string             `json:"title"`
	Description         string             `json:"description,omitempty"`
	Status              TaskStatus         `json:"status"`
	Priority            string             `json:"priority,omitempty"`
	EnergyLevel         string             `json:"energyLevel,omitempty"`
	AssignedTo          string             `json:"assignedTo,omitempty"`
	StartTime           *time.Time         `json:"startTime,omitempty"`
	RelativeStartOffset *RelativeOffset    `json:"relativeStartOffset,omitempty"`
	DueDate             *time.Time         `json:"dueDate,omitempty"`
	Recurrence          *RecurrenceRule    `json:"recurrence,omitempty"`
	Tags                []string           `json:"tags,omitempty"`
	Contexts            []string           `json:"contexts,omitempty"`
	TextDirection       string             `json:"textDirection,omitempty"`
	Attachments         []Attachment       `json:"attachments,omitempty"`
	Location            string             `json:"location,omitempty"`
	ProjectID           *string            `json:"projectId,omitempty"`
	SectionID           *string            `json:"sectionId,omitempty"`
	AreaID              *string            `json:"areaId,omitempty"`
	OrderNum            int                `json:"orderNum"`
	TimeEstimate        string             `json:"timeEstimate,omitempty"`
	TimeSpentMinutes    int                `json:"timeSpentMinutes,omitempty"`
	ReviewAt            *time.Time         `json:"reviewAt,omitempty"`
	CompletedAt         *time.Time         `json:"completedAt,omitempty"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
	DeletedAt           *time.Time         `json:"deletedAt,omitempty"`
}

func (t *Task) UpdateStatus(status TaskStatus, now time.Time) {
	t.Status = status
	t.UpdatedAt = now
	if status == TaskStatusDone || status == TaskStatusArchived {
		t.CompletedAt = &now
	} else {
		t.CompletedAt = nil
	}
}

func (t *Task) SoftDelete(now time.Time) {
	t.DeletedAt = &now
	t.UpdatedAt = now
}

func (t *Task) Restore(now time.Time) {
	t.DeletedAt = nil
	t.UpdatedAt = now
}

func (t *Task) SetReference() {
	t.Status = TaskStatusReference
	t.DueDate = nil
	t.StartTime = nil
	t.Priority = ""
	t.EnergyLevel = ""
	t.ReviewAt = nil
}

func (t *Task) SetProject(project *Project) {
	if project != nil {
		t.ProjectID = &project.ID
		t.AreaID = nil
		t.SectionID = nil
	}
}

func (t *Task) SetArea(area *Area) {
	if area != nil {
		t.AreaID = &area.ID
		t.ProjectID = nil
		t.SectionID = nil
	}
}

func (t *Task) SetSection(section *Section) {
	if section != nil {
		if t.ProjectID != nil && *t.ProjectID == section.ProjectID {
			t.SectionID = &section.ID
		}
	} else {
		t.SectionID = nil
	}
}

func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("%w: task ID cannot be empty", ErrValidation)
	}
	if t.Title == "" {
		return fmt.Errorf("%w: task title cannot be empty", ErrValidation)
	}
	switch t.Status {
	case TaskStatusInbox, TaskStatusNext, TaskStatusWaiting, TaskStatusSomeday, TaskStatusReference, TaskStatusDone, TaskStatusArchived:
		// Valid
	default:
		return fmt.Errorf("%w: invalid task status", ErrValidation)
	}
	return nil
}

func isDateOnly(t *time.Time) bool {
	return t != nil && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

func computeRelativeStartTime(due time.Time, offset *RelativeOffset) *time.Time {
	if offset == nil {
		return nil
	}
	if offset.Amount > 0 || offset.Amount < -10000 {
		return nil
	}
	if isDateOnly(&due) && (offset.Unit == "minute" || offset.Unit == "hour") {
		return nil
	}
	next := due
	switch offset.Unit {
	case "minute":
		next = next.Add(time.Duration(offset.Amount) * time.Minute)
	case "hour":
		next = next.Add(time.Duration(offset.Amount) * time.Hour)
	case "day":
		next = next.AddDate(0, 0, offset.Amount)
	case "week":
		next = next.AddDate(0, 0, offset.Amount*7)
	}
	return &next
}

func (t *Task) UpdateStartTime(newStart *time.Time) {
	t.StartTime = newStart
	t.RelativeStartOffset = nil // Link is broken by manual edit
}

func (t *Task) UpdateDueDate(newDue *time.Time) {
	t.DueDate = newDue
	if t.RelativeStartOffset != nil {
		if newDue == nil {
			t.RelativeStartOffset = nil
		} else {
			computed := computeRelativeStartTime(*newDue, t.RelativeStartOffset)
			if computed != nil {
				t.StartTime = computed
			} else {
				t.RelativeStartOffset = nil
			}
		}
	}
}

func (t *Task) UpdateRelativeStartOffset(offset *RelativeOffset) {
	t.RelativeStartOffset = offset
	if offset != nil && t.DueDate != nil {
		computed := computeRelativeStartTime(*t.DueDate, offset)
		if computed != nil {
			t.StartTime = computed
		} else {
			t.RelativeStartOffset = nil
		}
	}
}
