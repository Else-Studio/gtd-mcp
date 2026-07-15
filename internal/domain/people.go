package domain

import (
	"fmt"
	"time"
)

type Person struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Note          string     `json:"note,omitempty"`
	ReferenceLink string     `json:"referenceLink,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

func (p *Person) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("%w: person ID cannot be empty", ErrValidation)
	}
	if p.Name == "" {
		return fmt.Errorf("%w: person name cannot be empty", ErrValidation)
	}
	return nil
}

func (p *Person) SoftDelete(now time.Time) {
	p.DeletedAt = &now
	p.UpdatedAt = now
}

func (p *Person) Restore(now time.Time) {
	p.DeletedAt = nil
	p.UpdatedAt = now
}
