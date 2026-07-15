package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type SectionRepository struct {
	generic *GenericRepo[*domain.Section]
}

func NewSectionRepository(rootDir string) *SectionRepository {
	return &SectionRepository{
		generic: NewGenericRepo[*domain.Section](rootDir, "sections", &sectionCodec{}),
	}
}

type sectionFrontmatter struct {
	ProjectID   string     `yaml:"projectId"`
	OrderNum    int        `yaml:"orderNum"`
	IsCollapsed bool       `yaml:"isCollapsed"`
	CreatedAt   *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt   *time.Time `yaml:"updatedAt,omitempty"`
	DeletedAt   *time.Time `yaml:"deletedAt,omitempty"`
}

type sectionCodec struct{}

func (c *sectionCodec) Encode(section *domain.Section, now time.Time) ([]byte, string, string, error) {
	if err := section.Validate(); err != nil {
		return nil, "", "", err
	}

	if section.CreatedAt.IsZero() {
		section.CreatedAt = now
	}
	section.UpdatedAt = now

	fm := sectionFrontmatter{
		ProjectID:   section.ProjectID,
		OrderNum:    section.OrderNum,
		IsCollapsed: section.IsCollapsed,
		CreatedAt:   &section.CreatedAt,
		UpdatedAt:   &section.UpdatedAt,
		DeletedAt:   section.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, section.Title, section.Description, nil
}

func (c *sectionCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.Section, error) {
	var fm sectionFrontmatter
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

	section := &domain.Section{
		ID:          id,
		ProjectID:   fm.ProjectID,
		Title:       title,
		Description: desc,
		OrderNum:    fm.OrderNum,
		IsCollapsed: fm.IsCollapsed,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		DeletedAt:   fm.DeletedAt,
	}

	return section, nil
}

func (r *SectionRepository) Save(section *domain.Section) error {
	return r.generic.Save(section, section.ID)
}

func (r *SectionRepository) Get(id string) (*domain.Section, error) {
	return r.generic.Get(id)
}

func (r *SectionRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *SectionRepository) List() ([]*domain.Section, error) {
	return r.generic.List()
}
