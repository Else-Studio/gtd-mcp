package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestAppendFlagIfPresent(t *testing.T) {
	base := []string{"task", "update", "id1"}

	// absent key
	got := appendFlagIfPresent(base, map[string]interface{}{}, "project_id", "--project-id")
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("absent key should not append, got %v", got)
	}

	// empty string clears
	got = appendFlagIfPresent(base, map[string]interface{}{"project_id": ""}, "project_id", "--project-id")
	want := []string{"task", "update", "id1", "--project-id", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty present: got %v want %v", got, want)
	}

	// non-empty
	got = appendFlagIfPresent(base, map[string]interface{}{"project_id": "abc"}, "project_id", "--project-id")
	want = []string{"task", "update", "id1", "--project-id", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("non-empty: got %v want %v", got, want)
	}
}

func TestAppendFlagIfNonEmpty(t *testing.T) {
	base := []string{"task", "add", "x"}
	m := map[string]interface{}{"area": "", "area_id": "aid"}
	got := appendFlagIfNonEmpty(base, m, "area", "--area")
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("empty should skip, got %v", got)
	}
	got = appendFlagIfNonEmpty(base, m, "area_id", "--area-id")
	want := []string{"task", "add", "x", "--area-id", "aid"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAppendTaskFilters(t *testing.T) {
	m := map[string]interface{}{
		"area_id":     "a1",
		"area":        "Work",
		"project_id":  "p1",
		"project":     "Launch",
		"context":     "computer",
		"assigned_to": "Bob",
		"ignored":     "x",
	}
	got := appendTaskFilters([]string{"next"}, m)
	want := []string{
		"next",
		"--area-id", "a1",
		"--area", "Work",
		"--project-id", "p1",
		"--project", "Launch",
		"--context", "computer",
		"--assigned-to", "Bob",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAsArgsMap(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"text": "hi"}
	m, err := asArgsMap(req)
	if err != nil || m["text"] != "hi" {
		t.Fatalf("asArgsMap: %#v err=%v", m, err)
	}
	req.Params.Arguments = "bad"
	if _, err := asArgsMap(req); err == nil {
		t.Fatal("expected error for non-object args")
	}
}

func TestCountJSONDataArray(t *testing.T) {
	n, err := countJSONDataArray(`{"success":true,"data":[{"id":"1"},{"id":"2"}]}`)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	n, err = countJSONDataArray(`{"success":true,"data":[]}`)
	if err != nil || n != 0 {
		t.Fatalf("empty n=%d err=%v", n, err)
	}
	n, err = countJSONDataArray(`{"success":true,"data":null}`)
	if err != nil || n != 0 {
		t.Fatalf("null n=%d err=%v", n, err)
	}
	if _, err := countJSONDataArray(`{"success":false,"error":{"message":"nope"}}`); err == nil {
		t.Fatal("expected error on failure envelope")
	}
}

func TestMethodologyDocumentsMCPTools(t *testing.T) {
	required := []string{
		"gtd_task_list",
		"gtd_index_rebuild",
		"gtd_task_update",
		"gtd_get_agenda",
		"gtd_get_next",
		"gtd_get_inbox",
		"gtd_get_stalled",
		"gtd_project_update",
		"gtd://state",
		"Clearable fields",
		"Shared list filters",
		"Part 5: MCP Tool Catalog",
	}
	for _, s := range required {
		if !strings.Contains(methodologyText, s) {
			t.Errorf("methodology.md missing %q", s)
		}
	}
}

func TestGettingStartedMentionsMCP(t *testing.T) {
	for _, s := range []string{"gtd_init", "gtd_task_add", "gtd_get_inbox", "gtd://methodology"} {
		if !strings.Contains(gettingStartedText, s) {
			t.Errorf("getting_started.md missing %q", s)
		}
	}
}
