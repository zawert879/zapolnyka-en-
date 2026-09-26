package emu

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

const fixturePlay = "/gameengines/encounter/play/1/"

type testEnv struct {
	srv    *Server
	ts     *httptest.Server
	client *http.Client
	clock  *fakeClock
	data   string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	data := t.TempDir()
	srv, err := New(Options{GamePath: fixtureGame, DataDir: data, Now: clock.Now, Login: "tester"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	client := ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &testEnv{srv: srv, ts: ts, client: client, clock: clock, data: data}
}

func (e *testEnv) get(t *testing.T, path string) (int, string) {
	t.Helper()
	resp, err := e.client.Get(e.ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *testEnv) answer(t *testing.T, code string) (int, string) {
	t.Helper()
	resp, err := e.client.PostForm(e.ts.URL+fixturePlay, url.Values{"LevelAction.Answer": {code}})
	if err != nil {
		t.Fatalf("POST answer: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *testEnv) action(t *testing.T, level int, body map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := e.client.Post(e.ts.URL+"/api/state/"+strconv.Itoa(level), "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST state: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != 200 {
		t.Fatalf("state %v: %d %v", body, resp.StatusCode, out)
	}
	return out
}

func (e *testEnv) panel(t *testing.T, level int) map[string]any {
	t.Helper()
	_, body := e.get(t, "/api/level/"+strconv.Itoa(level))
	var out struct {
		Panel map[string]any `json:"panel"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("panel json: %v: %s", err, body)
	}
	return out.Panel
}

func (e *testEnv) goto_(t *testing.T, level int) {
	t.Helper()
	if code, _ := e.get(t, "/play/"+strconv.Itoa(level)); code != 303 {
		t.Fatalf("goto %d: status %d", level, code)
	}
}

func TestPlayPage(t *testing.T) {
	e := newTestEnv(t)
	code, body := e.get(t, fixturePlay)
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{
		`id="Answer"`, `name="LevelAction.Answer"`, `<div id="ordinary_helps">`, `<div id="penalty_helps">`, `<div id="bonuses">`,
		`Level one body`, `/assets/design.css`, `<h2>Уровень <span>1</span> из 3: Первый</h2>`,
		"\t\t<h3>Подсказка 1</h3>\r\n\t\t<p>сразу</p>", `emu-panel-host`, `https://world.en.cx/css/v2/en/engines/engine.css?ver=` + EngineVer,
		"\t<h3>Задание</h3>\r\n\t<div class=\"task\">\r\n\t<p>", `<h3 class="bonus_count">На уровне 1 бонус </h3>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page must contain %q", want)
		}
	}
	if strings.Contains(body, "через две минуты") || strings.Contains(body, `<link rel="stylesheet" href="https://example.com`) {
		t.Errorf("hidden hint or bare <link> leaked into the page")
	}
	// Один сектор — списка секторов нет; пустая история — две пустые строки.
	if strings.Contains(body, "cols-wrapper") || !strings.Contains(body, "<ul class=\"history\">\r\n\r\n\r\n\t\t</ul>") {
		t.Errorf("single sector must not render sector list; empty history must match engine")
	}

	// Корень и /play/{n} ведут на канонический путь; без слэша — редирект.
	if code, _ := e.get(t, "/"); code != 302 {
		t.Errorf("root status %d", code)
	}
	if code, _ := e.get(t, strings.TrimSuffix(fixturePlay, "/")); code != 302 {
		t.Errorf("no-slash status %d", code)
	}
	if code, _ := e.get(t, "/play/99"); code != 404 {
		t.Errorf("unknown level status %d", code)
	}
	// ?json=1 отдаёт модель движка.
	_, js := e.get(t, fixturePlay+"?json=1")
	if !strings.Contains(js, `"Number":1`) || !strings.Contains(js, `"Helps"`) {
		t.Errorf("json model: %s", js[:200])
	}
}

func TestAnswerFlowAndAutoAdvance(t *testing.T) {
	e := newTestEnv(t)

	code, body := e.answer(t, "wrong")
	if code != 200 {
		t.Fatalf("wrong answer status %d", code)
	}
	for _, want := range []string{
		"\t\t\t\t <li id=\"incorrect\">Ответ или код неверный</li> \r\n",
		`autocomplete="off" value="wrong" />`,
		"<span class=\"color_incorrect\">\r\n\t\t\t\t\t\t<i>wrong</i>\r\n",
		`<a href="/userdetails.aspx?uid=0">tester</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wrong-answer page must contain %q", want)
		}
	}
	_, body = e.get(t, fixturePlay)
	if strings.Contains(body, `id="incorrect"`) || !strings.Contains(body, `<span class="incorrect">`) || !strings.Contains(body, `value="" />`) {
		t.Fatalf("GET after wrong answer: notice must vanish, span class incorrect, input empty")
	}

	_, body = e.answer(t, "BETA")
	for _, want := range []string{
		"<li class=\"color_correct\">Ответ или код верный</li> \r\n",
		"<span class=\"color_bonus\">\r\n\t\t\t\t\t\tBETA\r\n",
		"(выполнен, награда 1 минуту)</span>\t\t\t\r\n",
		"[ <span class=\"color_bonus\">BETA</span> ]</span>\r\n\t\t\t<p>bonus help text</p>",
		`(Выполненные - 1)`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("bonus page must contain %q", want)
		}
	}

	// Закрытие единственного сектора: ответом приходит страница уровня 2 с записью «(1)».
	code, body = e.answer(t, "альфа")
	if code != 200 || !strings.Contains(body, `<h2>Уровень <span>2</span> из 3: Второй</h2>`) {
		t.Fatalf("closing the sector must render level 2: %d", code)
	}
	if !strings.Contains(body, "12:00:00\r\n\t\t\t\t\t(1)\r\n\r\n") || !strings.Contains(body, "<li class=\"color_correct\">Ответ или код верный</li>") {
		t.Fatalf("carried history entry with level mark expected")
	}
	_, body = e.get(t, fixturePlay)
	if strings.Contains(body, "(1)\r\n") {
		t.Fatalf("carried entry must not persist on GET")
	}
	p := e.panel(t, 1)
	if p["passed"] != true || p["closed"].(float64) != 1 {
		t.Fatalf("level 1 panel: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(e.data, ".emu-state.json")); err != nil {
		t.Fatalf("state file: %v", err)
	}
}

func TestSectorsBlockAndAutoAdvanceOff(t *testing.T) {
	e := newTestEnv(t)
	e.goto_(t, 2)
	e.action(t, 2, map[string]any{"action": "autoAdvance", "on": false})
	_, body := e.get(t, fixturePlay)
	for _, want := range []string{
		"\t<h3>На уровне 2 сектора \r\n\t\t<span class=\"color_sec\">(осталось закрыть 1)</span>\r\n\t</h3>\r\n\t<div class=\"cols-wrapper\">\r\n\t\t<div class=\"cols w100per\"> \t\t\r\n\t\t\r\n\r\n",
		`<span class="sector_name">S1</span>: <span class="color_dis">код не введён</span></p>`,
		"\t\t</div><!--end cols-->\r\n\t</div><!--end cols-wrapper -->\r\n\r\n\t<div class=\"spacer\"></div>\r\n\t<h3>Задание</h3>",
		"<strong>Автопереход</strong> на следующий уровень через&nbsp;<span class=\"bold_off\" id=\"time",
		"(штраф&nbsp;1 минуту)",
		`Взять подсказку (штраф 1 минуту)</a></p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("level 2 page must contain %q", want)
		}
	}

	_, body = e.answer(t, "one")
	if !strings.Contains(body, `<span class="color_correct">one</span> <span class="color_sec">(01.01.2026 12:00:00 <a href="/UserDetails.aspx?uid=0">tester</a>)</span></p>`) {
		t.Fatalf("closed sector line expected")
	}
	if !strings.Contains(body, `<h2>Уровень <span>2</span>`) {
		t.Fatalf("with autoAdvance off must stay on level 2")
	}
	p := e.panel(t, 2)
	if p["passed"] != true || p["closed"].(float64) != 1 || p["required"].(float64) != 1 {
		t.Fatalf("level 2 panel: %+v", p)
	}
}

func TestPenaltyHintProtocol(t *testing.T) {
	e := newTestEnv(t)
	e.goto_(t, 2)
	pid := penaltyHelpID(2, 0)
	_, body := e.get(t, fixturePlay+"?pact=2&pid="+strconv.Itoa(pid))
	if !strings.Contains(body, "начислено <span>1 минуту<span> штрафного времени.&nbsp;<a href=\""+fixturePlay+"?pact=1&amp;pid="+strconv.Itoa(pid)+"\">Согласен</a>") || strings.Contains(body, "penalty text") {
		t.Fatalf("pact=2 must show confirmation only")
	}
	// Футер несёт query текущего запроса, lang — вторым параметром.
	if !strings.Contains(body, `<a href="`+fixturePlay+`?pact=2&amp;lang=en&amp;pid=`+strconv.Itoa(pid)+`" `) {
		t.Fatalf("footer lang link must carry the query")
	}
	_, body = e.get(t, fixturePlay)
	if !strings.Contains(body, "sure?&nbsp;<a href=") {
		t.Fatalf("confirmation must not persist")
	}
	_, body = e.get(t, fixturePlay+"?pact=1&pid="+strconv.Itoa(pid))
	if !strings.Contains(body, "\t\t<h3 class=\"inline\">Штрафная подсказка 1</h3>\r\n\t\t\t<p>penalty text</p>") {
		t.Fatalf("pact=1 must open the hint")
	}
}

func TestToggleCodeAndTime(t *testing.T) {
	e := newTestEnv(t)
	e.goto_(t, 3)
	e.action(t, 3, map[string]any{"action": "autoAdvance", "on": false})

	out := e.action(t, 3, map[string]any{"action": "toggleCode", "key": "s:0", "on": true})
	if out["redirect"] != fixturePlay {
		t.Fatalf("redirect: %v", out["redirect"])
	}
	_, body := e.get(t, fixturePlay)
	if !strings.Contains(body, "combo help") {
		t.Fatalf("sector-bonus toggle must answer the bonus")
	}
	e.action(t, 3, map[string]any{"action": "toggleCode", "key": "s:0", "on": false})
	_, body = e.get(t, fixturePlay)
	if strings.Contains(body, "combo help") {
		t.Fatalf("toggle off must revert the bonus")
	}
	e.action(t, 3, map[string]any{"action": "toggleCode", "key": "b:2:2", "on": true})
	_, body = e.get(t, fixturePlay)
	if !strings.Contains(body, "multi help") {
		t.Fatalf("multi-level bonus must be answerable from level 3")
	}

	// Время: пауза, ползунок, автопереход по таймауту и его откат.
	e.goto_(t, 2)
	e.action(t, 2, map[string]any{"action": "pause", "on": true})
	e.clock.Advance(100 * time.Second)
	if p := e.panel(t, 2); p["elapsed"].(float64) != 0 {
		t.Fatalf("paused elapsed = %v", p["elapsed"])
	}
	e.action(t, 2, map[string]any{"action": "setElapsed", "seconds": 599})
	e.action(t, 2, map[string]any{"action": "pause", "on": false})
	e.clock.Advance(2 * time.Second)
	if p := e.panel(t, 2); p["passed"] != true || p["passedBy"] != "timeout" {
		t.Fatalf("autopass expected: %+v", p)
	}
	e.action(t, 2, map[string]any{"action": "setElapsed", "seconds": 10})
	if p := e.panel(t, 2); p["passed"] != false {
		t.Fatalf("moving time back must undo timeout pass")
	}
}

func TestAssetsAndSnapshot(t *testing.T) {
	e := newTestEnv(t)
	code, body := e.get(t, "/assets/design.css")
	if code != 200 || !strings.Contains(body, "color: red") {
		t.Fatalf("asset: %d %q", code, body)
	}
	if code, _ := e.get(t, "/assets/..%2fgame.yml"); code == 200 {
		t.Fatalf("traversal must fail")
	}
	if code, _ := e.get(t, "/assets/.manifest.json"); code == 200 {
		t.Fatalf("hidden files must not be served")
	}
	for _, p := range []string{"/engine/css/v2/en/engines/engine.css?ver=1.88.0.0", "/engine/js/v2/jQuery/ui/jquery-ui-1.7.2.core.js", "/engine/js/v2/Timer.js", "/engine/toastr/toastr.min.js", "/emu/panel.js", "/emu/capture.js"} {
		if code, _ := e.get(t, p); code != 200 {
			t.Errorf("%s: %d", p, code)
		}
	}

	req, _ := http.NewRequest(http.MethodOptions, e.ts.URL+"/api/snapshot", nil)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatalf("CORS preflight: %d %v", resp.StatusCode, resp.Header)
	}
	payload, _ := json.Marshal(map[string]string{"name": "L06-before", "html": "<html>x</html>", "json": `{"a":1}`})
	resp, err = e.client.Post(e.ts.URL+"/api/snapshot", "text/plain", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("snapshot: %d", resp.StatusCode)
	}
	for _, f := range []string{"L06-before.html", "L06-before.json"} {
		if _, err := os.Stat(filepath.Join(e.data, "snapshots", f)); err != nil {
			t.Errorf("snapshot file %s: %v", f, err)
		}
	}
	if code, body := e.get(t, "/snapshots/"); code != 200 || !strings.Contains(body, "L06-before.html") {
		t.Fatalf("snapshot list: %d", code)
	}
	if code, body := e.get(t, "/api/mtime"); code != 200 || !strings.Contains(body, `"mtime":`) {
		t.Fatalf("mtime: %d %s", code, body)
	}
}
