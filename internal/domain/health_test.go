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

func TestGetNextActionCandidates_SortOrder(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	pid := "p1"
	tasks := []*Task{
		{ID: "A", Title: "Alpha", Status: TaskStatusInbox, ProjectID: &pid, OrderNum: 2, CreatedAt: now},
		{ID: "B", Title: "Bravo", Status: TaskStatusWaiting, ProjectID: &pid, OrderNum: 2, CreatedAt: now.Add(time.Second)},
		{ID: "C", Title: "Charlie", Status: TaskStatusSomeday, ProjectID: &pid, OrderNum: 1, CreatedAt: now.Add(2 * time.Second)},
		{ID: "D", Title: "Delta", Status: TaskStatusInbox, ProjectID: &pid, OrderNum: 2, CreatedAt: now.Add(time.Second)},
		{ID: "next", Title: "Already next", Status: TaskStatusNext, ProjectID: &pid, OrderNum: 0, CreatedAt: now},
	}
	cands := GetNextActionCandidates(pid, tasks, "")
	want := []string{"C", "A", "B", "D"}
	if len(cands) != len(want) {
		t.Fatalf("got %d candidates, want %v", len(cands), want)
	}
	for i := range want {
		if cands[i].ID != want[i] {
			t.Errorf("order[%d] = %s, want %s", i, cands[i].ID, want[i])
		}
	}
}

func TestAgenda(t *testing.T) {
	// Stage 15 product rule for agenda:
	// - date-only due: calendar day comparison (date(due) <= date(now)) → due today included all day
	// - timed due: full timestamp (due <= now)
	// - startTime: start <= now
	// - exclude: done, archived, reference, soft-deleted
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	dueToday := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dueYesterday := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	dueTomorrow := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	timedFuture := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	timedPast := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	startPast := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	startFuture := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)

	tasks := []*Task{
		{ID: "due-today", Status: TaskStatusNext, DueDate: &dueToday},
		{ID: "due-yesterday", Status: TaskStatusNext, DueDate: &dueYesterday},
		{ID: "due-tomorrow", Status: TaskStatusNext, DueDate: &dueTomorrow},
		{ID: "timed-future", Status: TaskStatusNext, DueDate: &timedFuture},
		{ID: "timed-past", Status: TaskStatusNext, DueDate: &timedPast},
		{ID: "start-past", Status: TaskStatusNext, StartTime: &startPast},
		{ID: "start-future", Status: TaskStatusNext, StartTime: &startFuture},
		{ID: "ref", Status: TaskStatusReference, DueDate: &dueToday},
		{ID: "done", Status: TaskStatusDone, DueDate: &dueToday},
	}

	agenda := GetAgenda(tasks, now)
	got := map[string]bool{}
	for _, a := range agenda {
		got[a.ID] = true
	}

	mustInclude := []string{"due-today", "due-yesterday", "timed-past", "start-past"}
	mustExclude := []string{"due-tomorrow", "timed-future", "start-future", "ref", "done"}
	for _, id := range mustInclude {
		if !got[id] {
			t.Errorf("expected %s in agenda at noon on due day", id)
		}
	}
	for _, id := range mustExclude {
		if got[id] {
			t.Errorf("expected %s excluded from agenda", id)
		}
	}
}
