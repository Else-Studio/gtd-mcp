package domain

import (
	"testing"
	"time"
)

func TestCalculateNextRecurrence(t *testing.T) {
	// 1. Strict vs Fluid Math: Create a strict monthly task due June 15. Complete it on June 20. Assert new task is due July 15. Change to fluid. Complete on June 20. Assert new task is due July 20.
	
	baseDue := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	
	task := &Task{
		ID:      "t1",
		Status:  TaskStatusDone,
		DueDate: &baseDue,
		Recurrence: &RecurrenceRule{
			Rule:     "monthly",
			Strategy: "strict",
		},
	}

	newTask := task.DuplicateRecurringTask("t2", completedAt, TaskStatusDone)
	
	expectedStrictDue := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	if newTask.DueDate == nil || !newTask.DueDate.Equal(expectedStrictDue) {
		t.Errorf("expected strict math to produce %v, got %v", expectedStrictDue, newTask.DueDate)
	}

	task.Recurrence.Strategy = "fluid"
	newTaskFluid := task.DuplicateRecurringTask("t3", completedAt, TaskStatusDone)

	expectedFluidDue := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if newTaskFluid.DueDate == nil || !newTaskFluid.DueDate.Equal(expectedFluidDue) {
		t.Errorf("expected fluid math to produce %v, got %v", expectedFluidDue, newTaskFluid.DueDate)
	}
}

func TestRecurrenceMonthClamping(t *testing.T) {
	// 2. Month-end Clamping: Create a monthly task anchored on day 31. Advance to February. Assert date is Feb 28 (or 29). Advance another month. Assert date restores to March 31.

	jan31 := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:      "t1",
		Status:  TaskStatusDone,
		DueDate: &jan31,
		Recurrence: &RecurrenceRule{
			Rule:      "monthly",
			Strategy:  "strict",
			AnchorDay: 31,
		},
	}

	completedAtJan := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	febTask := task.DuplicateRecurringTask("t2", completedAtJan, TaskStatusDone)

	expectedFebDue := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC) // 2026 is not a leap year
	if febTask.DueDate == nil || !febTask.DueDate.Equal(expectedFebDue) {
		t.Errorf("expected Feb due date to clamp to %v, got %v", expectedFebDue, febTask.DueDate)
	}
	if febTask.Recurrence.AnchorDay != 31 {
		t.Errorf("expected anchor day to remain 31, got %v", febTask.Recurrence.AnchorDay)
	}

	completedAtFeb := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	marTask := febTask.DuplicateRecurringTask("t3", completedAtFeb, TaskStatusDone)

	expectedMarDue := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if marTask.DueDate == nil || !marTask.DueDate.Equal(expectedMarDue) {
		t.Errorf("expected March due date to restore to %v, got %v", expectedMarDue, marTask.DueDate)
	}
}

func TestRecurrenceLeapYearClamping(t *testing.T) {
	feb29 := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC) // 2024 is a leap year
	task := &Task{
		ID:      "t1",
		Status:  TaskStatusDone,
		DueDate: &feb29,
		Recurrence: &RecurrenceRule{
			Rule:      "yearly",
			Strategy:  "strict",
			AnchorDay: 29,
		},
	}

	completedAt2024 := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	task2025 := task.DuplicateRecurringTask("t2", completedAt2024, TaskStatusDone)

	expected2025Due := time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)
	if task2025.DueDate == nil || !task2025.DueDate.Equal(expected2025Due) {
		t.Errorf("expected 2025 due date to clamp to %v, got %v", expected2025Due, task2025.DueDate)
	}

	completedAt2025 := time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)
	task2026 := task2025.DuplicateRecurringTask("t3", completedAt2025, TaskStatusDone)

	expected2026Due := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	if task2026.DueDate == nil || !task2026.DueDate.Equal(expected2026Due) {
		t.Errorf("expected 2026 due date to remain %v, got %v", expected2026Due, task2026.DueDate)
	}
}

func TestRecurrenceDSTTransition(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("skipping DST test; America/New_York location data not available")
		return
	}

	mar7 := time.Date(2026, 3, 7, 9, 0, 0, 0, loc)
	task := &Task{
		ID:      "t1",
		Status:  TaskStatusDone,
		DueDate: &mar7,
		Recurrence: &RecurrenceRule{
			Rule:     "daily",
			Strategy: "strict",
		},
	}

	completedAt := time.Date(2026, 3, 7, 10, 0, 0, 0, loc)
	newTask := task.DuplicateRecurringTask("t2", completedAt, TaskStatusDone)

	if newTask.DueDate == nil {
		t.Fatalf("expected next due date to be set")
	}

	expectedDue := time.Date(2026, 3, 8, 9, 0, 0, 0, loc)
	if !newTask.DueDate.Equal(expectedDue) {
		t.Errorf("expected due date to remain 09:00:00 across DST transition, got %v (expected %v)", newTask.DueDate, expectedDue)
	}
}
