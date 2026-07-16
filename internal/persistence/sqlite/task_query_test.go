package sqlite_test

import (
	"context"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/sqlite"
)

// TestAgenda_DomainAndSQLParity locks R15: domain.GetAgenda and SQL ListAgendaTasks
// return the same membership set for shared fixtures.
func TestAgenda_DomainAndSQLParity(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	timedFuture := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	timedPast := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)

	fixtures := []*domain.Task{
		{ID: "due-today", Title: "Due today", Status: domain.TaskStatusNext, DueDate: &today, CreatedAt: now, UpdatedAt: now},
		{ID: "due-tomorrow", Title: "Due tomorrow", Status: domain.TaskStatusNext, DueDate: &tomorrow, CreatedAt: now, UpdatedAt: now},
		{ID: "ref", Title: "Ref", Status: domain.TaskStatusReference, DueDate: &today, CreatedAt: now, UpdatedAt: now},
		{ID: "timed-future", Title: "Timed future", Status: domain.TaskStatusNext, DueDate: &timedFuture, CreatedAt: now, UpdatedAt: now},
		{ID: "timed-past", Title: "Timed past", Status: domain.TaskStatusNext, DueDate: &timedPast, CreatedAt: now, UpdatedAt: now},
	}
	for _, task := range fixtures {
		if err := engine.SyncTask(context.Background(), task, now); err != nil {
			t.Fatalf("sync %s: %v", task.ID, err)
		}
	}

	domainIDs := map[string]bool{}
	for _, tsk := range domain.GetAgenda(fixtures, now) {
		domainIDs[tsk.ID] = true
	}

	q := sqlite.NewTaskQuery(db)
	sqlIDs, err := q.ListAgendaTasks(context.Background(), now, nil)
	if err != nil {
		t.Fatalf("ListAgendaTasks: %v", err)
	}
	sqlSet := map[string]bool{}
	for _, id := range sqlIDs {
		sqlSet[id] = true
	}

	if len(domainIDs) != len(sqlSet) {
		t.Errorf("domain count %d != sql count %d (domain=%v sql=%v)", len(domainIDs), len(sqlSet), domainIDs, sqlSet)
	}
	for id := range domainIDs {
		if !sqlSet[id] {
			t.Errorf("domain includes %s but SQL does not", id)
		}
	}
	for id := range sqlSet {
		if !domainIDs[id] {
			t.Errorf("SQL includes %s but domain does not", id)
		}
	}
}

// TestListActiveTasks_DueDateNullsLast locks technical API §6.B:
// within same status, dated tasks first (ascending due), then undated.
func TestListActiveTasks_DueDateNullsLast(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	dueA := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	dueC := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	tasks := []*domain.Task{
		{ID: "A", Title: "Later due", Status: domain.TaskStatusNext, DueDate: &dueA, CreatedAt: now, UpdatedAt: now},
		{ID: "B", Title: "No due", Status: domain.TaskStatusNext, CreatedAt: now.Add(time.Second), UpdatedAt: now},
		{ID: "C", Title: "Earliest due", Status: domain.TaskStatusNext, DueDate: &dueC, CreatedAt: now.Add(2 * time.Second), UpdatedAt: now},
	}
	for _, task := range tasks {
		if err := engine.SyncTask(context.Background(), task, now); err != nil {
			t.Fatalf("sync %s: %v", task.ID, err)
		}
	}

	q := sqlite.NewTaskQuery(db)
	ids, err := q.ListActiveTasks(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListActiveTasks: %v", err)
	}
	want := []string{"C", "A", "B"}
	if len(ids) != len(want) {
		t.Fatalf("got %v ids, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("order[%d] = %s, want %s (full=%v)", i, ids[i], want[i], ids)
		}
	}

	// Same order for status-filtered next list.
	nextIDs, err := q.ListTasksByStatus(context.Background(), "next", nil)
	if err != nil {
		t.Fatalf("ListTasksByStatus next: %v", err)
	}
	if len(nextIDs) != len(want) {
		t.Fatalf("next list got %v, want %v", nextIDs, want)
	}
	for i := range want {
		if nextIDs[i] != want[i] {
			t.Errorf("next order[%d] = %s, want %s (full=%v)", i, nextIDs[i], want[i], nextIDs)
		}
	}
}

