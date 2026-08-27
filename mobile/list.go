package mobile

import (
	"context"
	"time"

	"gtd/internal/app"
)

type listData struct {
	Tasks []hydratedTask `json:"tasks"`
}

func (g *Gtd) list(r request) string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	ctx := context.Background()
	var ids []string
	var err error
	switch r.View {
	case "inbox":
		ids, err = g.ctx.ListInboxIDs(ctx)
	case "next":
		ids, err = g.ctx.ListNextIDs(ctx, app.TaskListFilter{})
	case "agenda":
		now := time.Now()
		if r.Now != nil {
			if parsed, parseErr := time.Parse(time.RFC3339, *r.Now); parseErr == nil {
				now = parsed
			}
		}
		ids, err = g.ctx.ListAgendaIDs(ctx, now, app.TaskListFilter{})
	case "context":
		if r.Context == "" {
			return encodeFail("validation", "context is required when view=context")
		}
		ids, err = g.ctx.ListTaskIDs("", app.TaskListFilter{Context: r.Context})
	case "project":
		if r.ProjectId == "" && r.Project == "" {
			return encodeFail("validation", "project_id or project is required when view=project")
		}
		ids, err = g.ctx.ListTaskIDs("", app.TaskListFilter{ProjectID: r.ProjectId, ProjectTitle: r.Project})
	default:
		return encodeFail("validation", "unknown or missing view")
	}
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	tasks, err := g.hydrateIDs(ids)
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	return encodeOK(listData{Tasks: tasks})
}
