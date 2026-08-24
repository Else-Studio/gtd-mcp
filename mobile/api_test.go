package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
)

func TestInvoke_Open_IndexOutsideWorkspace(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)

	env := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	})
	wantOK(t, env)
	data := dataMap(t, env)
	if data["workspacePath"] != ws {
		t.Fatalf("workspacePath = %v, want %s", data["workspacePath"], ws)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected index file at %s: %v", indexPath, err)
	}
	if _, err := os.Stat(filepath.Join(ws, "index.db")); !os.IsNotExist(err) {
		t.Fatal("index.db must not be created inside the workspace when indexPath is elsewhere")
	}
}

func TestInvoke_Open_EmptyWorkspacePath_BadPath(t *testing.T) {
	g := newGtd(t)
	_, indexPath := tempWorkspace(t)
	env := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": "",
		"indexPath":     indexPath,
	})
	if env["ok"] != false {
		t.Fatalf("ok = %v, want false", env["ok"])
	}
	if errMap(t, env)["code"] != "bad_path" {
		t.Fatalf("error.code = %v, want bad_path", errMap(t, env)["code"])
	}
}

func TestInvoke_Rebuild_NotOpen(t *testing.T) {
	g := newGtd(t)
	env := invoke(t, g, map[string]string{"op": "rebuild"})
	golden := unmarshalMap(t, readGolden(t, "error_not_open.json"))
	assertContainsKeys(t, env, golden, "")
	if errMap(t, env)["code"] != goldenErrorCode(t, golden) {
		t.Fatalf("error.code = %v, want %s", errMap(t, env)["code"], goldenErrorCode(t, golden))
	}
}

func TestInvoke_Rebuild_FirstTimeMarkdownPresent(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	writeInboxTask(t, ws, "11111111-1111-1111-1111-111111111111", "External capture")

	openEnv := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	})
	wantOK(t, openEnv)

	rebuildEnv := invoke(t, g, map[string]string{"op": "rebuild"})
	wantOK(t, rebuildEnv)
	if n := dataIndexed(t, rebuildEnv); n < 1 {
		t.Fatalf("indexed = %d, want >= 1", n)
	}

	again := invoke(t, g, map[string]string{"op": "rebuild"})
	wantOK(t, again)
	if n := dataIndexed(t, again); n < 1 {
		t.Fatalf("second rebuild indexed = %d, want >= 1", n)
	}
}

func TestInvoke_Rebuild_SkipsSyncConflict(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	id := "22222222-2222-2222-2222-222222222222"
	taskDir := writeInboxTask(t, ws, id, "Keep me")

	src, err := os.ReadFile(filepath.Join(taskDir, id+".md"))
	if err != nil {
		t.Fatalf("read saved task: %v", err)
	}
	conflictName := id + ".sync-conflict-20260824-120000-phone.md"
	if err := os.WriteFile(filepath.Join(taskDir, conflictName), src, 0644); err != nil {
		t.Fatalf("write conflict: %v", err)
	}

	openEnv := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	})
	wantOK(t, openEnv)

	env := invoke(t, g, map[string]string{"op": "rebuild"})
	wantOK(t, env)
	if n := dataIndexed(t, env); n != 1 {
		t.Fatalf("indexed = %d, want 1", n)
	}
	found := false
	for _, name := range asStrings(t, dataMap(t, env)["skippedConflicts"]) {
		if name == conflictName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("skippedConflicts = %v, want to contain %s", dataMap(t, env)["skippedConflicts"], conflictName)
	}
}

func TestInvoke_Rebuild_CollectsParseErrors(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	id := "33333333-3333-3333-3333-333333333333"
	taskDir := writeInboxTask(t, ws, id, "Keep me")
	if err := os.WriteFile(filepath.Join(taskDir, "corrupt-1.md"), []byte(`---
status: [invalid yaml
---
Title`), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	openEnv := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	})
	wantOK(t, openEnv)

	env := invoke(t, g, map[string]string{"op": "rebuild"})
	wantOK(t, env)
	errs := asStrings(t, dataMap(t, env)["errors"])
	if len(errs) == 0 {
		t.Fatal("expected non-empty errors")
	}
	if n := dataIndexed(t, env); n != 1 {
		t.Fatalf("indexed = %d, want 1", n)
	}
}

