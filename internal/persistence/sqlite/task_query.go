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

// DefaultSortSQL returns the ORDER BY clause required for default task sorting
// (technical API §6.B): status rank, then dated tasks before undated (nulls last),
// then dueDate ASC, then createdAt ASC.
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
	CASE WHEN dueDate IS NULL THEN 1 ELSE 0 END ASC,
	dueDate ASC,
	createdAt ASC
`
}

// defaultSortSQLAliased is DefaultSortSQL with a table alias (e.g. "t") for joins.
func defaultSortSQLAliased(alias string) string {
	return `
ORDER BY
	CASE ` + alias + `.status
		WHEN 'inbox' THEN 1
		WHEN 'next' THEN 2
		WHEN 'waiting' THEN 3
		WHEN 'someday' THEN 4
		WHEN 'reference' THEN 5
		WHEN 'done' THEN 6
		WHEN 'archived' THEN 7
		ELSE 8
	END ASC,
	CASE WHEN ` + alias + `.dueDate IS NULL THEN 1 ELSE 0 END ASC,
	` + alias + `.dueDate ASC,
	` + alias + `.createdAt ASC
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
		// Direct area tasks, or tasks whose project belongs to the area
		// (container exclusivity: project tasks clear areaId but inherit via project).
		query += ` AND (
			areaId = ?
			OR projectId IN (
				SELECT id FROM projects
				WHERE areaId = ? AND deletedAt IS NULL
			)
		)`
		args = append(args, f.AreaID, f.AreaID)
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
	// Next-action lists exclude tasks whose parent project is someday/archived/deleted.
	if status == string(domain.TaskStatusNext) {
		return q.listNextTasksFiltered(ctx, filter)
	}

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

// listNextTasksFiltered returns non-deleted next tasks, excluding those under
// someday/archived/deleted projects (loose tasks with no project remain eligible).
func (q *TaskQuery) listNextTasksFiltered(ctx context.Context, filter *TaskQueryFilter) ([]string, error) {
	query := `
		SELECT t.id
		FROM tasks t
		LEFT JOIN projects p ON t.projectId = p.id
		WHERE t.deletedAt IS NULL
		  AND t.status = 'next'
		  AND (
		      t.projectId IS NULL
		      OR (
		          p.id IS NOT NULL
		          AND p.deletedAt IS NULL
		          AND p.status NOT IN ('someday', 'archived')
		      )
		  )
	`
	args := []interface{}{}
	// Re-apply optional filters against the tasks alias.
	if filter != nil {
		if filter.AreaID != "" {
			query += ` AND (
				t.areaId = ?
				OR t.projectId IN (
					SELECT id FROM projects
					WHERE areaId = ? AND deletedAt IS NULL
				)
			)`
			args = append(args, filter.AreaID, filter.AreaID)
		}
		if filter.ProjectID != "" {
			query += " AND t.projectId = ?"
			args = append(args, filter.ProjectID)
		}
		if filter.Context != "" {
			query += " AND EXISTS (SELECT 1 FROM json_each(t.contexts) WHERE value = ?)"
			args = append(args, filter.Context)
		}
		if filter.AssignedTo != "" {
			query += " AND t.assignedTo = ?"
			args = append(args, filter.AssignedTo)
		}
	}
	query += defaultSortSQLAliased("t")

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query next tasks: %w", err)
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

// ListNextTasks is a shortcut for listing all next tasks (optional filters).
func (q *TaskQuery) ListNextTasks(ctx context.Context, filter *TaskQueryFilter) ([]string, error) {
	return q.ListTasksByStatus(ctx, string(domain.TaskStatusNext), filter)
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

// ListAgendaTasks returns "What's Important Now" IDs (aligned with domain.GetAgenda):
//   - exclude done / archived / reference / soft-deleted
//   - date-only due (time 00:00:00): calendar day due <= calendar day now
//   - timed due: dueDate <= now
//   - startTime <= now
func (q *TaskQuery) ListAgendaTasks(ctx context.Context, now time.Time, filter *TaskQueryFilter) ([]string, error) {
	nowStr := timeString(now)
	// date(now) for calendar-day comparison of date-only dues.
	nowDate := now.Format("2006-01-02")
	query := `
		SELECT id 
		FROM tasks 
		WHERE deletedAt IS NULL 
		  AND status NOT IN ('done', 'archived', 'reference')
		  AND (
		      (startTime IS NOT NULL AND startTime <= ?)
		      OR
		      (dueDate IS NOT NULL AND (
		          (
		              strftime('%H:%M:%S', dueDate) = '00:00:00'
		              AND date(dueDate) <= date(?)
		          )
		          OR (
		              strftime('%H:%M:%S', dueDate) != '00:00:00'
		              AND dueDate <= ?
		          )
		      ))
		  )
	`
	args := []interface{}{nowStr, nowDate, nowStr}
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

// GetProjectCandidates fetches open tasks that could be promoted to next:
// non-deleted, not done/archived/reference, and not already next.
// Sort: orderNum ASC → createdAt ASC → title ASC (technical API §7.B).
func (q *TaskQuery) GetProjectCandidates(ctx context.Context, projectID string) ([]domain.Task, error) {
	query := `
		SELECT id, title, orderNum, createdAt 
		FROM tasks 
		WHERE deletedAt IS NULL
		  AND projectId = ?
		  AND status NOT IN ('done', 'archived', 'reference', 'next')
		ORDER BY orderNum ASC, createdAt ASC, title ASC
	`
	rows, err := q.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("query project candidates: %w", err)
	}
	defer rows.Close()

	candidates := []domain.Task{}
	for rows.Next() {
		var id, title, createdAtStr string
		var orderNum int
		if err := rows.Scan(&id, &title, &orderNum, &createdAtStr); err != nil {
			return nil, err
		}

		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtStr)
		candidates = append(candidates, domain.Task{
			ID:        id,
			Title:     title,
			OrderNum:  orderNum,
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
