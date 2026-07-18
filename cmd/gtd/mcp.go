package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

//go:embed methodology.md
var methodologyText string

//go:embed getting_started.md
var gettingStartedText string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the GTD Model Context Protocol (MCP) server",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := server.NewMCPServer("gtd-coach", "1.0.0")

		registerResources(s)
		registerPrompts(s)
		registerTools(s)

		return server.ServeStdio(s)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func registerResources(s *server.MCPServer) {
	s.AddResource(mcp.NewResource(
		"gtd://methodology",
		"GTD Coaching Methodology",
		mcp.WithResourceDescription("Full GTD coach rulebook: phases, NLP tokens, MCP SOPs, tool catalog, filter/clear conventions"),
		mcp.WithMIMEType("text/markdown"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, MIMEType: "text/markdown", Text: methodologyText}}, nil
	})

	s.AddResource(mcp.NewResource(
		"gtd://guides/getting_started",
		"GTD Getting Started Guide",
		mcp.WithResourceDescription("Onboarding primer: areas, projects, initial capture and clarify (CLI + MCP)"),
		mcp.WithMIMEType("text/markdown"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, MIMEType: "text/markdown", Text: gettingStartedText}}, nil
	})

	s.AddResource(mcp.NewResource(
		"gtd://state",
		"Current GTD System State",
		mcp.WithResourceDescription("Health counts only: inbox_count, next_count, agenda_count, stalled_project_count, waiting_count, someday_count, workspace_ok, errors. Use query tools for full lists."),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		state := buildStateJSON()
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, MIMEType: "application/json", Text: state}}, nil
	})
}

func registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("start_gtd_session", mcp.WithPromptDescription("Start a guided GTD coaching session")),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []mcp.PromptMessage{
					{
						Role: mcp.RoleUser,
						Content: mcp.TextContent{
							Type: "text",
							Text: "I want to do GTD. Read gtd://methodology for coach rules, then read gtd://state for health counts " +
								"(inbox, next, agenda, stalled, waiting, someday). Based on those counts, tell me what to focus on now. " +
								"Use MCP tools: gtd_get_inbox / gtd_get_agenda / gtd_get_next / gtd_get_stalled / gtd_task_list as needed — not shell.",
						},
					},
				},
			}, nil
		})
}

// buildStateJSON returns compact health counts by invoking CLI list shortcuts.
func buildStateJSON() string {
	type state struct {
		InboxCount          int      `json:"inbox_count"`
		NextCount           int      `json:"next_count"`
		AgendaCount         int      `json:"agenda_count"`
		StalledProjectCount int      `json:"stalled_project_count"`
		WaitingCount        int      `json:"waiting_count"`
		SomedayCount        int      `json:"someday_count"`
		WorkspaceOK         bool     `json:"workspace_ok"`
		Errors              []string `json:"errors"`
	}
	out := state{WorkspaceOK: true, Errors: []string{}}

	countFrom := func(label string, args ...string) int {
		res, err := executeCLI(args...)
		if err != nil {
			out.WorkspaceOK = false
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", label, err))
			return 0
		}
		if res == nil || len(res.Content) == 0 {
			out.WorkspaceOK = false
			out.Errors = append(out.Errors, label+": empty response")
			return 0
		}
		// Tool errors are returned as text content with IsError set.
		if res.IsError {
			out.WorkspaceOK = false
			text := toolResultText(res)
			out.Errors = append(out.Errors, label+": "+text)
			return 0
		}
		n, parseErr := countJSONDataArray(toolResultText(res))
		if parseErr != nil {
			out.WorkspaceOK = false
			out.Errors = append(out.Errors, label+": "+parseErr.Error())
			return 0
		}
		return n
	}

	out.InboxCount = countFrom("inbox", "inbox")
	out.NextCount = countFrom("next", "next")
	out.AgendaCount = countFrom("agenda", "agenda")
	out.StalledProjectCount = countFrom("stalled", "stalled")
	out.WaitingCount = countFrom("waiting", "task", "list", "waiting")
	out.SomedayCount = countFrom("someday", "task", "list", "someday")

	b, err := json.Marshal(out)
	if err != nil {
		return `{"workspace_ok":false,"errors":["marshal state failed"]}`
	}
	return string(b)
}

func toolResultText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return fmt.Sprint(res.Content[0])
}

