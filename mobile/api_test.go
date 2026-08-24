package mobile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	for _, op := range []string{"nope", "add", "complete", "undoComplete"} {
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

const (
	kitchenProjectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	homeAreaID       = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	idInboxKitchen   = "11111111-1111-4111-8111-111111111111"
	idNextOffice     = "22222222-2222-4222-8222-222222222222"
	idWaitingProj    = "33333333-3333-4333-8333-333333333333"
	idWaitingLoose   = "44444444-4444-4444-8444-444444444444"
	idDoneProj       = "55555555-5555-4555-8555-555555555555"
	idInboxHome      = "66666666-6666-4666-8666-666666666666"
	idRefProj        = "77777777-7777-4777-8777-777777777777"
	idDeletedOffice  = "88888888-8888-4888-8888-888888888888"
	idDuePast        = "99999999-9999-4999-8999-999999999999"
	idDueTomorrow    = "abababab-abab-4bab-8bab-abababababab"
)

func TestInvoke_ListInbox_HydratesBelongsAndProjectTitle(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	writeKitchenProject(t, ws)
	pid := kitchenProjectID
	writeTask(t, ws, &domain.Task{
		ID:        idInboxKitchen,
		Title:     "Buy grout",
		Status:    domain.TaskStatusInbox,
		ProjectID: &pid,
		Tags:      []string{"#errand"},
		Contexts:  []string{"@office"},
	})
	openRebuild(t, g, ws, indexPath)

	env := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	wantOK(t, env)
	tasks := dataTasks(t, env)
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	if tasks[0]["projectTitle"] != "Kitchen" {
		t.Errorf("projectTitle = %v, want Kitchen", tasks[0]["projectTitle"])
	}
	if tasks[0]["belongs"] != "project:Kitchen" {
		t.Errorf("belongs = %v, want project:Kitchen", tasks[0]["belongs"])
	}
}

func TestInvoke_ListContext_Union(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	writeContextUnionFixtures(t, ws)
	openRebuild(t, g, ws, indexPath)

	env := invoke(t, g, map[string]string{"op": "list", "view": "context", "context": "@office"})
	wantOK(t, env)
	ids := dataTaskIDs(t, env)
	want := []string{idInboxKitchen, idNextOffice, idWaitingProj}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("context union ids = %v, want %v", ids, want)
	}

	foundProjected := false
	for _, task := range dataTasks(t, env) {
		belongs, _ := task["belongs"].(string)
		if strings.HasPrefix(belongs, "project:") {
			foundProjected = true
			break
		}
	}
	if !foundProjected {
		t.Fatal("expected a projected task with belongs starting project:")
	}
}

func TestInvoke_ListAgenda_DueToday(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	past := today.AddDate(-1, 0, 0)
	tomorrow := today.AddDate(0, 0, 1)
	writeTask(t, ws, &domain.Task{
		ID:      idDuePast,
		Title:   "Overdue",
		Status:  domain.TaskStatusNext,
		DueDate: &past,
	})
	writeTask(t, ws, &domain.Task{
		ID:      idDueTomorrow,
		Title:   "Tomorrow",
		Status:  domain.TaskStatusNext,
		DueDate: &tomorrow,
	})
	openRebuild(t, g, ws, indexPath)

	env := invoke(t, g, map[string]string{"op": "list", "view": "agenda"})
	wantOK(t, env)
	has := map[string]bool{}
	for _, id := range dataTaskIDs(t, env) {
		has[id] = true
	}
	if !has[idDuePast] {
		t.Error("expected date-only due in the past to be included")
	}
	if has[idDueTomorrow] {
		t.Error("expected date-only due tomorrow to be excluded")
	}
}

