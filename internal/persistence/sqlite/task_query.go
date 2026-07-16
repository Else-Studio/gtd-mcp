package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gtd/internal/domain"
)

// TaskQuery provides query operations for tasks stored in SQLite.
type TaskQuery struct {
	db *sql.DB
}

func NewTaskQuery(db *sql.DB) *TaskQuery {
	return &TaskQuery{db: db}
}

// DefaultSortSQL returns the ORDER BY clause required for default task sorting.
func DefaultSortSQL() string {
	return `
ORDER BY
	CASE status
		WHEN 'inbox' THEN 1
		WHEN 'next' THEN 2
		WHEN 'waiting' THEN 3
		WHEN 'someday' THEN 4
		WHEN 'reference' THEN 5
		WHEN 'done' THEN 6
		WHEN 'archived' THEN 7
		ELSE 8
	END ASC,
	dueDate ASC,
	createdAt ASC
`
}

// TaskQueryFilter holds optional filters for tasks.
type TaskQueryFilter struct {
	AreaID     string
	ProjectID  string
	Context    string
	AssignedTo string
}

func (f *TaskQueryFilter) Apply(query string, args []interface{}) (string, []interface{}) {
	if f == nil {
		return query, args
	}
	if f.AreaID != "" {
		query += " AND areaId = ?"
		args = append(args, f.AreaID)
	}
	if f.ProjectID != "" {
		query += " AND projectId = ?"
		args = append(args, f.ProjectID)
	}
	if f.Context != "" {
		query += " AND EXISTS (SELECT 1 FROM json_each(contexts) WHERE value = ?)"
		args = append(args, f.Context)
	}
	if f.AssignedTo != "" {
		query += " AND assignedTo = ?"
		args = append(args, f.AssignedTo)
	}
	return query, args
}

