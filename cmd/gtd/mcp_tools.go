package main

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerExtraTools(s *server.MCPServer) {
	// Area Management
	areaAddTool := mcp.NewTool("gtd_area_add",
		mcp.WithDescription("Add a new Area of Focus. "+coachInstruction),
		mcp.WithString("name", mcp.Required(), mcp.Description("The Area name")),
	)
	s.AddTool(areaAddTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		name, _ := argsMap["name"].(string)
		return executeCLI("area", "add", name)
	})

	areaListTool := mcp.NewTool("gtd_area_list", mcp.WithDescription("List all active Areas. "+coachInstruction))
	s.AddTool(areaListTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("area", "list", "--plain")
	})

	areaUpdateTool := mcp.NewTool("gtd_area_update",
		mcp.WithDescription("Update an Area. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Area ID")),
		mcp.WithString("name", mcp.Required(), mcp.Description("The new name")),
	)
	s.AddTool(areaUpdateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		name, _ := argsMap["name"].(string)
		return executeCLI("area", "update", id, "--name", name)
	})

	areaDeleteTool := mcp.NewTool("gtd_area_delete",
		mcp.WithDescription("Soft-delete an Area (cascades to projects/tasks). "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Area ID")),
	)
	s.AddTool(areaDeleteTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("area", "delete", id)
	})

	areaRestoreTool := mcp.NewTool("gtd_area_restore",
		mcp.WithDescription("Restore a soft-deleted Area (cascades). "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Area ID")),
	)
	s.AddTool(areaRestoreTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("area", "restore", id)
	})

	// People Management
	peopleListTool := mcp.NewTool("gtd_people_list", mcp.WithDescription("List all active People. "+coachInstruction))
	s.AddTool(peopleListTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("people", "list", "--plain")
	})

	peopleAddTool := mcp.NewTool("gtd_people_add",
		mcp.WithDescription("Add a new Person. "+coachInstruction),
		mcp.WithString("name", mcp.Required(), mcp.Description("The Person name")),
	)
	s.AddTool(peopleAddTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		name, _ := argsMap["name"].(string)
		return executeCLI("people", "add", name)
	})

	peopleUpdateTool := mcp.NewTool("gtd_people_update",
		mcp.WithDescription("Update a Person. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Person ID")),
		mcp.WithString("name", mcp.Required(), mcp.Description("The new name")),
	)
	s.AddTool(peopleUpdateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		name, _ := argsMap["name"].(string)
		return executeCLI("people", "update", id, "--name", name)
	})

	peopleDeleteTool := mcp.NewTool("gtd_people_delete",
		mcp.WithDescription("Soft-delete a Person. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Person ID")),
	)
	s.AddTool(peopleDeleteTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("people", "delete", id)
	})

	// Project Maintenance
	projectListTool := mcp.NewTool("gtd_project_list", mcp.WithDescription("List all active Projects. "+coachInstruction))
	s.AddTool(projectListTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return executeCLI("project", "list", "--plain")
	})

	projectDeleteTool := mcp.NewTool("gtd_project_delete",
		mcp.WithDescription("Soft-delete a Project (cascades). "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Project ID")),
	)
	s.AddTool(projectDeleteTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("project", "delete", id)
	})

	projectRestoreTool := mcp.NewTool("gtd_project_restore",
		mcp.WithDescription("Restore a soft-deleted Project (cascades). "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Project ID")),
	)
	s.AddTool(projectRestoreTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("project", "restore", id)
	})

	// Task Maintenance
	taskDeleteTool := mcp.NewTool("gtd_task_delete",
		mcp.WithDescription("Soft-delete a Task. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Task ID")),
	)
	s.AddTool(taskDeleteTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("task", "delete", id)
	})

	taskRestoreTool := mcp.NewTool("gtd_task_restore",
		mcp.WithDescription("Restore a soft-deleted Task. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Task ID")),
	)
	s.AddTool(taskRestoreTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("task", "restore", id)
	})

	taskDuplicateTool := mcp.NewTool("gtd_task_duplicate",
		mcp.WithDescription("Duplicate a Task. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Task ID")),
	)
	s.AddTool(taskDuplicateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		return executeCLI("task", "duplicate", id)
	})

	taskPromoteTool := mcp.NewTool("gtd_task_promote",
		mcp.WithDescription("Promote a Task to a Project. "+coachInstruction),
		mcp.WithString("id", mcp.Required(), mcp.Description("The Task ID")),
		mcp.WithString("project_title", mcp.Required(), mcp.Description("The new Project Title")),
	)
	s.AddTool(taskPromoteTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argsMap, _ := request.Params.Arguments.(map[string]interface{})
		id, _ := argsMap["id"].(string)
		title, _ := argsMap["project_title"].(string)
		return executeCLI("task", "promote", id, title)
	})
}
