package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zapolnyaka/internal/emu"
)

// fakeActions — команды-заглушки: пишут в лог и запоминают вызовы.
type fakeActions struct{ calls []string }

func (f *fakeActions) ScanGames() []string { return []string{"testdata/game/game.yml"} }
func (f *fakeActions) Login() string        { return "tester" }
func (f *fakeActions) Go(gamePath string, levels []int, log io.Writer) error {
	f.calls = append(f.calls, fmt.Sprintf("go %v", levels))
	fmt.Fprintf(log, "▶ Уровень 1\n✔ уровень 1 завершён\n")
	return nil
}
func (f *fakeActions) Assets(gamePath string, log io.Writer) error   { fmt.Fprintln(log, "assets ok"); return nil }
func (f *fakeActions) Validate(gamePath string, log io.Writer) error { fmt.Fprintln(log, "validate ok"); return nil }
func (f *fakeActions) Check(gamePath string, log io.Writer) error    { return fmt.Errorf("нет сети") }
func (f *fakeActions) Snapshot(gamePath, name, send string, pid, pact int, log io.Writer) error {
	return nil
}
func (f *fakeActions) Auth(login, password string) error                     { f.calls = append(f.calls, "auth "+login); return nil }
func (f *fakeActions) NewGame(name, domain string, gameID int) (string, error) { return "data/" + name + "/game.yml", nil }
func (f *fakeActions) NewLevel(gamePath, dir string, num int) error           { f.calls = append(f.calls, "level "+dir); return nil }
func (f *fakeActions) SaveLastGame(gamePath string)                          {}

func copyFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "emu", "testdata", "game")
	dst := filepath.Join(t.TempDir(), "game")
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dst, "game.yml")
}

