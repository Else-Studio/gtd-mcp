package main

import (
	"context"
	_ "embed"
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
	// 1. Static Methodology Resource (Comprehensive guide)
	s.AddResource(mcp.NewResource("gtd://methodology", "GTD Coaching Methodology", mcp.WithResourceDescription("Exhaustive GTD rules for the AI Coach"), mcp.WithMIMEType("text/markdown")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, MIMEType: "text/markdown", Text: methodologyText}}, nil
		})

	// 2. Static Getting Started Guide Resource (Onboarding primer)
	s.AddResource(mcp.NewResource("gtd://guides/getting_started", "GTD Getting Started Guide", mcp.WithResourceDescription("Onboarding primer for setting up Areas, Projects, and capturing"), mcp.WithMIMEType("text/markdown")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: request.Params.URI, MIMEType: "text/markdown", Text: gettingStartedText}}, nil
		})

	// 2. Dynamic State Resource (Real-time system health)
	s.AddResource(mcp.NewResource("gtd://state", "Current GTD System State", mcp.WithResourceDescription("Current metrics like stalled projects and agenda"), mcp.WithMIMEType("application/json")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			stalledOut, _ := executeCLI("stalled")
			agendaOut, _ := executeCLI("agenda")

			stalledText := "{}"
			agendaText := "{}"
			if len(stalledOut.Content) > 0 {
				stalledText = stalledOut.Content[0].(mcp.TextContent).Text
			}
			if len(agendaOut.Content) > 0 {
				agendaText = agendaOut.Content[0].(mcp.TextContent).Text
			}

			state := fmt.Sprintf(`{"stalled_projects_response": %s, "agenda_response": %s}`, stalledText, agendaText)
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
							Text: "I want to do GTD. Please read the gtd://methodology resource to assume your coach persona, then read gtd://state to analyze my system, and finally tell me what I should focus on right now.",
						},
					},
				},
			}, nil
		})
}

const coachInstruction = "Before answering the user, you MUST read the gtd://methodology resource to adopt the GTD Coach persona. If a tool returns an error about the workspace not being initialized or missing, you MUST call the gtd_init tool."

func registerTools(s *server.MCPServer) {
	initTool := mcp.NewTool("gtd_init",
		mcp.WithDescription("Initialize the GTD workspace (creates folders and SQLite index). Use this if you get initialization/not found errors on first run. "+coachInstruction),
	)
	s.AddTool(initTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("init")
	})

	addTool := mcp.NewTool("gtd_task_add",
		mcp.WithDescription("Add a new task via NLP. "+coachInstruction),
		mcp.WithString("text", mcp.Required(), mcp.Description("The task string (e.g. 'Email Bob @computer +Work')")),
	)
	s.AddTool(addTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		text, _ := argsMap["text"].(string)
		return executeCLI("task", "add", text)
	})

	updateTool := mcp.NewTool("gtd_task_update",
		mcp.WithDescription("Update a task's text or status. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The task ID")),
		mcp.WithString("text", mcp.Description("Optional new NLP string to apply")),
		mcp.WithString("status", mcp.Description("Optional new status to set")),
	)
	s.AddTool(updateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		id, _ := argsMap["id"].(string)
		args := []string{"task", "update", id}
		if text, ok := argsMap["text"].(string); ok && text != "" {
			args = append(args, text)
		}
		if status, ok := argsMap["status"].(string); ok && status != "" {
			args = append(args, "--status", status)
		}
		return executeCLI(args...)
	})

	projectAddTool := mcp.NewTool("gtd_project_add",
		mcp.WithDescription("Add a new project. "+coachInstruction),
		mcp.WithString("title", mcp.Required(), mcp.Description("The project title")),
	)
	s.AddTool(projectAddTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		title, _ := argsMap["title"].(string)
		return executeCLI("project", "add", title)
	})

	projectUpdateTool := mcp.NewTool("gtd_project_update",
		mcp.WithDescription("Update a project's status. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The project ID")),
		mcp.WithString("status", mcp.Required(), mcp.Description("The new status (e.g. active, completed, archived)")),
	)
	s.AddTool(projectUpdateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		id, _ := argsMap["id"].(string)
		status, _ := argsMap["status"].(string)
		return executeCLI("project", "update", id, "--status", status)
	})

	agendaTool := mcp.NewTool("gtd_get_agenda",
		mcp.WithDescription("Get the user's agenda (due today, overdue). "+coachInstruction),
	)
	s.AddTool(agendaTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("agenda")
	})

	stalledTool := mcp.NewTool("gtd_get_stalled",
		mcp.WithDescription("Get a list of stalled projects (projects with no next actions). "+coachInstruction),
	)
	s.AddTool(stalledTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("stalled")
	})
	
	inboxTool := mcp.NewTool("gtd_get_inbox",
		mcp.WithDescription("Get the list of unprocessed inbox items. "+coachInstruction),
	)
	s.AddTool(inboxTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("inbox")
	})
}

func executeCLI(args ...string) (*mcp.CallToolResult, error) {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Execution failed: %v\nOutput: %s", err, string(out))), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
