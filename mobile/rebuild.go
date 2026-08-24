package mobile

import "time"

type rebuildData struct {
	RebuiltAt        string   `json:"rebuiltAt"`
	Indexed          int      `json:"indexed"`
	SkippedConflicts []string `json:"skippedConflicts"`
	Errors           []string `json:"errors"`
}

func (g *Gtd) rebuild() string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	result, err := g.ctx.RebuildIndex(time.Now().UTC())
	if err != nil {
		return encodeFail("rebuild", err.Error())
	}
	skipped := result.SkippedConflicts
	if skipped == nil {
		skipped = []string{}
	}
	errs := result.Errors
	if errs == nil {
		errs = []string{}
	}
	return encodeOK(rebuildData{
		RebuiltAt:        result.RebuiltAt.UTC().Format(time.RFC3339),
		Indexed:          result.Indexed,
		SkippedConflicts: skipped,
		Errors:           errs,
	})
}
