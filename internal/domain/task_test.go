package domain

import (
	"testing"
	"time"
)

func TestTaskExclusivity(t *testing.T) {
	area := &Area{ID: "area1"}
	project := &Project{ID: "project1"}
	task := &Task{ID: "task1"}

	// Assign to area
	task.SetArea(area)
	if task.AreaID == nil || *task.AreaID != "area1" {
		t.Errorf("expected task to be in area1, got %v", task.AreaID)
	}
	if task.ProjectID != nil {
		t.Errorf("expected ProjectID to be nil")
	}

	// Assign to project
	task.SetProject(project)
	if task.ProjectID == nil || *task.ProjectID != "project1" {
		t.Errorf("expected task to be in project1, got %v", task.ProjectID)
	}
	if task.AreaID != nil {
		t.Errorf("expected AreaID to be nil, got %v", *task.AreaID)
	}
}


func TestTaskCompletion(t *testing.T) {
	task := &Task{ID: "task1", Status: TaskStatusNext}
	now := time.Now().Truncate(time.Second)
	task.UpdateStatus(TaskStatusDone, now)

	if task.Status != TaskStatusDone {
		t.Errorf("expected status done, got %s", task.Status)
	}
	if task.CompletedAt == nil || !task.CompletedAt.Equal(now) {
		t.Errorf("expected CompletedAt to be %v, got %v", now, task.CompletedAt)
	}
}

func TestTaskReversion(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	task := &Task{ID: "task1", Status: TaskStatusDone, CompletedAt: &now}
	task.UpdateStatus(TaskStatusNext, now)

	if task.Status != TaskStatusNext {
		t.Errorf("expected status next, got %s", task.Status)
	}
	if task.CompletedAt != nil {
		t.Errorf("expected CompletedAt to be nil")
	}
}

func TestTaskReferenceConversion(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:          "task1",
		Status:      TaskStatusNext,
		DueDate:     &now,
		StartTime:   &now,
		Priority:    "high",
		EnergyLevel: "high",
		ReviewAt:    &now,
	}

	task.SetReference()

	if task.Status != TaskStatusReference {
		t.Errorf("expected status reference, got %s", task.Status)
	}
	if task.DueDate != nil {
		t.Errorf("expected DueDate to be nil")
	}
	if task.StartTime != nil {
		t.Errorf("expected StartTime to be nil")
	}
	if task.Priority != "" {
		t.Errorf("expected Priority to be empty, got %s", task.Priority)
	}
	if task.EnergyLevel != "" {
		t.Errorf("expected EnergyLevel to be empty, got %s", task.EnergyLevel)
	}
	if task.ReviewAt != nil {
		t.Errorf("expected ReviewAt to be nil")
	}
}

func TestTaskValidation(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: &Task{
				ID:     "t1",
				Title:  "Title",
				Status: TaskStatusNext,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			task: &Task{
				Title:  "Title",
				Status: TaskStatusNext,
			},
			wantErr: true,
		},
		{
			name: "missing title",
			task: &Task{
				ID:     "t1",
				Status: TaskStatusNext,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			task: &Task{
				ID:     "t1",
				Title:  "Title",
				Status: "banana",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTaskRelativeStartTime(t *testing.T) {
	// 3. Relative Offset Tracking: Task has DueDate 2026-03-12, offset 0 day. Update DueDate to 2026-03-19. Assert StartTime automatically updates to 2026-03-19.
	due := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:      "t1",
		DueDate: &due,
	}
	
	offset := &RelativeOffset{Amount: 0, Unit: "day"}
	task.UpdateRelativeStartOffset(offset)
	if task.StartTime == nil || !task.StartTime.Equal(due) {
		t.Errorf("expected StartTime to be %v, got %v", due, task.StartTime)
	}

	newDue := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	task.UpdateDueDate(&newDue)
	if task.StartTime == nil || !task.StartTime.Equal(newDue) {
		t.Errorf("expected StartTime to update to %v, got %v", newDue, task.StartTime)
	}
}

func TestTaskRelativeStartTimeGranularity(t *testing.T) {
	due := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:      "t1",
		DueDate: &due,
	}
	
	offset := &RelativeOffset{Amount: -30, Unit: "minute"}
	task.UpdateRelativeStartOffset(offset)
	if task.StartTime != nil {
		t.Errorf("expected StartTime to be nil because of granularity guard, got %v", task.StartTime)
	}
	if task.RelativeStartOffset != nil {
		t.Errorf("expected RelativeStartOffset to be nil because of granularity guard")
	}

	// Update with explicit time
	timeDue := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)
	task.UpdateDueDate(&timeDue)
	task.UpdateRelativeStartOffset(offset)

	expectedStart := time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC)
	if task.StartTime == nil || !task.StartTime.Equal(expectedStart) {
		t.Errorf("expected StartTime to be %v, got %v", expectedStart, task.StartTime)
	}
}

