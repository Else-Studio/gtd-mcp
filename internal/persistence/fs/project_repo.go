package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type ProjectRepository struct {
	generic *GenericRepo[*domain.Project]
}

func NewProjectRepository(rootDir string) *ProjectRepository {
	return &ProjectRepository{
		generic: NewGenericRepo[*domain.Project](rootDir, "projects", &projectCodec{}),
	}
}

type projectFrontmatter struct {
	Status      domain.ProjectStatus `yaml:"status"`
	Color       string               `yaml:"color,omitempty"`
	OrderNum    int                  `yaml:"orderNum,omitempty"`
	TagIDs      []string             `yaml:"tagIds,omitempty"`
	Attachments []domain.Attachment  `yaml:"attachments,omitempty"`
	DueDate     *time.Time           `yaml:"dueDate,omitempty"`
	ReviewAt    *time.Time           `yaml:"reviewAt,omitempty"`
	AreaID      *string              `yaml:"areaId,omitempty"`
	AreaTitle   string               `yaml:"areaTitle,omitempty"`
	CreatedAt   *time.Time           `yaml:"createdAt,omitempty"`
	UpdatedAt   *time.Time           `yaml:"updatedAt,omitempty"`
	DeletedAt   *time.Time           `yaml:"deletedAt,omitempty"`
}

type projectCodec struct{}

func (c *projectCodec) Encode(project *domain.Project, now time.Time) ([]byte, string, string, error) {
	if err := project.Validate(); err != nil {
		return nil, "", "", err
	}

	if project.CreatedAt.IsZero() {
		project.CreatedAt = now
	}
	project.UpdatedAt = now

	fm := projectFrontmatter{
		Status:      project.Status,
		Color:       project.Color,
		OrderNum:    project.OrderNum,
		TagIDs:      project.TagIDs,
		Attachments: project.Attachments,
		DueDate:     project.DueDate,
		ReviewAt:    project.ReviewAt,
		AreaID:      project.AreaID,
		AreaTitle:   project.AreaTitle,
		CreatedAt:   &project.CreatedAt,
		UpdatedAt:   &project.UpdatedAt,
		DeletedAt:   project.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, project.Title, project.SupportNotes, nil
}

func (c *projectCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.Project, error) {
	var fm projectFrontmatter
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

	project := &domain.Project{
		ID:           id,
		Title:        title,
		SupportNotes: desc,
		Status:       fm.Status,
		Color:        fm.Color,
		OrderNum:     fm.OrderNum,
		TagIDs:       fm.TagIDs,
		Attachments:  fm.Attachments,
		DueDate:      fm.DueDate,
		ReviewAt:     fm.ReviewAt,
		AreaID:       fm.AreaID,
		AreaTitle:    fm.AreaTitle,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		DeletedAt:    fm.DeletedAt,
	}

	return project, nil
}

func (r *ProjectRepository) Save(project *domain.Project) error {
	return r.generic.Save(project, project.ID)
}

func (r *ProjectRepository) Get(id string) (*domain.Project, error) {
	return r.generic.Get(id)
}

func (r *ProjectRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *ProjectRepository) List() ([]*domain.Project, error) {
	return r.generic.List()
}
