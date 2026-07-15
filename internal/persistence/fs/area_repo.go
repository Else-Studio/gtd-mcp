package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type AreaRepository struct {
	generic *GenericRepo[*domain.Area]
}

func NewAreaRepository(rootDir string) *AreaRepository {
	return &AreaRepository{
		generic: NewGenericRepo[*domain.Area](rootDir, "areas", &areaCodec{}),
	}
}

type areaFrontmatter struct {
	Color     string     `yaml:"color,omitempty"`
	Icon      string     `yaml:"icon,omitempty"`
	OrderNum  int        `yaml:"orderNum"`
	CreatedAt *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt *time.Time `yaml:"updatedAt,omitempty"`
	DeletedAt *time.Time `yaml:"deletedAt,omitempty"`
}

type areaCodec struct{}

func (c *areaCodec) Encode(area *domain.Area, now time.Time) ([]byte, string, string, error) {
	if err := area.Validate(); err != nil {
		return nil, "", "", err
	}

	if area.CreatedAt.IsZero() {
		area.CreatedAt = now
	}
	area.UpdatedAt = now

	fm := areaFrontmatter{
		Color:     area.Color,
		Icon:      area.Icon,
		OrderNum:  area.OrderNum,
		CreatedAt: &area.CreatedAt,
		UpdatedAt: &area.UpdatedAt,
		DeletedAt: area.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, area.Name, "", nil
}

func (c *areaCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.Area, error) {
	var fm areaFrontmatter
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

	area := &domain.Area{
		ID:        id,
		Name:      title,
		Color:     fm.Color,
		Icon:      fm.Icon,
		OrderNum:  fm.OrderNum,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		DeletedAt: fm.DeletedAt,
	}

	return area, nil
}

func (r *AreaRepository) Save(area *domain.Area) error {
	return r.generic.Save(area, area.ID)
}

func (r *AreaRepository) Get(id string) (*domain.Area, error) {
	return r.generic.Get(id)
}

func (r *AreaRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *AreaRepository) List() ([]*domain.Area, error) {
	return r.generic.List()
}
