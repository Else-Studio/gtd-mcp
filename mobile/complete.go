package mobile

import (
	"errors"

	"gtd/internal/app"
	"gtd/internal/domain"
)

type completeData struct {
	hydratedTask
	PreviousStatus string `json:"previousStatus"`
}

func (g *Gtd) complete(r request) string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	if r.ID == "" {
		return encodeFail("validation", "id is required")
	}
	existing, err := g.ctx.GetTask(r.ID)
	if err != nil {
		return encodeTaskMutateError(err)
	}
	previousStatus := string(existing.Status)
	result, err := g.ctx.UpdateTask(r.ID, app.UpdateTaskOptions{Status: "done"})
	if err != nil {
		return encodeTaskMutateError(err)
	}
	h, err := g.hydratePersisted(result.Task)
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	return encodeOK(completeData{hydratedTask: h, PreviousStatus: previousStatus})
}

func (g *Gtd) undoComplete(r request) string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	if r.ID == "" || r.Status == "" {
		return encodeFail("validation", "id and status are required")
	}
	result, err := g.ctx.UpdateTask(r.ID, app.UpdateTaskOptions{Status: r.Status})
	if err != nil {
		return encodeTaskMutateError(err)
	}
	h, err := g.hydratePersisted(result.Task)
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	return encodeOK(h)
}

func encodeTaskMutateError(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return encodeFail("not_found", err.Error())
	case errors.Is(err, domain.ErrValidation):
		return encodeFail("validation", err.Error())
	default:
		return encodeFail("internal", err.Error())
	}
}
