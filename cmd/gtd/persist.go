package main

import (
	"context"
	"fmt"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/sqlite"
)

// Dual-store contract (see requirements/app_design.md §3):
//
//	Markdown files  = source of truth (durable, human-readable)
//	index.db        = rebuildable read cache (filters, joins, catalogs)
//
// Write path: validate/normalize → file Save → in-process SQLite Sync for that
// entity. Command handlers must use Persist* helpers; do not call bare
// repo.Save + syncEngine.Sync* pairs. External file edits require
// `gtd index rebuild`.
//
// Normalize policy A: apply sqlite.NormalizeTaskForLoad on the domain object
// *before* file Save so the file and SQL always match after a CLI write.

// PersistTask normalizes the task (policy A), writes the markdown file, then
// upserts the SQLite row. If Save fails, Sync is not attempted.
func (c *appContext) PersistTask(t *domain.Task, now time.Time) error {
	if t == nil {
		return fmt.Errorf("persist task: nil task")
	}
	sqlite.NormalizeTaskForLoad(t, now)
	if err := c.taskRepo.Save(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	if err := c.syncEngine.SyncTask(context.Background(), t, now); err != nil {
		return fmt.Errorf("sync task: %w", err)
	}
	return nil
}

// PersistProject writes the project file then upserts the SQLite row.
func (c *appContext) PersistProject(p *domain.Project) error {
	if p == nil {
		return fmt.Errorf("persist project: nil project")
	}
	if err := c.projectRepo.Save(p); err != nil {
		return fmt.Errorf("save project: %w", err)
	}
	if err := c.syncEngine.SyncProject(context.Background(), p); err != nil {
		return fmt.Errorf("sync project: %w", err)
	}
	return nil
}

// PersistArea writes the area file then upserts the SQLite row.
func (c *appContext) PersistArea(a *domain.Area) error {
	if a == nil {
		return fmt.Errorf("persist area: nil area")
	}
	if err := c.areaRepo.Save(a); err != nil {
		return fmt.Errorf("save area: %w", err)
	}
	if err := c.syncEngine.SyncArea(context.Background(), a); err != nil {
		return fmt.Errorf("sync area: %w", err)
	}
	return nil
}

// PersistPerson writes the person file then upserts the SQLite row.
func (c *appContext) PersistPerson(p *domain.Person) error {
	if p == nil {
		return fmt.Errorf("persist person: nil person")
	}
	if err := c.personRepo.Save(p); err != nil {
		return fmt.Errorf("save person: %w", err)
	}
	if err := c.syncEngine.SyncPerson(context.Background(), p); err != nil {
		return fmt.Errorf("sync person: %w", err)
	}
	return nil
}
