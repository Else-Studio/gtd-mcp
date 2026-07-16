package parser_test

import (
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/parser"
)

func TestParse(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	catalog := &domain.EntityCatalog{
		Projects: []domain.Project{
			{ID: "proj-1", Title: "Project Name"},
			{ID: "proj-2", Title: "Launch"},
		},
		Areas: []domain.Area{
			{ID: "area-1", Name: "Work"},
		},
		People: []domain.Person{
			{ID: "person-1", Name: "Jane Doe"},
		},
		Tags: []string{"#home office"},
	}

	tests := []struct {
		name          string
		input         string
		opts          parser.ParseOptions
		expectedTitle string
		validate      func(t *testing.T, res *parser.ParseResult)
		expectErr     bool
	}{
		{
			name:          "Core Token Parsing & Extraction",
			input:         "Call mom @phone #family /next /due:tomorrow 5pm /note:ask about trip",
			expectedTitle: "Call mom",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.Status == nil || *res.Status != domain.TaskStatusNext {
					t.Errorf("expected status 'next', got %v", res.Status)
				}
				if len(res.Contexts) != 1 || res.Contexts[0] != "@phone" {
					t.Errorf("expected context '@phone', got %v", res.Contexts)
				}
				if len(res.Tags) != 1 || res.Tags[0] != "#family" {
					t.Errorf("expected tag '#family', got %v", res.Tags)
				}
				if res.Description == nil || *res.Description != "ask about trip" {
					t.Errorf("expected description 'ask about trip', got %v", res.Description)
				}
				if res.DueDate == nil || res.DueDate.Hour() != 17 {
					t.Errorf("expected due date at 17:00, got %v", res.DueDate)
				}
			},
		},
		{
			name:          "Energy Level Extraction",
			input:         "Draft proposal /energy:High /next",
			expectedTitle: "Draft proposal",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.EnergyLevel == nil || *res.EnergyLevel != "high" {
					t.Errorf("expected energyLevel 'high', got %v", res.EnergyLevel)
				}
				if res.Status == nil || *res.Status != domain.TaskStatusNext {
					t.Errorf("expected status 'next', got %v", res.Status)
				}
			},
		},
		{
			name:          "Link Command Parsing - No Title",
			input:         "Read source /link:https://example.com/docs#section /next @desk",
			expectedTitle: "Read source",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if len(res.Attachments) != 1 {
					t.Fatalf("expected 1 attachment, got %d", len(res.Attachments))
				}
				att := res.Attachments[0]
				if att.Kind != "link" || att.URI != "https://example.com/docs#section" || att.Title != "example.com/docs" {
					t.Errorf("unexpected attachment properties: kind=%q, uri=%q, title=%q", att.Kind, att.URI, att.Title)
				}
				if res.Status == nil || *res.Status != domain.TaskStatusNext {
					t.Errorf("expected status 'next', got %v", res.Status)
				}
				if len(res.Contexts) != 1 || res.Contexts[0] != "@desk" {
					t.Errorf("expected context '@desk', got %v", res.Contexts)
				}
			},
		},
		{
			name:          "Link Command Parsing - Custom Title",
			input:         "Review plan /link:Sprint Plan | https://example.com/doc",
			expectedTitle: "Review plan",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if len(res.Attachments) != 1 {
					t.Fatalf("expected 1 attachment, got %d", len(res.Attachments))
				}
				att := res.Attachments[0]
				if att.Kind != "link" || att.URI != "https://example.com/doc" || att.Title != "Sprint Plan" {
					t.Errorf("unexpected attachment properties: kind=%q, uri=%q, title=%q", att.Kind, att.URI, att.Title)
				}
			},
		},
		{
			name:          "Explicit Date Commands",
			input:         "Review proposal /start:tomorrow /due:next week",
			expectedTitle: "Review proposal",
			validate: func(t *testing.T, res *parser.ParseResult) {
				tomorrow := now.AddDate(0, 0, 1)
				nextWeek := now.AddDate(0, 0, 7)
				if res.StartTime == nil || res.StartTime.Year() != tomorrow.Year() || res.StartTime.Month() != tomorrow.Month() || res.StartTime.Day() != tomorrow.Day() {
					t.Errorf("expected startTime tomorrow, got %v", res.StartTime)
				}
				if res.DueDate == nil || res.DueDate.Year() != nextWeek.Year() || res.DueDate.Month() != nextWeek.Month() || res.DueDate.Day() != nextWeek.Day() {
					t.Errorf("expected dueDate next week, got %v", res.DueDate)
				}
			},
		},
		{
			name:          "Invalid Date Command",
			input:         "Task /start:monx /due:tomorrow",
			expectedTitle: "Task",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.StartTime != nil {
					t.Errorf("expected startTime nil due to invalid command, got %v", res.StartTime)
				}
				tomorrow := now.AddDate(0, 0, 1)
				if res.DueDate == nil || res.DueDate.Year() != tomorrow.Year() || res.DueDate.Month() != tomorrow.Month() || res.DueDate.Day() != tomorrow.Day() {
					t.Errorf("expected dueDate tomorrow, got %v", res.DueDate)
				}
				if len(res.InvalidDateCommands) != 1 || res.InvalidDateCommands[0] != "/start:monx" {
					t.Errorf("expected invalidDateCommands to contain '/start:monx', got %v", res.InvalidDateCommands)
				}
			},
		},

		{
			name:          "Quoted Multi-word Override",
			input:         `task %"Jane Doe" more words`,
			expectedTitle: "task more words",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.AssignedTo == nil || *res.AssignedTo != "Jane Doe" {
					t.Errorf("expected assignedTo 'Jane Doe', got %v", res.AssignedTo)
				}
			},
		},
		{
			name:          "Unknown Multi-word Tag",
			input:         "Ask %Jim Smith for report",
			expectedTitle: "Ask Smith for report",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.AssignedTo == nil || *res.AssignedTo != "Jim" {
					t.Errorf("expected assignedTo 'Jim', got %v", res.AssignedTo)
				}
			},
		},
		{
			name:          "Container Exclusivity on Capture",
			input:         "+Launch !Work",
			expectedTitle: "Screenshot",
			opts:          parser.ParseOptions{FallbackTitle: "Screenshot"},
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.ProjectID == nil || *res.ProjectID != "proj-2" {
					t.Errorf("expected ProjectID 'proj-2', got %v", res.ProjectID)
				}
				if res.AreaID != nil {
					t.Errorf("expected AreaID nil, got %v", res.AreaID)
				}
				if res.Title != "Screenshot" {
					t.Errorf("expected Title 'Screenshot', got %v", res.Title)
				}
			},
		},
		{
			name:          "Preserve Text Mode",
			input:         "Call mom @phone #family /due:tomorrow",
			opts:          parser.ParseOptions{PreserveText: true},
			expectedTitle: "Call mom @phone #family /due:tomorrow",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if len(res.Contexts) != 1 || res.Contexts[0] != "@phone" {
					t.Errorf("expected context '@phone', got %v", res.Contexts)
				}
				if len(res.Tags) != 1 || res.Tags[0] != "#family" {
					t.Errorf("expected tag '#family', got %v", res.Tags)
				}
			},
		},
		{
			name:          "New Project Parsed but Not Found",
			input:         "Buy paint +New Project",
			expectedTitle: "Buy paint Project",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.ProjectID != nil {
					t.Errorf("expected ProjectID to be nil, got %v", res.ProjectID)
				}
				if res.ProjectTitle == nil || *res.ProjectTitle != "New" {
					var pt string
					if res.ProjectTitle != nil {
						pt = *res.ProjectTitle
					}
					t.Errorf("expected ProjectTitle 'New', got %v", pt)
				}
			},
		},
		{
			name:          "New Area Parsed but Not Found",
			input:         "Buy paint !New Area",
			expectedTitle: "Buy paint Area",
			validate: func(t *testing.T, res *parser.ParseResult) {
				if res.AreaID != nil {
					t.Errorf("expected AreaID to be nil, got %v", res.AreaID)
				}
				if res.AreaName == nil || *res.AreaName != "New" {
					var an string
					if res.AreaName != nil {
						an = *res.AreaName
					}
					t.Errorf("expected AreaName 'New', got %v", an)
				}
			},
		},
		{
			name:      "Empty Title Fails",
			input:     " ",
			opts:      parser.ParseOptions{FallbackTitle: ""},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parser.Parse(tt.input, catalog, tt.opts, now)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Title != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, res.Title)
			}
			if tt.validate != nil {
				tt.validate(t, res)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	input := "  Email Bob  \r\n\nCall Alice\n\t\nReview notes +Work  "
	res := parser.SplitLines(input)
	if len(res) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(res), res)
	}
	if res[0] != "Email Bob" || res[1] != "Call Alice" || res[2] != "Review notes +Work" {
		t.Errorf("unexpected lines: %v", res)
	}
}
