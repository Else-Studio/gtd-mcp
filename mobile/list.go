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
	case "agenda":
		ids, err = g.ctx.ListAgendaIDs(ctx, time.Now().UTC(), app.TaskListFilter{})
	case "context":
		if r.Context == "" {
			return encodeFail("validation", "context is required when view=context")
		}
		ids, err = g.ctx.ListContextUnionIDs(ctx, r.Context)
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