func TestGoldens_OpenRebuild(t *testing.T) {
	encodedOpen, err := json.Marshal(map[string]string{
		"op":            "open",
		"workspacePath": "/abs/gtd",
		"indexPath":     "/abs/index.db",
	})
	if err != nil {
		t.Fatalf("marshal open request: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedOpen), unmarshalMap(t, readGolden(t, "open_request.json"))) {
		t.Errorf("open request does not match testdata/open_request.json")
	}

	encodedRebuild, err := json.Marshal(map[string]string{"op": "rebuild"})
	if err != nil {
		t.Fatalf("marshal rebuild request: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedRebuild), unmarshalMap(t, readGolden(t, "rebuild_request.json"))) {
		t.Errorf("rebuild request does not match testdata/rebuild_request.json")
	}

	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	openEnv := invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	})
	assertContainsKeys(t, openEnv, unmarshalMap(t, readGolden(t, "open_response.json")), "")

	rebuildEnv := invoke(t, g, map[string]string{"op": "rebuild"})
	assertContainsKeys(t, rebuildEnv, unmarshalMap(t, readGolden(t, "rebuild_response.json")), "")

	notOpen := invoke(t, newGtd(t), map[string]string{"op": "rebuild"})
	notOpenGolden := unmarshalMap(t, readGolden(t, "error_not_open.json"))
	assertContainsKeys(t, notOpen, notOpenGolden, "")
	if errMap(t, notOpen)["code"] != goldenErrorCode(t, notOpenGolden) {
		t.Errorf("not_open code = %v, want %s", errMap(t, notOpen)["code"], goldenErrorCode(t, notOpenGolden))
	}

	badPath := invoke(t, newGtd(t), map[string]string{
		"op":            "open",
		"workspacePath": "",
		"indexPath":     indexPath,
	})
	badPathGolden := unmarshalMap(t, readGolden(t, "error_bad_path.json"))
	assertContainsKeys(t, badPath, badPathGolden, "")
	if errMap(t, badPath)["code"] != goldenErrorCode(t, badPathGolden) {
		t.Errorf("bad_path code = %v, want %s", errMap(t, badPath)["code"], goldenErrorCode(t, badPathGolden))
	}
}

func TestInvoke_UnknownOp_Validation(t *testing.T) {
	g := newGtd(t)
	for _, op := range []string{"nope", "add", "list", "catalog", "complete", "undoComplete"} {
		env := invoke(t, g, map[string]string{"op": op})
		if env["ok"] != false {
			t.Errorf("op %q: ok = %v, want false", op, env["ok"])
		}
		if errMap(t, env)["code"] != "validation" {
			t.Errorf("op %q: error.code = %v, want validation", op, errMap(t, env)["code"])
		}
	}

	env := invoke(t, g, `{`)
	if env["ok"] != false {
		t.Errorf("invalid JSON: ok = %v, want false", env["ok"])
	}
	if errMap(t, env)["code"] != "validation" {
		t.Errorf("invalid JSON: error.code = %v, want validation", errMap(t, env)["code"])
	}
}

func newGtd(t *testing.T) *Gtd {
	t.Helper()
	g := &Gtd{}
	t.Cleanup(func() {
		if g.ctx != nil {
			_ = g.ctx.Close()
		}
	})
	return g
}

func tempWorkspace(t *testing.T) (ws, indexPath string) {
	t.Helper()
	root := t.TempDir()
	return filepath.Join(root, "gtd"), filepath.Join(root, "private", "index.db")
}

func writeInboxTask(t *testing.T, ws, id, title string) string {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := fs.NewTaskRepository(filepath.Join(ws, "tasks"))
	if err := repo.Save(&domain.Task{
		ID:        id,
		Title:     title,
		Status:    domain.TaskStatusInbox,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	return filepath.Join(ws, "tasks", "tasks")
}

func invoke(t *testing.T, g *Gtd, req interface{}) map[string]interface{} {
	t.Helper()
	var raw []byte
	switch v := req.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}
	return unmarshalMap(t, []byte(g.Invoke(string(raw))))
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return b
}

func unmarshalMap(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, b)
	}
	return m
}

func wantOK(t *testing.T, env map[string]interface{}) {
	t.Helper()
	ok, _ := env["ok"].(bool)
	if !ok {
		t.Fatalf("ok = false, envelope = %v", env)
	}
}

func dataMap(t *testing.T, env map[string]interface{}) map[string]interface{} {
	t.Helper()
	d, ok := env["data"].(map[string]interface{})
	if !ok || d == nil {
		t.Fatalf("data is not an object: %#v", env["data"])
	}
	return d
}

func errMap(t *testing.T, env map[string]interface{}) map[string]interface{} {
	t.Helper()
	e, ok := env["error"].(map[string]interface{})
	if !ok || e == nil {
		t.Fatalf("error is not an object: %#v", env["error"])
	}
	return e
}

func goldenErrorCode(t *testing.T, golden map[string]interface{}) string {
	t.Helper()
	code, _ := errMap(t, golden)["code"].(string)
	if code == "" {
		t.Fatal("golden error.code missing")
	}
	return code
}

func dataIndexed(t *testing.T, env map[string]interface{}) int {
	t.Helper()
	n, ok := dataMap(t, env)["indexed"].(float64)
	if !ok {
		t.Fatalf("indexed is not a number: %#v", dataMap(t, env)["indexed"])
	}
	return int(n)
}

func asStrings(t *testing.T, v interface{}) []string {
	t.Helper()
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("want array, got %T (%v)", v, v)
	}
	out := make([]string, len(arr))
	for i, x := range arr {
		s, ok := x.(string)
		if !ok {
			t.Fatalf("want string element, got %T (%v)", x, x)
		}
		out[i] = s
	}
	return out
}

func assertContainsKeys(t *testing.T, got, golden map[string]interface{}, prefix string) {
	t.Helper()
	for k, gv := range golden {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		ov, ok := got[k]
		if !ok {
			t.Errorf("missing key %s", path)
			continue
		}
		gm, gIsObj := gv.(map[string]interface{})
		if !gIsObj {
			continue
		}
		if ov == nil {
			t.Errorf("%s is null, want object", path)
			continue
		}
		om, oIsObj := ov.(map[string]interface{})
		if !oIsObj {
			t.Errorf("%s: want object, got %T", path, ov)
			continue
		}
		assertContainsKeys(t, om, gm, path)
	}
}
