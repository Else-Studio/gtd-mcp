package domain

import (
	"testing"
	"time"
)

func TestCascadeDelete(t *testing.T) {
	project := &Project{ID: "project1"}
	task1 := &Task{ID: "task1", ProjectID: &project.ID}
	task2 := &Task{ID: "task2", ProjectID: &project.ID}
	task3 := &Task{ID: "task3", ProjectID: &project.ID}
	section := &Section{ID: "sec1", ProjectID: "project1"}

	tasks := []*Task{task1, task2, task3}
	sections := []*Section{section}

	now := time.Now().Truncate(time.Second)

	project.SoftDelete(now, tasks, sections)

	if project.DeletedAt == nil || !project.DeletedAt.Equal(now) {
		t.Errorf("expected project DeletedAt to be %v, got %v", now, project.DeletedAt)
	}
	for _, tk := range tasks {
		if tk.DeletedAt == nil || !tk.DeletedAt.Equal(now) {
			t.Errorf("expected task DeletedAt to be %v for %s, got %v", now, tk.ID, tk.DeletedAt)
		}
	}
	if section.DeletedAt == nil || !section.DeletedAt.Equal(now) {
		t.Errorf("expected section DeletedAt to be %v, got %v", now, section.DeletedAt)
	}

	// Test Restore
	project.Restore(now, tasks, sections)
	if project.DeletedAt != nil {
		t.Errorf("expected project DeletedAt to be nil")
	}
	for _, tk := range tasks {
		if tk.DeletedAt != nil {
			t.Errorf("expected task DeletedAt to be nil for %s", tk.ID)
		}
	}
	if section.DeletedAt != nil {
		t.Errorf("expected section DeletedAt to be nil")
	}
}

func TestProjectValidation(t *testing.T) {
	tests := []struct {
		name    string
		project *Project
		wantErr bool
	}{
		{
			name: "valid project",
			project: &Project{
				ID:     "p1",
				Title:  "Title",
				Status: ProjectStatusActive,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			project: &Project{
				Title:  "Title",
				Status: ProjectStatusActive,
			},
			wantErr: true,
		},
		{
			name: "missing title",
			project: &Project{
				ID:     "p1",
				Status: ProjectStatusActive,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			project: &Project{
				ID:     "p1",
				Title:  "Title",
				Status: "banana",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
