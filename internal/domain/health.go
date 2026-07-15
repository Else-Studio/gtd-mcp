package domain

import "time"

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

func GetNextActionCandidates(projectID string, tasks []*Task, excludeTaskID string) []*Task {
	var candidates []*Task
	for _, t := range tasks {
		if t.ID != excludeTaskID && t.ProjectID != nil && *t.ProjectID == projectID && t.DeletedAt == nil {
			if t.Status != TaskStatusNext && t.Status != TaskStatusDone && t.Status != TaskStatusArchived && t.Status != TaskStatusReference {
				candidates = append(candidates, t)
			}
		}
	}
	return candidates
}

func GetAgenda(tasks []*Task, now time.Time) []*Task {
	var agenda []*Task
	for _, t := range tasks {
		if t.DeletedAt != nil || t.Status == TaskStatusDone || t.Status == TaskStatusArchived || t.Status == TaskStatusReference {
			continue
		}
		include := false
		if t.StartTime != nil && !t.StartTime.After(now) {
			include = true
		}
		if t.DueDate != nil {
			effectiveDue := *t.DueDate
			if isDateOnly(t.DueDate) {
				effectiveDue = effectiveDue.Add(23*time.Hour + 59*time.Minute + 59*time.Second + 999*time.Millisecond)
			}
			if !effectiveDue.After(now) {
				include = true
			}
		}
		if include {
			agenda = append(agenda, t)
		}
	}
	return agenda
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