type env struct {
	ts   *httptest.Server
	acts *fakeActions
	game string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	game := copyFixture(t)
	srv, err := emu.New(emu.Options{GamePath: game, Login: "tester", Now: func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	acts := &fakeActions{}
	Mount(srv.Mux(), Deps{Emu: srv, Actions: acts, Version: "test"})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &env{ts: ts, acts: acts, game: game}
}

func (e *env) do(t *testing.T, method, path string, body any, raw bool) (int, string) {
	t.Helper()
	var rdr io.Reader
	ct := "application/json"
	if body != nil {
		if raw {
			rdr, ct = strings.NewReader(body.(string)), "text/plain"
		} else {
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rdr)
	req.Header.Set("Content-Type", ct)
	resp, err := e.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestStateAndLevel(t *testing.T) {
	e := newEnv(t)
	code, body := e.do(t, "GET", "/ui/", nil, false)
	if code != 200 || !strings.Contains(body, "zapolnyaka") {
		t.Fatalf("ui page: %d", code)
	}
	code, body = e.do(t, "GET", "/api/ui/state", nil, false)
	if code != 200 {
		t.Fatalf("state: %d %s", code, body)
	}
	var st stateResponse
	_ = json.Unmarshal([]byte(body), &st)
	if st.Login != "tester" || len(st.Levels) != 3 || st.Levels[0].Number != 1 || st.Levels[0].Codes != 2 || len(st.Assets) != 1 {
		t.Fatalf("state: %+v", st)
	}
	code, body = e.do(t, "GET", "/api/ui/level/2", nil, false)
	var d LevelData
	_ = json.Unmarshal([]byte(body), &d)
	if code != 200 || len(d.Codes) != 3 || d.Conf == nil || d.Conf.Autopass == nil || *d.Conf.Autopass != 600 || !strings.Contains(d.Body, "Level two") || d.Raw["conf"] == "" {
		t.Fatalf("level 2: %d %+v", code, d)
	}
}

func TestSaveCodesConfRaw(t *testing.T) {
	e := newEnv(t)
	_, body := e.do(t, "GET", "/api/ui/level/1", nil, false)
	var d LevelData
	_ = json.Unmarshal([]byte(body), &d)
	codes := append(d.Codes, d.Codes[1]) // ещё один бонус
	name := "Второй бонус"
	codes[2].BonusName = &name
	if code, body := e.do(t, "PUT", "/api/ui/level/1/codes", codes, false); code != 200 {
		t.Fatalf("save codes: %d %s", code, body)
	}
	_, body = e.do(t, "GET", "/api/ui/level/1", nil, false)
	_ = json.Unmarshal([]byte(body), &d)
	if len(d.Codes) != 3 || d.Codes[2].BonusName == nil || *d.Codes[2].BonusName != name {
		t.Fatalf("codes after save: %+v", d.Codes)
	}
	// Бонус без времени не проходит проверку и файл не портится.
	bad := []map[string]any{{"type": "бонус", "bonusName": "x", "answers": []string{"a"}}}
	if code, body := e.do(t, "PUT", "/api/ui/level/1/codes", bad, false); code != 400 || !strings.Contains(body, "time") {
		t.Fatalf("invalid codes must be rejected: %d %s", code, body)
	}
	_, body = e.do(t, "GET", "/api/ui/level/1", nil, false)
	_ = json.Unmarshal([]byte(body), &d)
	if len(d.Codes) != 3 {
		t.Fatalf("file must stay intact after rejected save")
	}

	// conf: подсказка и автопереход.
	conf := map[string]any{"name": "Первый!", "autopass": 120, "hints": []map[string]any{{"time": 5, "text": "новая"}}, "clean": true}
	if code, body := e.do(t, "PUT", "/api/ui/level/1/conf", conf, false); code != 200 {
		t.Fatalf("save conf: %d %s", code, body)
	}
	_, body = e.do(t, "GET", "/api/ui/level/1", nil, false)
	_ = json.Unmarshal([]byte(body), &d)
	if *d.Conf.Name != "Первый!" || *d.Conf.Autopass != 120 || len(d.Conf.Hints) != 1 || d.Conf.Codes == nil || *d.Conf.Codes != "codes.yml" {
		t.Fatalf("conf after save: %+v", d.Conf)
	}

	// raw: тело пишется как есть, плохой YAML отклоняется.
	if code, _ := e.do(t, "PUT", "/api/ui/level/1/raw/body", "<p>new body</p>\n", true); code != 200 {
		t.Fatalf("raw body save")
	}
	if code, body := e.do(t, "PUT", "/api/ui/level/1/raw/conf", "level: [", true); code != 400 || !strings.Contains(body, "проверка") {
		t.Fatalf("bad yaml must be rejected: %d %s", code, body)
	}
	_, body = e.do(t, "GET", "/api/ui/level/1", nil, false)
	_ = json.Unmarshal([]byte(body), &d)
	if d.Body != "<p>new body</p>\n" || *d.Conf.Name != "Первый!" {
		t.Fatalf("raw save result: %q", d.Body)
	}
}

func TestJobsAndPreview(t *testing.T) {
	e := newEnv(t)
	code, body := e.do(t, "POST", "/api/ui/run", map[string]any{"cmd": "go", "levels": []int{1, 3}}, false)
	if code != 200 {
		t.Fatalf("run: %d %s", code, body)
	}
	var r struct{ ID int }
	_ = json.Unmarshal([]byte(body), &r)
	var res struct {
		Job   jobStatus `json:"job"`
		Lines []string  `json:"lines"`
		Total int       `json:"total"`
	}
	for i := 0; i < 50; i++ {
		_, body = e.do(t, "GET", fmt.Sprintf("/api/ui/jobs/%d?from=0", r.ID), nil, false)
		_ = json.Unmarshal([]byte(body), &res)
		if res.Job.Done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !res.Job.Done || res.Job.Error != "" || !strings.Contains(strings.Join(res.Lines, "\n"), "✔ уровень 1 завершён") || !strings.Contains(res.Lines[len(res.Lines)-1], "готово") {
		t.Fatalf("job: %+v %v", res.Job, res.Lines)
	}
	if len(e.acts.calls) == 0 || e.acts.calls[0] != "go [1 3]" {
		t.Fatalf("calls: %v", e.acts.calls)
	}
	// Ошибка команды попадает в статус.
	_, body = e.do(t, "POST", "/api/ui/run", map[string]any{"cmd": "check"}, false)
	_ = json.Unmarshal([]byte(body), &r)
	for i := 0; i < 50; i++ {
		_, body = e.do(t, "GET", fmt.Sprintf("/api/ui/jobs/%d", r.ID), nil, false)
		_ = json.Unmarshal([]byte(body), &res)
		if res.Job.Done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if res.Job.Error != "нет сети" {
		t.Fatalf("job error: %+v", res.Job)
	}

	code, body = e.do(t, "GET", "/ui/preview/1?fog=0", nil, false)
	if code != 200 || !strings.Contains(body, "Level one body") || !strings.Contains(body, "<h3>Подсказка 2</h3>") || !strings.Contains(body, "bonus help text") {
		t.Fatalf("preview: %d", code)
	}
	code, body = e.do(t, "GET", "/api/ui/preview/1/checks", nil, false)
	if code != 200 || !strings.Contains(body, "@import") {
		t.Fatalf("checks: %d %s", code, body)
	}
	if code, _ := e.do(t, "POST", "/api/ui/level/new", map[string]any{"dir": "../x", "number": 9}, false); code != 400 {
		t.Fatalf("dir traversal must be rejected")
	}
}
