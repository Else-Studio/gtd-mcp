package domain

import (
	"time"
)

// DuplicateRecurringTask produces the next instance of a recurring task.
func (t *Task) DuplicateRecurringTask(newID string, completedAt time.Time, previousStatus TaskStatus) *Task {
	if t.Recurrence == nil {
		return nil
	}

	rule := t.Recurrence.Rule
	if rule == "" {
		return nil
	}

	startAnchor := resolveAnchorDay(t.Recurrence.StartAnchorDay, t.Recurrence.AnchorDay, t.StartTime)
	dueAnchor := resolveAnchorDay(t.Recurrence.DueAnchorDay, t.Recurrence.AnchorDay, t.DueDate)
	reviewAnchor := resolveAnchorDay(t.Recurrence.ReviewAnchorDay, t.Recurrence.AnchorDay, t.ReviewAt)

	strategy := t.Recurrence.Strategy
	if strategy == "" {
		strategy = "strict"
	}

	var nextDue *time.Time
	if t.DueDate != nil {
		base := *t.DueDate
		if strategy == "fluid" {
			base = completedAt
		}
		anchor := dueAnchor
		if strategy == "fluid" {
			anchor = 0
		}
		nd := nextDateFrom(base, rule, anchor)
		if strategy == "strict" && !nd.After(completedAt) {
			nd = nextDateFrom(completedAt, rule, dueAnchor)
		}
		nextDue = &nd
	}

	var nextStart *time.Time
	if t.StartTime != nil {
		base := *t.StartTime
		if strategy == "fluid" {
			base = completedAt
		}
		anchor := startAnchor
		if strategy == "fluid" {
			anchor = 0
		}
		nd := nextDateFrom(base, rule, anchor)
		if strategy == "strict" && !nd.After(completedAt) {
			nd = nextDateFrom(completedAt, rule, startAnchor)
		}
		nextStart = &nd
	}

	var nextReview *time.Time
	if t.ReviewAt != nil {
		base := *t.ReviewAt
		if strategy == "fluid" {
			base = completedAt
		}
		anchor := reviewAnchor
		if strategy == "fluid" {
			anchor = 0
		}
		nd := nextDateFrom(base, rule, anchor)
		if strategy == "strict" && !nd.After(completedAt) {
			nd = nextDateFrom(completedAt, rule, reviewAnchor)
		}
		nextReview = &nd
	}

	var nextOffset *RelativeOffset
	if t.RelativeStartOffset != nil && nextDue != nil {
		computed := computeRelativeStartTime(*nextDue, t.RelativeStartOffset)
		if computed != nil {
			nextStart = computed
			nextOffset = &RelativeOffset{
				Amount: t.RelativeStartOffset.Amount,
				Unit:   t.RelativeStartOffset.Unit,
			}
		}
	}

	nextRecurrence := &RecurrenceRule{
		Rule:                 rule,
		Strategy:             t.Recurrence.Strategy,
		CompletedOccurrences: t.Recurrence.CompletedOccurrences + 1,
		AnchorDay:            t.Recurrence.AnchorDay,
		StartAnchorDay:       startAnchor,
		DueAnchorDay:         dueAnchor,
		ReviewAnchorDay:      reviewAnchor,
	}

	newStatus := previousStatus
	if newStatus == TaskStatusDone || newStatus == TaskStatusArchived {
		newStatus = TaskStatusNext
	}

	// Copy tags
	var nextTags []string
	if t.Tags != nil {
		nextTags = make([]string, len(t.Tags))
		copy(nextTags, t.Tags)
	}

	// Copy contexts
	var nextContexts []string
	if t.Contexts != nil {
		nextContexts = make([]string, len(t.Contexts))
		copy(nextContexts, t.Contexts)
	}

	return &Task{
		ID:                  newID,
		Title:               t.Title,
		Status:              newStatus,
		StartTime:           nextStart,
		DueDate:             nextDue,
		ReviewAt:            nextReview,
		RelativeStartOffset: nextOffset,
		Recurrence:          nextRecurrence,
		Tags:                nextTags,
		Contexts:            nextContexts,
		ProjectID:           t.ProjectID,
		SectionID:           t.SectionID,
		AreaID:              t.AreaID,
		TextDirection:       t.TextDirection,
		CreatedAt:           completedAt,
		UpdatedAt:           completedAt,
	}
}

func resolveAnchorDay(explicit, fallback int, date *time.Time) int {
	if explicit > 0 {
		return explicit
	}
	if fallback > 0 {
		return fallback
	}
	if date != nil {
		return date.Day()
	}
	return 0
}

func nextDateFrom(base time.Time, rule string, anchorDay int) time.Time {
	switch rule {
	case "daily":
		return base.AddDate(0, 0, 1)
	case "weekly":
		return base.AddDate(0, 0, 7)
	case "monthly":
		return addMonthsClamped(base, 1, anchorDay)
	case "yearly":
		return addYearsClamped(base, 1, anchorDay)
	}
	return base
}

func getLastDayOfMonth(year int, month time.Month) int {
	// The day before the first day of the *next* month is the last day of the current month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func addMonthsClamped(base time.Time, months int, anchorDay int) time.Time {
	year := base.Year()
	month := base.Month() + time.Month(months)

	// Normalize year and month
	for month > 12 {
		month -= 12
		year++
	}
	for month < 1 {
		month += 12
		year--
	}

	lastDay := getLastDayOfMonth(year, month)
	day := base.Day()
	if anchorDay > 0 {
		day = anchorDay
	}
	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, month, day, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
}

func addYearsClamped(base time.Time, years int, anchorDay int) time.Time {
	year := base.Year() + years
	month := base.Month()

	lastDay := getLastDayOfMonth(year, month)
	day := base.Day()
	if anchorDay > 0 {
		day = anchorDay
	}
	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, month, day, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location())
}