func TestInvoke_Catalog_TagsContextsProjectsAreas(t *testing.T) {
	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	writeKitchenProject(t, ws)
	writeHomeArea(t, ws)
	pid := kitchenProjectID
	writeTask(t, ws, &domain.Task{
		ID:        idInboxKitchen,
		Title:     "Buy grout",
		Status:    domain.TaskStatusInbox,
		ProjectID: &pid,
		Tags:      []string{"#errand"},
		Contexts:  []string{"@office"},
	})
	openRebuild(t, g, ws, indexPath)

	env := invoke(t, g, map[string]string{"op": "catalog"})
	wantOK(t, env)
	data := dataMap(t, env)
	if _, ok := data["people"]; ok {
		t.Error("catalog must not emit people")
	}
	if !containsString(asStrings(t, data["tags"]), "#errand") {
		t.Errorf("tags = %v, want to contain #errand", data["tags"])
	}
	if !containsString(asStrings(t, data["contexts"]), "@office") {
		t.Errorf("contexts = %v, want to contain @office", data["contexts"])
	}

	projects := dataObjectArray(t, data["projects"], "projects")
	if len(projects) == 0 {
		t.Fatal("expected at least one project")
	}
	foundKitchen := false
	for _, p := range projects {
		if _, ok := p["id"]; !ok {
			t.Error("project missing id")
		}
		if _, ok := p["title"]; !ok {
			t.Error("project missing title")
		}
		if p["id"] == kitchenProjectID && p["title"] == "Kitchen" {
			foundKitchen = true
		}
	}
	if !foundKitchen {
		t.Errorf("projects = %v, want Kitchen %s", projects, kitchenProjectID)
	}

	areas := dataObjectArray(t, data["areas"], "areas")
	if len(areas) == 0 {
		t.Fatal("expected at least one area")
	}
	foundHome := false
	for _, a := range areas {
		if _, ok := a["id"]; !ok {
			t.Error("area missing id")
		}
		if _, ok := a["name"]; !ok {
			t.Error("area missing name")
		}
		if a["id"] == homeAreaID && a["name"] == "Home" {
			foundHome = true
		}
	}
	if !foundHome {
		t.Errorf("areas = %v, want Home %s", areas, homeAreaID)
	}
}

func TestInvoke_List_NotOpen(t *testing.T) {
	g := newGtd(t)
	env := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	if env["ok"] != false {
		t.Fatalf("ok = %v, want false", env["ok"])
	}
	if errMap(t, env)["code"] != "not_open" {
		t.Fatalf("error.code = %v, want not_open", errMap(t, env)["code"])
	}
}

