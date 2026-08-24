package app

import (
	"context"
	"fmt"
	"sort"

	"gtd/internal/domain"
)

// EntityCatalog returns the SQLite entity catalog (projects, areas, people, tags, contexts).
func (c *Context) EntityCatalog() (*domain.EntityCatalog, error) {
	catalog, err := c.taskQuery.GetEntityCatalog(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get catalog: %w", err)
	}
	return catalog, nil
}

// ListContexts returns sorted distinct contexts from the entity catalog.
func (c *Context) ListContexts() ([]string, error) {
	catalog, err := c.EntityCatalog()
	if err != nil {
		return nil, err
	}
	out := append([]string(nil), catalog.Contexts...)
	sort.Strings(out)
	return out, nil
}

// ListTags returns sorted distinct tags from the entity catalog.
func (c *Context) ListTags() ([]string, error) {
	catalog, err := c.EntityCatalog()
	if err != nil {
		return nil, err
	}
	out := append([]string(nil), catalog.Tags...)
	sort.Strings(out)
	return out, nil
}
