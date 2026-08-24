package app

import (
	"context"
	"fmt"
	"time"
)

// ListInboxIDs returns IDs of inbox tasks.
func (c *Context) ListInboxIDs(ctx context.Context) ([]string, error) {
	ids, err := c.taskQuery.ListInboxTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	return ids, nil
}

// ListNextIDs returns IDs of next-action tasks, optionally filtered.
func (c *Context) ListNextIDs(ctx context.Context, f TaskListFilter) ([]string, error) {
	ids, err := c.taskQuery.ListNextTasks(ctx, c.resolveTaskListFilter(f))
	if err != nil {
		return nil, fmt.Errorf("list next: %w", err)
	}
	return ids, nil
}

// ListAgendaIDs returns IDs of agenda tasks ("what's important now").
func (c *Context) ListAgendaIDs(ctx context.Context, now time.Time, f TaskListFilter) ([]string, error) {
	ids, err := c.taskQuery.ListAgendaTasks(ctx, now, c.resolveTaskListFilter(f))
	if err != nil {
		return nil, fmt.Errorf("list agenda: %w", err)
	}
	return ids, nil
}

// ListStalledProjectIDs returns IDs of active projects with zero next actions.
func (c *Context) ListStalledProjectIDs(ctx context.Context) ([]string, error) {
	ids, err := c.taskQuery.ListStalledProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stalled: %w", err)
	}
	return ids, nil
}
