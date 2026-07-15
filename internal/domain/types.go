package domain

import "time"

type TaskStatus string

const (
	TaskStatusInbox     TaskStatus = "inbox"
	TaskStatusNext      TaskStatus = "next"
	TaskStatusWaiting   TaskStatus = "waiting"
	TaskStatusSomeday   TaskStatus = "someday"
	TaskStatusReference TaskStatus = "reference"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusArchived  TaskStatus = "archived"
)

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusSomeday  ProjectStatus = "someday"
	ProjectStatusWaiting  ProjectStatus = "waiting"
	ProjectStatusArchived ProjectStatus = "archived"
)

type RelativeOffset struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

type Attachment struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	URI       string     `json:"uri"`
	MimeType  string     `json:"mimeType,omitempty"`
	Size      int        `json:"size,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type RecurrenceRule struct {
	Rule                 string     `json:"rule"`
	Strategy             string     `json:"strategy,omitempty"`
	ByDay                []string   `json:"byDay,omitempty"`
	ByMonthDay           []int      `json:"byMonthDay,omitempty"`
	WeekStart            string     `json:"weekStart,omitempty"`
	Count                int        `json:"count,omitempty"`
	Until                *time.Time `json:"until,omitempty"`
	CompletedOccurrences int        `json:"completedOccurrences,omitempty"`
	AnchorDay            int        `json:"anchorDay,omitempty"`
	DueAnchorDay         int        `json:"dueAnchorDay,omitempty"`
	StartAnchorDay       int        `json:"startAnchorDay,omitempty"`
	ReviewAnchorDay      int        `json:"reviewAnchorDay,omitempty"`
}
