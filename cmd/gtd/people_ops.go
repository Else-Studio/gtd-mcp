package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gtd/internal/domain"
)

// CreatePerson creates a person with the given name.
func (c *appContext) CreatePerson(name string) (*domain.Person, error) {
	person := &domain.Person{
		ID:   uuid.New().String(),
		Name: name,
	}

	if err := c.PersistPerson(person); err != nil {
		return nil, fmt.Errorf("persist person: %w", err)
	}
	return person, nil
}

// UpdatePersonOptions holds fields that may be changed on a person.
type UpdatePersonOptions struct {
	Name string // empty = no change
}

// UpdatePerson loads a person, applies name change if set, and persists.
func (c *appContext) UpdatePerson(id string, opts UpdatePersonOptions) (*domain.Person, error) {
	person, err := c.personRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("person not found: %w", err)
	}

	if opts.Name != "" {
		person.Name = opts.Name
		person.UpdatedAt = time.Now()
	}

	if err := c.PersistPerson(person); err != nil {
		return nil, fmt.Errorf("persist person: %w", err)
	}
	return person, nil
}

// DeletePerson soft-deletes a person by ID.
func (c *appContext) DeletePerson(id string) (*domain.Person, error) {
	person, err := c.personRepo.Get(id)
	if err != nil {
		return nil, fmt.Errorf("person not found: %w", err)
	}

	now := time.Now()
	person.SoftDelete(now)

	if err := c.PersistPerson(person); err != nil {
		return nil, fmt.Errorf("persist person: %w", err)
	}
	return person, nil
}

// ListActivePeople returns non-deleted people.
func (c *appContext) ListActivePeople() ([]*domain.Person, error) {
	people, err := c.personRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}

	activePeople := make([]*domain.Person, 0)
	for _, p := range people {
		if p.DeletedAt == nil {
			activePeople = append(activePeople, p)
		}
	}
	return activePeople, nil
}