func TestTaskRelativeStartTimeBounds(t *testing.T) {
	due := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:      "t1",
		DueDate: &due,
	}

	// Positive offset is invalid
	posOffset := &RelativeOffset{Amount: 1, Unit: "day"}
	task.UpdateRelativeStartOffset(posOffset)
	if task.StartTime != nil {
		t.Errorf("expected StartTime to be nil for positive offset, got %v", task.StartTime)
	}
	if task.RelativeStartOffset != nil {
		t.Errorf("expected RelativeStartOffset to be nil for positive offset")
	}

	// Extremely negative offset is invalid (< -10000)
	negOffset := &RelativeOffset{Amount: -10001, Unit: "day"}
	task.UpdateRelativeStartOffset(negOffset)
	if task.StartTime != nil {
		t.Errorf("expected StartTime to be nil for out of bounds negative offset, got %v", task.StartTime)
	}
	if task.RelativeStartOffset != nil {
		t.Errorf("expected RelativeStartOffset to be nil for out of bounds negative offset")
	}
}

func TestTaskStatusTransitions(t *testing.T) {
	// Setup: Create a task in TaskStatusDone with a CompletedAt timestamp.
	now := time.Now().Truncate(time.Second)
	task := &Task{
		ID:          "task1",
		Status:      TaskStatusDone,
		CompletedAt: &now,
	}

	// Action: Update status to TaskStatusNext
	task.UpdateStatus(TaskStatusNext, now.Add(time.Second))
	// Outcome: Assert CompletedAt is cleared when moved to Next.
	if task.Status != TaskStatusNext {
		t.Errorf("expected status next, got %s", task.Status)
	}
	if task.CompletedAt != nil {
		t.Errorf("expected CompletedAt to be nil when reverted from done")
	}

	// Action: Update status to TaskStatusArchived
	archivedTime := now.Add(2 * time.Second)
	task.UpdateStatus(TaskStatusArchived, archivedTime)
	// Outcome: Assert CompletedAt is populated when Archived.
	if task.Status != TaskStatusArchived {
		t.Errorf("expected status archived, got %s", task.Status)
	}
	if task.CompletedAt == nil || !task.CompletedAt.Equal(archivedTime) {
		t.Errorf("expected CompletedAt to be %v, got %v", archivedTime, task.CompletedAt)
	}
}

func TestTaskSoftDeleteAndRestore(t *testing.T) {
	// Setup: Create an independent task.
	task := &Task{ID: "task1"}
	now := time.Now().Truncate(time.Second)

	// Action: Call SoftDelete(now)
	task.SoftDelete(now)
	// Outcome: Assert DeletedAt matches 'now' after SoftDelete.
	if task.DeletedAt == nil || !task.DeletedAt.Equal(now) {
		t.Errorf("expected DeletedAt to be %v, got %v", now, task.DeletedAt)
	}
	if !task.UpdatedAt.Equal(now) {
		t.Errorf("expected UpdatedAt to be %v, got %v", now, task.UpdatedAt)
	}

	// Action: Call Restore(now)
	restoreTime := now.Add(time.Second)
	task.Restore(restoreTime)
	// Outcome: Assert DeletedAt is nil after Restore, and UpdatedAt changes.
	if task.DeletedAt != nil {
		t.Errorf("expected DeletedAt to be nil after restore")
	}
	if !task.UpdatedAt.Equal(restoreTime) {
		t.Errorf("expected UpdatedAt to be %v, got %v", restoreTime, task.UpdatedAt)
	}
}

func TestTaskSetProjectAndAreaNil(t *testing.T) {
	// Setup: Create a task.
	task := &Task{ID: "task1"}

	// Action: Call SetProject(nil) and SetArea(nil)
	task.SetProject(nil)
	task.SetArea(nil)

	// Outcome: Assert IDs remain nil.
	if task.ProjectID != nil {
		t.Errorf("expected ProjectID to remain nil")
	}
	if task.AreaID != nil {
		t.Errorf("expected AreaID to remain nil")
	}
}

func TestTaskUpdateDueDate_EdgeCases(t *testing.T) {
	// Setup: Create a task with a DueDate and a RelativeStartOffset.
	due := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:      "t1",
		DueDate: &due,
	}
	offset := &RelativeOffset{Amount: -2, Unit: "day"}
	task.UpdateRelativeStartOffset(offset)

	// Action: UpdateDueDate(nil)
	task.UpdateDueDate(nil)

	// Outcome: Assert RelativeStartOffset is set to nil.
	if task.RelativeStartOffset != nil {
		t.Errorf("expected RelativeStartOffset to be nil after setting DueDate to nil")
	}
}
