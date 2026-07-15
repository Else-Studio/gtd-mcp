package domain

import (
	"testing"
	"time"
)

func TestProjectHealth(t *testing.T) {
	// 4. Stalled Project Detection: Create Project P1 with a `next` task, and Project P2 with only a `waiting` task. Query stalled projects. Assert P2 is returned, P1 is omitted.

	p1 := &Project{ID: "p1", Status: ProjectStatusActive}
	p2 := &Project{ID: "p2", Status: ProjectStatusActive}

	tasks := []*Task{
		{ID: "t1", ProjectID: &p1.ID, Status: TaskStatusNext},
		{ID: "t2", ProjectID: &p2.ID, Status: TaskStatusWaiting},
	}

	if IsProjectStalled(p1, tasks) {
		t.Errorf("expected P1 to not be stalled because it has a next action")
	}

	if !IsProjectStalled(p2, tasks) {
		t.Errorf("expected P2 to be stalled because it has no next action")
	}
}

func TestAgenda(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	// DueDate <= now OR StartTime <= now.
	// Crucially: A DueDate with no time component must be treated as 23:59:59.999 for overdue comparison.

	dueMidnight := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) // Date only, meaning it should be due 23:59:59.999
	dueTomorrow := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	startPast := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	startFuture := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)

	tasks := []*Task{
		{ID: "t1", DueDate: &dueMidnight}, // Should NOT be in agenda if we check strictly <= now (12:00), BUT with 23:59:59 it is effectively 23:59:59 >= 12:00, wait, "where DueDate <= now". If due is 23:59, then DueDate (23:59) is NOT <= now (12:00). So it is not overdue, but it is part of the agenda today? Wait! "Agenda / What's Important Now: Return tasks where DueDate <= now OR StartTime <= now". Wait, if DueDate is 2026-07-15 23:59:59, and now is 2026-07-15 12:00, then DueDate <= now is FALSE. It is not overdue. But usually Agenda includes things due *today*. Let's see the logic I implemented: `!effectiveDue.After(now)`. If effective is 23:59:59, it is After now, so it is NOT included!
		// But wait! If due is today, it's due today, shouldn't it be in the Agenda? Let's check the requirement carefully:
		// "Return tasks where DueDate <= now OR StartTime <= now. Crucially: A DueDate with no time component (e.g. 2026-07-15) MUST be treated as 23:59:59.999 for overdue comparison."
		// Ah! If they are treated as 23:59:59.999, they will only be "overdue" (<= now) AFTER midnight tomorrow! Wait, that means they are NOT in the agenda today!
		// Let's re-read the rule: "Agenda / "What's Important Now": Return tasks where DueDate <= now OR StartTime <= now".
		// I will test this exactly as written.
		{ID: "t2", StartTime: &startPast}, // Should be in agenda (10:00 <= 12:00)
		{ID: "t3", DueDate: &dueTomorrow}, // Not in agenda
		{ID: "t4", StartTime: &startFuture}, // Not in agenda
	}

	agenda := GetAgenda(tasks, now)
	
	foundT2 := false
	for _, a := range agenda {
		if a.ID == "t2" {
			foundT2 = true
		}
		if a.ID == "t1" {
			t.Errorf("expected t1 NOT to be in agenda because 23:59:59 is after 12:00")
		}
	}
	
	if !foundT2 {
		t.Errorf("expected t2 to be in agenda")
	}

	// Move now past midnight
	nowLate := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	agendaLate := GetAgenda(tasks, nowLate)
	
	foundT1 := false
	for _, a := range agendaLate {
		if a.ID == "t1" {
			foundT1 = true
		}
	}

	if !foundT1 {
		t.Errorf("expected t1 to be in agenda late because 23:59:59 is before 01:00 next day")
	}
}