// countJSONDataArray parses CLI JSON envelope {"success":true,"data":[...]} and returns len(data).
func countJSONDataArray(raw string) (int, error) {
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return 0, fmt.Errorf("invalid JSON: %w", err)
	}
	if !envelope.Success {
		msg := "command failed"
		if envelope.Error != nil && envelope.Error.Message != "" {
			msg = envelope.Error.Message
		}
		return 0, fmt.Errorf("%s", msg)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return 0, nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(envelope.Data, &arr); err != nil {
		return 0, fmt.Errorf("data is not an array: %w", err)
	}
	return len(arr), nil
}

func registerTools(s *server.MCPServer) {
	// --- Workspace ---
	s.AddTool(mcp.NewTool("gtd_init",
		mcp.WithDescription("Initialize the GTD workspace (dirs + SQLite index.db). Call once before other tools if the workspace is missing. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("init")
	})

	s.AddTool(mcp.NewTool("gtd_index_rebuild",
		mcp.WithDescription("Rebuild the SQLite index from markdown files on disk. Use after external file edits or as weekly-review step 1. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("index", "rebuild")
	})

	// --- Task capture / update ---
	s.AddTool(mcp.NewTool("gtd_task_add",
		mcp.WithDescription("Capture a task via NLP quick-add. Default status is inbox unless text includes /next, /waiting, etc. NLP tokens: +project !area @context #tag %person /due: /start: /recur:. Structured overrides optional. "+coachInstruction),
		mcp.WithString("text", mcp.Required(), mcp.Description("NLP task string, e.g. 'Email Bob @computer +Work /due:tomorrow'")),
		mcp.WithString("project_id", mcp.Description("Optional project UUID override (sets project, clears area)")),
		mcp.WithString("area_id", mcp.Description("Optional area UUID override (sets area, clears project)")),
		mcp.WithString("area", mcp.Description("Optional area name override (sets area, clears project; area must already exist)")),
		mcp.WithString("assigned_to", mcp.Description("Optional assignee name override")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text := stringArg(m, "text")
		args := []string{"task", "add", text}
		args = appendFlagIfNonEmpty(args, m, "project_id", "--project-id")
		args = appendFlagIfNonEmpty(args, m, "area_id", "--area-id")
		args = appendFlagIfNonEmpty(args, m, "area", "--area")
		args = appendFlagIfNonEmpty(args, m, "assigned_to", "--assigned-to")
		return executeCLI(args...)
	})

	s.AddTool(mcp.NewTool("gtd_task_update",
		mcp.WithDescription("Clarify/organize a task: NLP text patch and/or structured fields. Completing to done/archived may return project_stalled + next_action_candidates. "+clearableHelp+" "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task UUID")),
		mcp.WithString("text", mcp.Description("Optional NLP string to merge (title, tokens, dates)")),
		mcp.WithString("status", mcp.Description(taskStatusHelp)),
		mcp.WithString("project_id", mcp.Description("Project UUID; empty string clears project. "+clearableHelp)),
		mcp.WithString("area_id", mcp.Description("Area UUID; sets area and clears project; empty clears area. "+clearableHelp)),
		mcp.WithString("area", mcp.Description("Area name; sets area and clears project; empty clears when used as clear. "+clearableHelp)),
		mcp.WithString("assigned_to", mcp.Description("Assignee name; empty string clears. "+clearableHelp)),
		mcp.WithString("start_offset", mcp.Description("Relative start offset: JSON {\"amount\":-1,\"unit\":\"day\"} or \"-1 day\"; empty clears. "+clearableHelp)),
		mcp.WithString("recurrence", mcp.Description("Recurrence JSON e.g. {\"rule\":\"daily\"}; empty clears. "+clearableHelp)),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id := stringArg(m, "id")
		args := []string{"task", "update", id}
		if text := stringArg(m, "text"); text != "" {
			args = append(args, text)
		}
		args = appendFlagIfNonEmpty(args, m, "status", "--status")
		args = appendFlagIfPresent(args, m, "project_id", "--project-id")
		args = appendFlagIfPresent(args, m, "area_id", "--area-id")
		args = appendFlagIfPresent(args, m, "area", "--area")
		args = appendFlagIfPresent(args, m, "assigned_to", "--assigned-to")
		args = appendFlagIfPresent(args, m, "start_offset", "--start-offset")
		args = appendFlagIfPresent(args, m, "recurrence", "--recurrence")
		return executeCLI(args...)
	})

	s.AddTool(mcp.NewTool("gtd_task_list",
		mcp.WithDescription("List tasks by optional status and filters. Use for waiting/someday/reference/done or all active tasks. Prefer gtd_get_inbox / gtd_get_next / gtd_get_agenda for those specific views. "+filterParamsHelp+" "+coachInstruction),
		mcp.WithString("status", mcp.Description(taskStatusHelp)),
		mcp.WithString("area_id", mcp.Description("Filter by area UUID")),
		mcp.WithString("area", mcp.Description("Filter by area name")),
		mcp.WithString("project_id", mcp.Description("Filter by project UUID")),
		mcp.WithString("project", mcp.Description("Filter by project title")),
		mcp.WithString("context", mcp.Description("Filter by context")),
		mcp.WithString("assigned_to", mcp.Description("Filter by assignee")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := []string{"task", "list"}
		if status := stringArg(m, "status"); status != "" {
			args = append(args, status)
		}
		args = appendTaskFilters(args, m)
		return executeCLI(args...)
	})

	// --- Projects ---
	s.AddTool(mcp.NewTool("gtd_project_add",
		mcp.WithDescription("Create an active project (multi-step outcome). Optionally bind to an area by id or name (name creates area if missing). "+coachInstruction),
		mcp.WithString("title", mcp.Required(), mcp.Description("Project title")),
		mcp.WithString("area_id", mcp.Description("Optional area UUID")),
		mcp.WithString("area", mcp.Description("Optional area name (creates area if it does not exist)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := []string{"project", "add", stringArg(m, "title")}
		args = appendFlagIfNonEmpty(args, m, "area_id", "--area-id")
		args = appendFlagIfNonEmpty(args, m, "area", "--area")
		return executeCLI(args...)
	})

	s.AddTool(mcp.NewTool("gtd_project_update",
		mcp.WithDescription("Update project status and/or area. Provide at least one of status, area_id, area. Status optional: active | someday | completed | archived. "+clearableHelp+" "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project UUID")),
		mcp.WithString("status", mcp.Description("Optional new status: active | someday | completed | archived")),
		mcp.WithString("area_id", mcp.Description("Area UUID; empty string clears area. "+clearableHelp)),
		mcp.WithString("area", mcp.Description("Area name (creates if missing); empty clears when key present. "+clearableHelp)),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := []string{"project", "update", stringArg(m, "id")}
		args = appendFlagIfNonEmpty(args, m, "status", "--status")
		args = appendFlagIfPresent(args, m, "area_id", "--area-id")
		args = appendFlagIfPresent(args, m, "area", "--area")
		return executeCLI(args...)
	})

	// --- Engage shortcuts ---
	s.AddTool(mcp.NewTool("gtd_get_agenda",
		mcp.WithDescription("Engage: tasks due today/overdue or start time reached (excludes reference). Date-only dues include all of due day. "+filterParamsHelp+" "+coachInstruction),
		mcp.WithString("area_id", mcp.Description("Filter by area UUID")),
		mcp.WithString("area", mcp.Description("Filter by area name")),
		mcp.WithString("project_id", mcp.Description("Filter by project UUID")),
		mcp.WithString("project", mcp.Description("Filter by project title")),
		mcp.WithString("context", mcp.Description("Filter by context")),
		mcp.WithString("assigned_to", mcp.Description("Filter by assignee")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, _ := asArgsMap(request)
		args := appendTaskFilters([]string{"agenda"}, m)
		return executeCLI(args...)
	})

	s.AddTool(mcp.NewTool("gtd_get_next",
		mcp.WithDescription("Engage: list tasks with status next (immediately actionable). "+filterParamsHelp+" "+coachInstruction),
		mcp.WithString("area_id", mcp.Description("Filter by area UUID")),
		mcp.WithString("area", mcp.Description("Filter by area name")),
		mcp.WithString("project_id", mcp.Description("Filter by project UUID")),
		mcp.WithString("project", mcp.Description("Filter by project title")),
		mcp.WithString("context", mcp.Description("Filter by context")),
		mcp.WithString("assigned_to", mcp.Description("Filter by assignee")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, _ := asArgsMap(request)
		args := appendTaskFilters([]string{"next"}, m)
		return executeCLI(args...)
	})

	s.AddTool(mcp.NewTool("gtd_get_stalled",
		mcp.WithDescription("Reflect: active projects with zero next-action tasks. Use in weekly review to unblock outcomes. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("stalled")
	})

	s.AddTool(mcp.NewTool("gtd_get_inbox",
		mcp.WithDescription("Clarify: list unprocessed inbox tasks. No filters. Process each with gtd_task_update. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("inbox")
	})

	registerExtraTools(s)
}

func executeCLI(args ...string) (*mcp.CallToolResult, error) {
	bin := gtdExecutable
	if bin == "" {
		bin = os.Args[0]
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Execution failed: %v\nOutput: %s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