func TestGoldens_ListCatalog(t *testing.T) {
	encodedInbox, err := json.Marshal(map[string]string{"op": "list", "view": "inbox"})
	if err != nil {
		t.Fatalf("marshal list inbox: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedInbox), unmarshalMap(t, readGolden(t, "list_inbox_request.json"))) {
		t.Errorf("list inbox request does not match testdata/list_inbox_request.json")
	}

	encodedAgenda, err := json.Marshal(map[string]string{"op": "list", "view": "agenda"})
	if err != nil {
		t.Fatalf("marshal list agenda: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedAgenda), unmarshalMap(t, readGolden(t, "list_agenda_request.json"))) {
		t.Errorf("list agenda request does not match testdata/list_agenda_request.json")
	}

	encodedContext, err := json.Marshal(map[string]string{"op": "list", "view": "context", "context": "@office"})
	if err != nil {
		t.Fatalf("marshal list context: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedContext), unmarshalMap(t, readGolden(t, "list_context_request.json"))) {
		t.Errorf("list context request does not match testdata/list_context_request.json")
	}

	encodedCatalog, err := json.Marshal(map[string]string{"op": "catalog"})
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	if !reflect.DeepEqual(unmarshalMap(t, encodedCatalog), unmarshalMap(t, readGolden(t, "catalog_request.json"))) {
		t.Errorf("catalog request does not match testdata/catalog_request.json")
	}

	g := newGtd(t)
	ws, indexPath := tempWorkspace(t)
	writeKitchenProject(t, ws)
	writeHomeArea(t, ws)
	writeContextUnionFixtures(t, ws)
	openRebuild(t, g, ws, indexPath)

	inboxEnv := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	assertContainsKeys(t, inboxEnv, unmarshalMap(t, readGolden(t, "list_inbox_response.json")), "")

	contextEnv := invoke(t, g, map[string]string{"op": "list", "view": "context", "context": "@office"})
	assertContainsKeys(t, contextEnv, unmarshalMap(t, readGolden(t, "list_context_response.json")), "")

	catalogEnv := invoke(t, g, map[string]string{"op": "catalog"})
	assertContainsKeys(t, catalogEnv, unmarshalMap(t, readGolden(t, "catalog_response.json")), "")
}

func writeInboxTask(t *testing.T, ws, id, title string) string {
	t.Helper()
	writeTask(t, ws, &domain.Task{
		ID:     id,
		Title:  title,
		Status: domain.TaskStatusInbox,
	})
	return filepath.Join(ws, "tasks", "tasks")
}

func writeTask(t *testing.T, ws string, task *domain.Task) {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	repo := fs.NewTaskRepository(filepath.Join(ws, "tasks"))
	if err := repo.Save(task); err != nil {
		t.Fatalf("write task %s: %v", task.ID, err)
	}
}

func writeKitchenProject(t *testing.T, ws string) {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := fs.NewProjectRepository(filepath.Join(ws, "projects"))
	if err := repo.Save(&domain.Project{
		ID:        kitchenProjectID,
		Title:     "Kitchen",
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("write project: %v", err)
	}
}

func writeHomeArea(t *testing.T, ws string) {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := fs.NewAreaRepository(filepath.Join(ws, "areas"))
	if err := repo.Save(&domain.Area{
		ID:        homeAreaID,
		Name:      "Home",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("write area: %v", err)
	}
}

func writeContextUnionFixtures(t *testing.T, ws string) {
	t.Helper()
	writeKitchenProject(t, ws)
	pid := kitchenProjectID
	office := []string{"@office"}
	home := []string{"@home"}
	deletedAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	writeTask(t, ws, &domain.Task{
		ID: idInboxKitchen, Title: "Buy grout", Status: domain.TaskStatusInbox,
		ProjectID: &pid, Tags: []string{"#errand"}, Contexts: office, CreatedAt: base,
	})
	writeTask(t, ws, &domain.Task{
		ID: idNextOffice, Title: "Call store", Status: domain.TaskStatusNext,
		Contexts: office, CreatedAt: base.Add(time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idWaitingProj, Title: "Wait for plumber", Status: domain.TaskStatusWaiting,
		ProjectID: &pid, Contexts: office, CreatedAt: base.Add(2 * time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idWaitingLoose, Title: "Waiting loose", Status: domain.TaskStatusWaiting,
		Contexts: office, CreatedAt: base.Add(3 * time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idDoneProj, Title: "Done projected", Status: domain.TaskStatusDone,
		ProjectID: &pid, Contexts: office, CreatedAt: base.Add(4 * time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idInboxHome, Title: "Inbox home", Status: domain.TaskStatusInbox,
		Contexts: home, CreatedAt: base.Add(5 * time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idRefProj, Title: "Ref projected", Status: domain.TaskStatusReference,
		ProjectID: &pid, Contexts: office, CreatedAt: base.Add(6 * time.Second),
	})
	writeTask(t, ws, &domain.Task{
		ID: idDeletedOffice, Title: "Deleted office", Status: domain.TaskStatusInbox,
		Contexts: office, DeletedAt: &deletedAt, CreatedAt: base.Add(7 * time.Second),
	})
}

func openRebuild(t *testing.T, g *Gtd, ws, indexPath string) {
	t.Helper()
	wantOK(t, invoke(t, g, map[string]string{
		"op":            "open",
		"workspacePath": ws,
		"indexPath":     indexPath,
	}))
	wantOK(t, invoke(t, g, map[string]string{"op": "rebuild"}))
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

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func dataTasks(t *testing.T, env map[string]interface{}) []map[string]interface{} {
	t.Helper()
	return dataObjectArray(t, dataMap(t, env)["tasks"], "data.tasks")
}

func dataTaskIDs(t *testing.T, env map[string]interface{}) []string {
	t.Helper()
	tasks := dataTasks(t, env)
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		id, _ := task["id"].(string)
		ids[i] = id
	}
	return ids
}

func dataObjectArray(t *testing.T, v interface{}, path string) []map[string]interface{} {
	t.Helper()
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("%s is not an array: %#v", path, v)
	}
	out := make([]map[string]interface{}, len(arr))
	for i, x := range arr {
		m, ok := x.(map[string]interface{})
		if !ok {
			t.Fatalf("%s[%d] is not an object: %T", path, i, x)
		}
		out[i] = m
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
		assertContainsValue(t, ov, gv, path)
	}
}

func assertContainsValue(t *testing.T, ov, gv interface{}, path string) {
	t.Helper()
	if gm, gIsObj := gv.(map[string]interface{}); gIsObj {
		if ov == nil {
			t.Errorf("%s is null, want object", path)
			return
		}
		om, oIsObj := ov.(map[string]interface{})
		if !oIsObj {
			t.Errorf("%s: want object, got %T", path, ov)
			return
		}
		assertContainsKeys(t, om, gm, path)
		return
	}
	if garr, gIsArr := gv.([]interface{}); gIsArr {
		if ov == nil {
			t.Errorf("%s is null, want array", path)
			return
		}
		oarr, oIsArr := ov.([]interface{})
		if !oIsArr {
			t.Errorf("%s: want array, got %T", path, ov)
			return
		}
		for i, ge := range garr {
			if i >= len(oarr) {
				t.Errorf("%s[%d] missing", path, i)
				continue
			}
			assertContainsValue(t, oarr[i], ge, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}
