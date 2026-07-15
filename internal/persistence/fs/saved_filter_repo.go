package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type SavedFilterRepository struct {
	generic *GenericRepo[*domain.SavedFilter]
}

func NewSavedFilterRepository(rootDir string) *SavedFilterRepository {
	return &SavedFilterRepository{
		generic: NewGenericRepo[*domain.SavedFilter](rootDir, "saved_filters", &savedFilterCodec{}),
	}
}

type savedFilterFrontmatter struct {
	Icon      string     `yaml:"icon,omitempty"`
	View      string     `yaml:"view"`
	Criteria  string     `yaml:"criteria"`
	SortBy    string     `yaml:"sortBy,omitempty"`
	SortOrder string     `yaml:"sortOrder,omitempty"`
	GroupBy   string     `yaml:"groupBy,omitempty"`
	CreatedAt *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt *time.Time `yaml:"updatedAt,omitempty"`
	DeletedAt *time.Time `yaml:"deletedAt,omitempty"`
}

type savedFilterCodec struct{}

func (c *savedFilterCodec) Encode(filter *domain.SavedFilter, now time.Time) ([]byte, string, string, error) {
	if err := filter.Validate(); err != nil {
		return nil, "", "", err
	}

	if filter.CreatedAt.IsZero() {
		filter.CreatedAt = now
	}
	filter.UpdatedAt = now

	fm := savedFilterFrontmatter{
		Icon:      filter.Icon,
		View:      filter.View,
		Criteria:  filter.Criteria,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
		GroupBy:   filter.GroupBy,
		CreatedAt: &filter.CreatedAt,
		UpdatedAt: &filter.UpdatedAt,
		DeletedAt: filter.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, filter.Name, "", nil
}

func (c *savedFilterCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.SavedFilter, error) {
	var fm savedFilterFrontmatter
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

	filter := &domain.SavedFilter{
		ID:        id,
		Name:      title,
		Icon:      fm.Icon,
		View:      fm.View,
		Criteria:  fm.Criteria,
		SortBy:    fm.SortBy,
		SortOrder: fm.SortOrder,
		GroupBy:   fm.GroupBy,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		DeletedAt: fm.DeletedAt,
	}

	return filter, nil
}

func (r *SavedFilterRepository) Save(filter *domain.SavedFilter) error {
	return r.generic.Save(filter, filter.ID)
}

func (r *SavedFilterRepository) Get(id string) (*domain.SavedFilter, error) {
	return r.generic.Get(id)
}

func (r *SavedFilterRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *SavedFilterRepository) List() ([]*domain.SavedFilter, error) {
	return r.generic.List()
}
