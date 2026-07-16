package fs

import (
	"gtd/internal/domain"
	"time"

	"gopkg.in/yaml.v3"
)

type TaskRepository struct {
	generic *GenericRepo[*domain.Task]
}

func NewTaskRepository(rootDir string) *TaskRepository {
	return &TaskRepository{
		generic: NewGenericRepo[*domain.Task](rootDir, "tasks", &taskCodec{}),
	}
}

type taskFrontmatter struct {
	Status              domain.TaskStatus      `yaml:"status"`
	Priority            string                 `yaml:"priority,omitempty"`
	EnergyLevel         string                 `yaml:"energyLevel,omitempty"`
	AssignedTo          string                 `yaml:"assignedTo,omitempty"`
	StartTime           *time.Time             `yaml:"startTime,omitempty"`
	RelativeStartOffset *domain.RelativeOffset `yaml:"relativeStartOffset,omitempty"`
	DueDate             *time.Time             `yaml:"dueDate,omitempty"`
	Recurrence          *domain.RecurrenceRule `yaml:"recurrence,omitempty"`
	Tags                []string               `yaml:"tags,omitempty"`
	Contexts            []string               `yaml:"contexts,omitempty"`
	TextDirection       string                 `yaml:"textDirection,omitempty"`
	Attachments         []domain.Attachment    `yaml:"attachments,omitempty"`
	Location            string                 `yaml:"location,omitempty"`
	ProjectID           *string                `yaml:"projectId,omitempty"`
	AreaID              *string                `yaml:"areaId,omitempty"`
	OrderNum            int                    `yaml:"orderNum,omitempty"`
	TimeEstimate        string                 `yaml:"timeEstimate,omitempty"`
	TimeSpentMinutes    int                    `yaml:"timeSpentMinutes,omitempty"`
	ReviewAt            *time.Time             `yaml:"reviewAt,omitempty"`
	CompletedAt         *time.Time             `yaml:"completedAt,omitempty"`
	CreatedAt           *time.Time             `yaml:"createdAt,omitempty"`
	UpdatedAt           *time.Time             `yaml:"updatedAt,omitempty"`
	DeletedAt           *time.Time             `yaml:"deletedAt,omitempty"`
}

type taskCodec struct{}

func (c *taskCodec) Encode(task *domain.Task, now time.Time) ([]byte, string, string, error) {
	if err := task.Validate(); err != nil {
		return nil, "", "", err
	}

	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	fm := taskFrontmatter{
		Status:              task.Status,
		Priority:            task.Priority,
		EnergyLevel:         task.EnergyLevel,
		AssignedTo:          task.AssignedTo,
		StartTime:           task.StartTime,
		RelativeStartOffset: task.RelativeStartOffset,
		DueDate:             task.DueDate,
		Recurrence:          task.Recurrence,
		Tags:                task.Tags,
		Contexts:            task.Contexts,
		TextDirection:       task.TextDirection,
		Attachments:         task.Attachments,
		Location:            task.Location,
		ProjectID:           task.ProjectID,
		AreaID:              task.AreaID,
		OrderNum:            task.OrderNum,
		TimeEstimate:        task.TimeEstimate,
		TimeSpentMinutes:    task.TimeSpentMinutes,
		ReviewAt:            task.ReviewAt,
		CompletedAt:         task.CompletedAt,
		CreatedAt:           &task.CreatedAt,
		UpdatedAt:           &task.UpdatedAt,
		DeletedAt:           task.DeletedAt,
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", "", err
	}

	return yamlBytes, task.Title, task.Description, nil
}

func (c *taskCodec) Decode(id, title, desc string, frontmatter []byte, now time.Time) (*domain.Task, error) {
	var fm taskFrontmatter
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

	task := &domain.Task{
		ID:                  id,
		Title:               title,
		Description:         desc,
		Status:              fm.Status,
		Priority:            fm.Priority,
		EnergyLevel:         fm.EnergyLevel,
		AssignedTo:          fm.AssignedTo,
		StartTime:           fm.StartTime,
		RelativeStartOffset: fm.RelativeStartOffset,
		DueDate:             fm.DueDate,
		Recurrence:          fm.Recurrence,
		Tags:                fm.Tags,
		Contexts:            fm.Contexts,
		TextDirection:       fm.TextDirection,
		Attachments:         fm.Attachments,
		Location:            fm.Location,
		ProjectID:           fm.ProjectID,
		AreaID:              fm.AreaID,
		OrderNum:            fm.OrderNum,
		TimeEstimate:        fm.TimeEstimate,
		TimeSpentMinutes:    fm.TimeSpentMinutes,
		ReviewAt:            fm.ReviewAt,
		CompletedAt:         fm.CompletedAt,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
		DeletedAt:           fm.DeletedAt,
	}

	return task, nil
}

func (r *TaskRepository) Save(task *domain.Task) error {
	return r.generic.Save(task, task.ID)
}

func (r *TaskRepository) Get(id string) (*domain.Task, error) {
	return r.generic.Get(id)
}

func (r *TaskRepository) Delete(id string) error {
	return r.generic.Delete(id)
}

func (r *TaskRepository) List() ([]*domain.Task, error) {
	return r.generic.List()
}