// TestGetProjectCandidates_SortOrder locks technical API §7.B:
// candidates ordered by orderNum → createdAt → title.
func TestGetProjectCandidates_SortOrder(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	projectID := "proj-cand"

	if err := engine.SyncProject(context.Background(), &domain.Project{
		ID:        projectID,
		Title:     "Cand Project",
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("sync project: %v", err)
	}

	// Seed inbox candidates with deliberate orderNum / createdAt / title.
	// Expected order: orderNum 1 (C), orderNum 2 earlier created (A), orderNum 2 later created (B by title before D if same time).
	seed := []*domain.Task{
		{ID: "A", Title: "Alpha", Status: domain.TaskStatusInbox, ProjectID: &projectID, OrderNum: 2, CreatedAt: now, UpdatedAt: now},
		{ID: "B", Title: "Bravo", Status: domain.TaskStatusWaiting, ProjectID: &projectID, OrderNum: 2, CreatedAt: now.Add(time.Second), UpdatedAt: now},
		{ID: "C", Title: "Charlie", Status: domain.TaskStatusSomeday, ProjectID: &projectID, OrderNum: 1, CreatedAt: now.Add(2 * time.Second), UpdatedAt: now},
		{ID: "D", Title: "Delta", Status: domain.TaskStatusInbox, ProjectID: &projectID, OrderNum: 2, CreatedAt: now.Add(time.Second), UpdatedAt: now},
	}
	for _, task := range seed {
		if err := engine.SyncTask(context.Background(), task, now); err != nil {
			t.Fatalf("sync %s: %v", task.ID, err)
		}
	}

	q := sqlite.NewTaskQuery(db)
	cands, err := q.GetProjectCandidates(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProjectCandidates: %v", err)
	}
	// C (order 1), then A (order 2, earliest created), then B before D by title (same createdAt).
	want := []string{"C", "A", "B", "D"}
	if len(cands) != len(want) {
		t.Fatalf("got %d candidates %v, want %v", len(cands), idsOf(cands), want)
	}
	for i := range want {
		if cands[i].ID != want[i] {
			t.Errorf("order[%d] = %s, want %s (full=%v)", i, cands[i].ID, want[i], idsOf(cands))
		}
	}
}

func idsOf(tasks []domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

// TestListAgendaTasks_DateOnlyTodayAndReferenceExcluded aligns SQL agenda
// with Stage 15 product rules (calendar-day date-only; exclude reference).
func TestListAgendaTasks_DateOnlyTodayAndReferenceExcluded(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	today := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	timedFuture := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	timedPast := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)

	seed := []*domain.Task{
		{ID: "due-today", Title: "Due today", Status: domain.TaskStatusNext, DueDate: &today, CreatedAt: now, UpdatedAt: now},
		{ID: "due-tomorrow", Title: "Due tomorrow", Status: domain.TaskStatusNext, DueDate: &tomorrow, CreatedAt: now, UpdatedAt: now},
		{ID: "ref", Title: "Ref note", Status: domain.TaskStatusReference, DueDate: &today, CreatedAt: now, UpdatedAt: now},
		{ID: "timed-future", Title: "Timed future", Status: domain.TaskStatusNext, DueDate: &timedFuture, CreatedAt: now, UpdatedAt: now},
		{ID: "timed-past", Title: "Timed past", Status: domain.TaskStatusNext, DueDate: &timedPast, CreatedAt: now, UpdatedAt: now},
	}
	for _, task := range seed {
		if err := engine.SyncTask(context.Background(), task, now); err != nil {
			t.Fatalf("sync %s: %v", task.ID, err)
		}
	}

	q := sqlite.NewTaskQuery(db)
	ids, err := q.ListAgendaTasks(context.Background(), now, nil)
	if err != nil {
		t.Fatalf("ListAgendaTasks: %v", err)
	}

	has := map[string]bool{}
	for _, id := range ids {
		has[id] = true
	}
	if !has["due-today"] {
		t.Error("expected date-only due today to be included")
	}
	if has["due-tomorrow"] {
		t.Error("expected date-only due tomorrow to be excluded")
	}
	if has["ref"] {
		t.Error("expected reference status to be excluded")
	}
	if has["timed-future"] {
		t.Error("expected timed due later today to be excluded")
	}
	if !has["timed-past"] {
		t.Error("expected timed due in the past to be included")
	}
}
