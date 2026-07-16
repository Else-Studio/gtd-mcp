package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Shared agent-facing description snippets (kept consistent across tools).
const (
	filterParamsHelp = "Optional filters (omit if empty): area_id (UUID), area (name), " +
		"project_id (UUID), project (title), context (e.g. computer or @computer), " +
		"assigned_to (person name). Prefer names when the user speaks names; use IDs from prior tool results."

	clearableHelp = "Clearable fields: pass empty string \"\" to clear; omit the key to leave unchanged."

	taskStatusHelp = "status: inbox | next | waiting | someday | reference | done | archived. " +
		"Omit status on list = all active (non-deleted) tasks."

	coachInstruction = "Before coaching, read gtd://methodology for GTD rules and gtd://state for system health. " +
		"If workspace-not-initialized errors appear, call gtd_init first. " +
		"Use MCP tools (gtd_*), not shell. Prefer gtd_task_list for waiting/someday/reference; " +
		"prefer gtd_get_agenda / gtd_get_next / gtd_get_inbox for engage lists."
)

// gtdExecutable overrides the binary path used by executeCLI.
// Empty means os.Args[0] (correct when the host runs `gtd mcp`). Tests set this.
var gtdExecutable string

func asArgsMap(request mcp.CallToolRequest) (map[string]interface{}, error) {
	if request.Params.Arguments == nil {
		return map[string]interface{}{}, nil
	}
	m, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid arguments: expected object")
	}
	return m, nil
}

// appendFlagIfPresent appends --flag value when key is present (including empty string for clear).
func appendFlagIfPresent(args []string, m map[string]interface{}, key, flag string) []string {
	if m == nil {
		return args
	}
	v, ok := m[key]
	if !ok {
		return args
	}
	s, ok := v.(string)
	if !ok {
		return args
	}
	return append(args, flag, s)
}

// appendFlagIfNonEmpty appends --flag value only when key is a non-empty string.
func appendFlagIfNonEmpty(args []string, m map[string]interface{}, key, flag string) []string {
	if m == nil {
		return args
	}
	s, ok := m[key].(string)
	if !ok || s == "" {
		return args
	}
	return append(args, flag, s)
}

// appendTaskFilters wires the six shared list/agenda/next filter keys (non-empty only).
func appendTaskFilters(args []string, m map[string]interface{}) []string {
	args = appendFlagIfNonEmpty(args, m, "area_id", "--area-id")
	args = appendFlagIfNonEmpty(args, m, "area", "--area")
	args = appendFlagIfNonEmpty(args, m, "project_id", "--project-id")
	args = appendFlagIfNonEmpty(args, m, "project", "--project")
	args = appendFlagIfNonEmpty(args, m, "context", "--context")
	args = appendFlagIfNonEmpty(args, m, "assigned_to", "--assigned-to")
	return args
}

// stringArg returns a string value for key, or empty if missing/non-string.
func stringArg(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
