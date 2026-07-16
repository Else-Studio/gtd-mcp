package domain

import (
	"sort"
	"time"
)

// Production list/agenda/candidate/stalled queries run through SQLite
// (sqlite.TaskQuery). Domain helpers below keep the same rules for pure unit
// tests and documentation; keep them in lockstep with the SQL path (Stage 15 R15).

// IsProjectStalled reports whether an active project has zero non-deleted next tasks.
// Production: sqlite.ListStalledProjects / CountProjectNextTasks.
func IsProjectStalled(project *Project, tasks []*Task) bool {
	if project.Status != ProjectStatusActive || project.DeletedAt != nil {
		return false
	}
	for _, t := range tasks {
		if t.ProjectID != nil && *t.ProjectID == project.ID && t.Status == TaskStatusNext && t.DeletedAt == nil {
			return false
		}
	}
	return true
}

// GetNextActionCandidates returns open tasks that could be promoted to next
// (technical API §7.B). Sorted orderNum → createdAt → title.
// Production: sqlite.GetProjectCandidates.
func GetNextActionCandidates(projectID string, tasks []*Task, excludeTaskID string) []*Task {
	var candidates []*Task
	for _, t := range tasks {
		if t.ID != excludeTaskID && t.ProjectID != nil && *t.ProjectID == projectID && t.DeletedAt == nil {
			if t.Status != TaskStatusNext && t.Status != TaskStatusDone && t.Status != TaskStatusArchived && t.Status != TaskStatusReference {
				candidates = append(candidates, t)
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.OrderNum != b.OrderNum {
			return a.OrderNum < b.OrderNum
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.Title < b.Title
	})
	return candidates
}

// GetAgenda returns "What's Important Now" tasks (Stage 15 product rules):
//   - date-only due: calendar-day comparison (date(due) <= date(now)) so due today is included all day
//   - timed due: dueDate <= now
//   - startTime <= now opens the window
//   - exclude done, archived, reference, soft-deleted
//
// Production: sqlite.ListAgendaTasks (same predicate via SQL).
func GetAgenda(tasks []*Task, now time.Time) []*Task {
	var agenda []*Task
	for _, t := range tasks {
		if t.DeletedAt != nil || t.Status == TaskStatusDone || t.Status == TaskStatusArchived || t.Status == TaskStatusReference {
			continue
		}
		if TaskOnAgenda(t, now) {
			agenda = append(agenda, t)
		}
	}
	return agenda
}

// TaskOnAgenda is the shared agenda membership predicate (domain + docs for SQL).
func TaskOnAgenda(t *Task, now time.Time) bool {
	if t.StartTime != nil && !t.StartTime.After(now) {
		return true
	}
	if t.DueDate == nil {
		return false
	}
	if isDateOnly(t.DueDate) {
		dueDay := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, t.DueDate.Location())
		nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return !dueDay.After(nowDay)
	}
	return !t.DueDate.After(now)
}

func ValidateTaskCoherence(t *Task) []string {
	var warnings []string
	
	dateOnly := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}

	if t.StartTime != nil && t.DueDate != nil {
		if dateOnly(*t.StartTime).After(dateOnly(*t.DueDate)) {
			warnings = append(warnings, "start_after_due")
		}
	}
	if t.DueDate != nil && t.DueDate.Before(t.CreatedAt) {
		warnings = append(warnings, "due_before_created")
	}
	if t.StartTime != nil && t.StartTime.Before(t.CreatedAt) {
		warnings = append(warnings, "start_before_created")
	}
	
	return warnings
}
