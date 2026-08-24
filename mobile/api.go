// Package mobile is the gomobile JSON facade. Only Gtd.Invoke is Java-visible.
package mobile

import "gtd/internal/app"

// Gtd is the bindable session. All ops go through Invoke as JSON strings.
type Gtd struct {
	ctx *app.Context
}

func (g *Gtd) Invoke(req string) string {
	r, err := decodeRequest(req)
	if err != nil {
		return encodeFail("validation", "invalid JSON")
	}
	if r.Op == "" {
		return encodeFail("validation", "missing op")
	}
	switch r.Op {
	case "open":
		return g.open(r)
	case "rebuild":
		return g.rebuild()
	case "list":
		return g.list(r)
	case "catalog":
		return g.catalog()
	case "add", "complete", "undoComplete":
		return encodeFail("validation", "op "+r.Op+" is not implemented")
	default:
		return encodeFail("validation", "unknown op: "+r.Op)
	}
}
