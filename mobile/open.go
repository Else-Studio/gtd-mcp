package mobile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gtd/internal/app"
)

func (g *Gtd) open(r request) string {
	ws := strings.TrimSpace(r.WorkspacePath)
	idx := strings.TrimSpace(r.IndexPath)
	if ws == "" || idx == "" || !filepath.IsAbs(ws) || !filepath.IsAbs(idx) {
		return encodeFail("bad_path", "workspacePath and indexPath must be non-empty absolute paths")
	}
	ws = filepath.Clean(ws)
	idx = filepath.Clean(idx)

	if g.ctx != nil {
		_ = g.ctx.Close()
		g.ctx = nil
	}

	if err := app.Init(ws, idx); err != nil {
		return encodeFail(mapInitOpenError(err))
	}
	ctx, err := app.Open(ws, idx)
	if err != nil {
		return encodeFail(mapInitOpenError(err))
	}
	ctx.SetIndexMaxOpenConns(1)
	g.ctx = ctx
	return encodeOK(map[string]string{"workspacePath": ws})
}

func mapInitOpenError(err error) (string, string) {
	if err == nil {
		return "internal", "unknown error"
	}
	msg := err.Error()
	if errors.Is(err, os.ErrPermission) {
		return "permission", msg
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrInvalid) {
		return "bad_path", msg
	}
	return "internal", msg
}
