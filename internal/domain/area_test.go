package domain

import (
	"testing"
	"time"
)

func TestAreaValidation(t *testing.T) {
	tests := []struct {
		name    string
		area    *Area
		wantErr bool
	}{
		{
			name: "valid area",
			area: &Area{
				ID:   "a1",
				Name: "Personal",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			area: &Area{
				Name: "Personal",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			area: &Area{
				ID: "a1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.area.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAreaCascadeDeleteAndRestore(t *testing.T) {
	// Setup: Create an Area with 2 Projects, each containing 2 Tasks.
	area := &Area{ID: "area1"}
	
	proj1 := &Project{ID: "proj1", AreaID: &area.ID}
	proj2 := &Project{ID: "proj2", AreaID: &area.ID}
	
	task1 := &Task{ID: "task1", ProjectID: &proj1.ID}
	task2 := &Task{ID: "task2", ProjectID: &proj1.ID}
	task3 := &Task{ID: "task3", ProjectID: &proj2.ID}
	task4 := &Task{ID: "task4", ProjectID: &proj2.ID}
	task5 := &Task{ID: "task5", AreaID: &area.ID} // Directly in area

	projects := []*Project{proj1, proj2}
	tasks := []*Task{task1, task2, task3, task4, task5}

	now := time.Now().Truncate(time.Second)

	// Action: Call area.SoftDelete(now, projects, tasks)
	area.SoftDelete(now, projects, tasks)

	// Outcome: Assert Area, Projects, and Tasks are soft-deleted.
	if area.DeletedAt == nil || !area.DeletedAt.Equal(now) {
		t.Errorf("expected area DeletedAt to be %v, got %v", now, area.DeletedAt)
	}
	for _, p := range projects {
		if p.DeletedAt == nil || !p.DeletedAt.Equal(now) {
			t.Errorf("expected project %s DeletedAt to be %v, got %v", p.ID, now, p.DeletedAt)
		}
	}
	for _, tk := range tasks {
		if tk.DeletedAt == nil || !tk.DeletedAt.Equal(now) {
			t.Errorf("expected task %s DeletedAt to be %v, got %v", tk.ID, now, tk.DeletedAt)
		}
	}

	// Action 2: Call area.Restore(now, projects, tasks)
	restoreTime := now.Add(time.Second)
	area.Restore(restoreTime, projects, tasks)

	// Outcome: Assert Area, Projects, and Tasks are restored.
	if area.DeletedAt != nil {
		t.Errorf("expected area DeletedAt to be nil after restore")
	}
	for _, p := range projects {
		if p.DeletedAt != nil {
			t.Errorf("expected project %s DeletedAt to be nil after restore", p.ID)
		}
	}
	for _, tk := range tasks {
		if tk.DeletedAt != nil {
			t.Errorf("expected task %s DeletedAt to be nil after restore", tk.ID)
		}
	}
}
