package mobile

import (
	"errors"
	"strings"

	"gtd/internal/app"
	"gtd/internal/domain"
)

func (g *Gtd) add(r request) string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	text := strings.TrimSpace(r.Text)
	if text == "" {
		return encodeFail("validation", "text is required")
	}
	result, err := g.ctx.CreateTask(app.CreateTaskOptions{Text: text})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return encodeFail("validation", err.Error())
		}
		return encodeFail("internal", err.Error())
	}
	h, err := g.hydratePersisted(result.Task)
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	return encodeOK(h)
}