// ListActiveTasks queries tasks with the default sort order.
func (q *TaskQuery) ListActiveTasks(ctx context.Context, filter *TaskQueryFilter) ([]string, error) {
	query := `
		SELECT id 
		FROM tasks 
		WHERE deletedAt IS NULL AND status NOT IN ('done', 'archived')
	`
	args := []interface{}{}
	query, args = filter.Apply(query, args)
	query += DefaultSortSQL()

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active tasks: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListTasksByStatus queries tasks matching a specific status.
func (q *TaskQuery) ListTasksByStatus(ctx context.Context, status string, filter *TaskQueryFilter) ([]string, error) {
	query := `
		SELECT id 
		FROM tasks 
		WHERE deletedAt IS NULL AND status = ?
	`
	args := []interface{}{status}
	query, args = filter.Apply(query, args)
	query += DefaultSortSQL()

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks by status: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountProjectNextTasks returns the number of non-deleted 'next' tasks for a given project.
func (q *TaskQuery) CountProjectNextTasks(ctx context.Context, projectID string) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM tasks 
		WHERE deletedAt IS NULL AND projectId = ? AND status = 'next'
	`
	var count int
	if err := q.db.QueryRowContext(ctx, query, projectID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count project next tasks: %w", err)
	}
	return count, nil
}

// ListInboxTasks is a shortcut for listing all inbox tasks.
func (q *TaskQuery) ListInboxTasks(ctx context.Context) ([]string, error) {
	return q.ListTasksByStatus(ctx, string(domain.TaskStatusInbox), nil)
}

// ListNextTasks is a shortcut for listing all next tasks.
func (q *TaskQuery) ListNextTasks(ctx context.Context) ([]string, error) {
	return q.ListTasksByStatus(ctx, string(domain.TaskStatusNext), nil)
}

// ListStalledProjects returns UUIDs of active projects with exactly 0 non-deleted 'next' tasks.
func (q *TaskQuery) ListStalledProjects(ctx context.Context) ([]string, error) {
	query := `
		SELECT p.id 
		FROM projects p
		LEFT JOIN tasks t ON p.id = t.projectId AND t.status = 'next' AND t.deletedAt IS NULL
		WHERE p.status = 'active' AND p.deletedAt IS NULL
		GROUP BY p.id
		HAVING COUNT(t.id) = 0
	`
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListAgendaTasks returns tasks where dueDate <= now or startTime <= now
func (q *TaskQuery) ListAgendaTasks(ctx context.Context, now time.Time, filter *TaskQueryFilter) ([]string, error) {
	nowStr := timeString(now)
	query := `
		SELECT id 
		FROM tasks 
		WHERE deletedAt IS NULL 
		  AND status NOT IN ('done', 'archived')
		  AND (
		      (dueDate IS NOT NULL AND dueDate <= ?)
		      OR
		      (startTime IS NOT NULL AND startTime <= ?)
		  )
	`
	args := []interface{}{nowStr, nowStr}
	query, args = filter.Apply(query, args)

	query += `
		ORDER BY 
		  CASE WHEN dueDate IS NOT NULL THEN dueDate ELSE '9999-99-99' END ASC,
		  CASE WHEN startTime IS NOT NULL THEN startTime ELSE '9999-99-99' END ASC
	`
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetProjectCandidates fetches non-deleted tasks for a project that are not done or archived.
func (q *TaskQuery) GetProjectCandidates(ctx context.Context, projectID string) ([]domain.Task, error) {
	query := `
		SELECT id, title, createdAt 
		FROM tasks 
		WHERE deletedAt IS NULL AND projectId = ? AND status NOT IN ('done', 'archived')
		ORDER BY createdAt ASC
	`
	rows, err := q.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("query project candidates: %w", err)
	}
	defer rows.Close()

	candidates := []domain.Task{}
	for rows.Next() {
		var id, title, createdAtStr string
		if err := rows.Scan(&id, &title, &createdAtStr); err != nil {
			return nil, err
		}
		
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtStr)
		candidates = append(candidates, domain.Task{
			ID:        id,
			Title:     title,
			CreatedAt: createdAt,
		})
	}
	return candidates, rows.Err()
}

// IsProjectActive checks if a project is in the 'active' status.
func (q *TaskQuery) IsProjectActive(ctx context.Context, projectID string) (bool, error) {
	query := `SELECT status FROM projects WHERE id = ? AND deletedAt IS NULL`
	var status string
	err := q.db.QueryRowContext(ctx, query, projectID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return status == "active", nil
}

// GetEntityCatalog loads active projects, areas, people, and distinct tags/contexts.
func (q *TaskQuery) GetEntityCatalog(ctx context.Context) (*domain.EntityCatalog, error) {
	catalog := &domain.EntityCatalog{}

	projRows, err := q.db.QueryContext(ctx, "SELECT id, title FROM projects WHERE deletedAt IS NULL AND status != 'archived'")
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer projRows.Close()
	for projRows.Next() {
		var p domain.Project
		if err := projRows.Scan(&p.ID, &p.Title); err != nil {
			return nil, err
		}
		catalog.Projects = append(catalog.Projects, p)
	}

	areaRows, err := q.db.QueryContext(ctx, "SELECT id, name FROM areas WHERE deletedAt IS NULL")
	if err != nil {
		return nil, fmt.Errorf("query areas: %w", err)
	}
	defer areaRows.Close()
	for areaRows.Next() {
		var a domain.Area
		if err := areaRows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		catalog.Areas = append(catalog.Areas, a)
	}

	personRows, err := q.db.QueryContext(ctx, "SELECT id, name FROM people WHERE deletedAt IS NULL")
	if err != nil {
		return nil, fmt.Errorf("query people: %w", err)
	}
	defer personRows.Close()
	for personRows.Next() {
		var p domain.Person
		if err := personRows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		catalog.People = append(catalog.People, p)
	}

	tagRows, err := q.db.QueryContext(ctx, "SELECT DISTINCT json_each.value FROM tasks, json_each(tasks.tags) WHERE tasks.deletedAt IS NULL AND tasks.tags IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var t string
		if err := tagRows.Scan(&t); err != nil {
			return nil, err
		}
		catalog.Tags = append(catalog.Tags, t)
	}

	ctxRows, err := q.db.QueryContext(ctx, "SELECT DISTINCT json_each.value FROM tasks, json_each(tasks.contexts) WHERE tasks.deletedAt IS NULL AND tasks.contexts IS NOT NULL")
	if err != nil {
		return nil, fmt.Errorf("query contexts: %w", err)
	}
	defer ctxRows.Close()
	for ctxRows.Next() {
		var c string
		if err := ctxRows.Scan(&c); err != nil {
			return nil, err
		}
		catalog.Contexts = append(catalog.Contexts, c)
	}

	return catalog, nil
}
