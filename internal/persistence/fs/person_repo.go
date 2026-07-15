package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type PersonRepository struct {
	generic *GenericRepo[*domain.Person]
}

func NewPersonRepository(rootDir string) *PersonRepository {
	return &PersonRepository{
		generic: NewGenericRepo[*domain.Person](rootDir, "people", &personCodec{}),
	}
}

type personFrontmatter struct {
	ReferenceLink string     `yaml:"referenceLink,omitempty"`
	CreatedAt     *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt     *time.Time `yaml:"updatedAt,omitempty"`
	DeletedAt     *time.Time `yaml:"deletedAt,omitempty"`
}

type personCodec struct{}

func (c *personCodec) Encode(person *domain.Person, now time.Time) ([]byte, string, string, error) {
	if err := person.Validate(); err != nil {
		return nil, "", "", err
	}

	if person.CreatedAt.IsZero() {
		person.CreatedAt = now
	}
	person.UpdatedAt = now

	fm := personFrontmatter{
		ReferenceLink: person.ReferenceLink,
		CreatedAt:     &person.CreatedAt,
		UpdatedAt:     &person.UpdatedAt,
		DeletedAt:     person.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, person.Name, person.Note, nil
}

func (c *personCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.Person, error) {
	var fm personFrontmatter
	if len(frontmatter) > 0 {
		if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
			return nil, err
		}
	}

	createdAt := now
	if fm.CreatedAt != nil && !fm.CreatedAt.IsZero() {
		createdAt = *fm.CreatedAt
	}
	updatedAt := now
	if fm.UpdatedAt != nil && !fm.UpdatedAt.IsZero() {
		updatedAt = *fm.UpdatedAt
	}
	if updatedAt.Before(createdAt) {
		updatedAt = createdAt
	}

	person := &domain.Person{
		ID:            id,
		Name:          title,
		Note:          desc,
		ReferenceLink: fm.ReferenceLink,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		DeletedAt:     fm.DeletedAt,
	}

	return person, nil
}

func (r *PersonRepository) Save(person *domain.Person) error {
	return r.generic.Save(person, person.ID)
}

func (r *PersonRepository) Get(id string) (*domain.Person, error) {
	return r.generic.Get(id)
}

func (r *PersonRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *PersonRepository) List() ([]*domain.Person, error) {
	return r.generic.List()
}
