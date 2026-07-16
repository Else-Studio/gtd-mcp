package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"gtd/internal/domain"
)

type SyncEngine struct {
	db          *sql.DB
	taskRepo    domain.TaskRepository
	projectRepo domain.ProjectRepository
	areaRepo    domain.AreaRepository
	personRepo  domain.PersonRepository
}

func NewSyncEngine(
	db *sql.DB,
	taskRepo domain.TaskRepository,
	projectRepo domain.ProjectRepository,
	areaRepo domain.AreaRepository,
	personRepo domain.PersonRepository,
) *SyncEngine {
	return &SyncEngine{
		db:          db,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		areaRepo:    areaRepo,
		personRepo:  personRepo,
	}
}

func (s *SyncEngine) Sync(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing cache since it's a full sync
	tables := []string{"tasks", "projects", "areas", "people"}
	for _, t := range tables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return err
		}
	}

	if s.areaRepo != nil {
		areas, err := s.areaRepo.List()
		if err != nil {
			return err
		}
		for _, a := range areas {
			if err := insertArea(tx, a); err != nil {
				return err
			}
		}
	}

	if s.projectRepo != nil {
		projects, err := s.projectRepo.List()
		if err != nil {
			return err
		}
		for _, p := range projects {
			if err := insertProject(tx, p); err != nil {
				return err
			}
		}
	}



	if s.personRepo != nil {
		people, err := s.personRepo.List()
		if err != nil {
			return err
		}
		for _, p := range people {
			if err := insertPerson(tx, p); err != nil {
				return err
			}
		}
	}



	if s.taskRepo != nil {
		tasks, err := s.taskRepo.List()
		if err != nil {
			return err
		}
		for _, t := range tasks {
			NormalizeTaskForLoad(t, now)
			if err := insertTask(tx, t); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *SyncEngine) SyncTask(ctx context.Context, t *domain.Task, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tasks WHERE id = ?", t.ID); err != nil {
		return err
	}

	NormalizeTaskForLoad(t, now)
	if err := insertTask(tx, t); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SyncEngine) SyncProject(ctx context.Context, p *domain.Project) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM projects WHERE id = ?", p.ID); err != nil {
		return err
	}
	if err := insertProject(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SyncEngine) SyncArea(ctx context.Context, a *domain.Area) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM areas WHERE id = ?", a.ID); err != nil {
		return err
	}
	if err := insertArea(tx, a); err != nil {
		return err
	}
	return tx.Commit()
}



func (s *SyncEngine) SyncPerson(ctx context.Context, p *domain.Person) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM people WHERE id = ?", p.ID); err != nil {
		return err
	}
	if err := insertPerson(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}



func NormalizeTaskForLoad(t *domain.Task, now time.Time) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	if t.UpdatedAt.Before(t.CreatedAt) {
		t.UpdatedAt = t.CreatedAt
	}

	if t.ProjectID != nil {
		t.AreaID = nil
	}

	switch t.Status {
	case domain.TaskStatusDone, domain.TaskStatusArchived:
		if t.CompletedAt == nil {
			t.CompletedAt = &t.UpdatedAt
		}
	default:
		if t.CompletedAt != nil {
			t.CompletedAt = nil
		}
	}
}

func insertArea(tx *sql.Tx, a *domain.Area) error {
	query := `INSERT INTO areas (id, name, color, icon, orderNum, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		a.ID, a.Name, a.Color, a.Icon, a.OrderNum,
		timeString(a.CreatedAt), timeString(a.UpdatedAt), timePtrString(a.DeletedAt),
	)
	return err
}

func insertProject(tx *sql.Tx, p *domain.Project) error {
	query := `INSERT INTO projects (id, title, status, color, orderNum, tagIds, supportNotes, attachments, dueDate, reviewAt, areaId, areaTitle, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		p.ID, p.Title, string(p.Status), p.Color, p.OrderNum,
		jsonString(p.TagIDs), p.SupportNotes, jsonString(p.Attachments),
		timePtrString(p.DueDate), timePtrString(p.ReviewAt), p.AreaID, p.AreaTitle,
		timeString(p.CreatedAt), timeString(p.UpdatedAt), timePtrString(p.DeletedAt),
	)
	return err
}

func insertTask(tx *sql.Tx, t *domain.Task) error {
	query := `INSERT INTO tasks (
		id, title, status, priority, energyLevel, assignedTo, startTime, relativeStartOffset,
		dueDate, recurrence, tags, contexts, description, textDirection, attachments, location,
		projectId, areaId, orderNum, timeEstimate, timeSpentMinutes, reviewAt, completedAt,
		createdAt, updatedAt, deletedAt
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.Exec(query,
		t.ID, t.Title, string(t.Status), t.Priority, t.EnergyLevel, t.AssignedTo,
		timePtrString(t.StartTime), jsonString(t.RelativeStartOffset),
		timePtrString(t.DueDate), jsonString(t.Recurrence),
		jsonString(t.Tags), jsonString(t.Contexts), t.Description, t.TextDirection,
		jsonString(t.Attachments), t.Location, t.ProjectID, t.AreaID,
		t.OrderNum, t.TimeEstimate, t.TimeSpentMinutes, timePtrString(t.ReviewAt),
		timePtrString(t.CompletedAt), timeString(t.CreatedAt), timeString(t.UpdatedAt), timePtrString(t.DeletedAt),
	)
	return err
}



func insertPerson(tx *sql.Tx, p *domain.Person) error {
	query := `INSERT INTO people (id, name, note, referenceLink, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		p.ID, p.Name, p.Note, p.ReferenceLink,
		timeString(p.CreatedAt), timeString(p.UpdatedAt), timePtrString(p.DeletedAt),
	)
	return err
}



func timeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func timePtrString(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

func jsonString(v any) *string {
	if v == nil {
		return nil
	}
	
	// Handle nil slices specifically
	switch val := v.(type) {
	case []string:
		if val == nil {
			return nil
		}
	case []domain.Attachment:
		if val == nil {
			return nil
		}
	}
	
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return nil
	}
	s := string(b)
	return &s
}
