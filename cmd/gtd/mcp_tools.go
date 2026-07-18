package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerExtraTools(s *server.MCPServer) {
	// --- Areas ---
	s.AddTool(mcp.NewTool("gtd_area_add",
		mcp.WithDescription("Add a new Area of Focus (ongoing responsibility, e.g. Work, Home). "+coachInstruction),
		mcp.WithString("name", mcp.Required(), mcp.Description("Area name")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("area", "add", stringArg(m, "name"))
	})

	s.AddTool(mcp.NewTool("gtd_area_list",
		mcp.WithDescription("List all active Areas as JSON. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("area", "list")
	})

	s.AddTool(mcp.NewTool("gtd_area_update",
		mcp.WithDescription("Rename an Area by ID. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Area UUID")),
		mcp.WithString("name", mcp.Required(), mcp.Description("New area name")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("area", "update", stringArg(m, "id"), "--name", stringArg(m, "name"))
	})

	s.AddTool(mcp.NewTool("gtd_area_delete",
		mcp.WithDescription("Soft-delete an Area; cascades soft-delete to child projects and tasks. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Area UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("area", "delete", stringArg(m, "id"))
	})

	s.AddTool(mcp.NewTool("gtd_area_restore",
		mcp.WithDescription("Restore a soft-deleted Area; cascades restore to child projects and tasks. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Area UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("area", "restore", stringArg(m, "id"))
	})

	// --- People ---
	s.AddTool(mcp.NewTool("gtd_people_list",
		mcp.WithDescription("List all active People (delegates) as JSON. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("people", "list")
	})

	// --- Derived catalogs (free-form labels on tasks) ---
	s.AddTool(mcp.NewTool("gtd_context_list",
		mcp.WithDescription("List distinct contexts currently used on non-deleted tasks (e.g. @computer). Contexts are free-form, not first-class entities. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("context", "list")
	})

	s.AddTool(mcp.NewTool("gtd_tag_list",
		mcp.WithDescription("List distinct tags currently used on non-deleted tasks (e.g. #weekend). Tags are free-form, not first-class entities. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("tag", "list")
	})

	s.AddTool(mcp.NewTool("gtd_people_add",
		mcp.WithDescription("Add a Person used for waiting-for / delegation (%name). "+coachInstruction),
		mcp.WithString("name", mcp.Required(), mcp.Description("Person name")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("people", "add", stringArg(m, "name"))
	})

	s.AddTool(mcp.NewTool("gtd_people_update",
		mcp.WithDescription("Rename a Person by ID. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Person UUID")),
		mcp.WithString("name", mcp.Required(), mcp.Description("New name")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("people", "update", stringArg(m, "id"), "--name", stringArg(m, "name"))
	})

	s.AddTool(mcp.NewTool("gtd_people_delete",
		mcp.WithDescription("Soft-delete a Person. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Person UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("people", "delete", stringArg(m, "id"))
	})

	// --- Project maintenance ---
	s.AddTool(mcp.NewTool("gtd_project_list",
		mcp.WithDescription("List all active Projects as JSON. "+coachInstruction),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("project", "list")
	})

	s.AddTool(mcp.NewTool("gtd_project_delete",
		mcp.WithDescription("Soft-delete a Project; cascades soft-delete to child tasks. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("project", "delete", stringArg(m, "id"))
	})

	s.AddTool(mcp.NewTool("gtd_project_restore",
		mcp.WithDescription("Restore a soft-deleted Project; cascades restore to child tasks. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("project", "restore", stringArg(m, "id"))
	})

	// --- Task maintenance ---
	s.AddTool(mcp.NewTool("gtd_task_delete",
		mcp.WithDescription("Soft-delete a Task (hidden from normal lists, kept in history). "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("task", "delete", stringArg(m, "id"))
	})

	s.AddTool(mcp.NewTool("gtd_task_restore",
		mcp.WithDescription("Restore a soft-deleted Task into active rotation. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("task", "restore", stringArg(m, "id"))
	})

	s.AddTool(mcp.NewTool("gtd_task_duplicate",
		mcp.WithDescription("Deep-copy a Task with a new ID; clone status is next and completedAt is cleared. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Source task UUID")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("task", "duplicate", stringArg(m, "id"))
	})

	s.AddTool(mcp.NewTool("gtd_task_promote",
		mcp.WithDescription("Promote a Task into a Project: creates or reuses project by title, links task, clears task area. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("Task UUID")),
		mcp.WithString("project_title", mcp.Required(), mcp.Description("New or existing project title")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, err := asArgsMap(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return executeCLI("task", "promote", stringArg(m, "id"), stringArg(m, "project_title"))
	})
}
