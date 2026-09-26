// Package ui — веб-интерфейс zapolnyaka: вкладки «Команды», «Коды», «Редактор»,
// «Превью», «Эмулятор» поверх эмулятора (internal/emu). Монтируется в его
// маршрутизатор под /ui/ и /api/ui/.
package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zapolnyaka/internal/config"
	"zapolnyaka/internal/emu"
)

//go:embed static
var static embed.FS

// Actions — команды приложения, которые UI запускает как задания. Реализуются в
// пакете cmd (ui не может импортировать cmd: cmd импортирует ui).
type Actions interface {
	ScanGames() []string
	Login() string
	Go(gamePath string, levels []int, log io.Writer) error
	Assets(gamePath string, log io.Writer) error
	Validate(gamePath string, log io.Writer) error
	Check(gamePath string, log io.Writer) error
	Snapshot(gamePath, name, send string, pid, pact int, log io.Writer) error
	Auth(login, password string) error
	NewGame(name, domain string, gameID int) (string, error)
	NewLevel(gamePath, dir string, num int) error
	SaveLastGame(gamePath string)
}

// Deps — зависимости веб-интерфейса.
type Deps struct {
	Emu     *emu.Server
	Actions Actions
	Version string
	Dev     bool   // статику читать с диска
	DevDir  string // папка internal/ui
}

// UI — обработчики веб-интерфейса.
type UI struct {
	deps Deps
	jobs *Jobs
}

// Mount регистрирует маршруты UI в mux.
func Mount(mux *http.ServeMux, deps Deps) *UI {
	u := &UI{deps: deps, jobs: NewJobs()}
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ui/", http.StatusFound) })
	mux.HandleFunc("GET /ui/{$}", u.handleIndex)
	mux.HandleFunc("GET /ui/static/{file}", u.handleStatic)
	mux.HandleFunc("GET /ui/preview/{n}", u.handlePreview)
	mux.HandleFunc("GET /api/ui/state", u.handleState)
	mux.HandleFunc("POST /api/ui/game", u.handleSelectGame)
	mux.HandleFunc("GET /api/ui/level/{n}", u.handleLevel)
	mux.HandleFunc("PUT /api/ui/level/{n}/codes", u.handleSaveCodes)
	mux.HandleFunc("PUT /api/ui/level/{n}/conf", u.handleSaveConf)
	mux.HandleFunc("PUT /api/ui/level/{n}/raw/{which}", u.handleSaveRaw)
	mux.HandleFunc("GET /api/ui/preview/{n}/checks", u.handlePreviewChecks)
	mux.HandleFunc("POST /api/ui/run", u.handleRun)
	mux.HandleFunc("GET /api/ui/jobs/{id}", u.handleJob)
	mux.HandleFunc("POST /api/ui/auth", u.handleAuth)
	mux.HandleFunc("POST /api/ui/game/new", u.handleNewGame)
	mux.HandleFunc("POST /api/ui/level/new", u.handleNewLevel)
	mux.HandleFunc("GET /api/ui/snapshots", u.handleSnapshots)
	mux.HandleFunc("GET /api/ui/diff/{name}", u.handleDiff)
	return u
}

func (u *UI) staticFS() fs.FS {
	if u.deps.Dev && u.deps.DevDir != "" {
		return os.DirFS(filepath.Join(u.deps.DevDir, "static"))
	}
	sub, _ := fs.Sub(static, "static")
	return sub
}

func (u *UI) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFileFS(w, r, u.staticFS(), "index.html")
}

func (u *UI) handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if strings.Contains(file, "/") || strings.HasPrefix(file, ".") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFileFS(w, r, u.staticFS(), file)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (u *UI) levelNum(r *http.Request) (int, error) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("плохой номер уровня %q", r.PathValue("n"))
	}
	return n, nil
}

// ---------------------------------------------------------------- состояние

type stateResponse struct {
	Games    []GameInfo  `json:"games"`
	Game     *GameInfo   `json:"game"`
	Levels   []LevelInfo `json:"levels"`
	Assets   []AssetInfo `json:"assets"`
	Login    string      `json:"login"`
	PlayPath string      `json:"playPath"`
	Addr     string      `json:"addr"`
	Version  string      `json:"version"`
	Job      *jobStatus  `json:"job,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type jobStatus struct {
	ID       int    `json:"id"`
	Cmd      string `json:"cmd"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
	Finished string `json:"finished,omitempty"`
}

func (u *UI) handleState(w http.ResponseWriter, r *http.Request) {
	cur := u.deps.Emu.GamePath()
	resp := stateResponse{Login: u.deps.Actions.Login(), PlayPath: u.deps.Emu.PlayPath(), Addr: u.deps.Emu.Addr(), Version: u.deps.Version}
	for _, p := range u.deps.Actions.ScanGames() {
		abs, _ := filepath.Abs(p)
		info := GameInfo{Path: filepath.ToSlash(p), Current: abs == cur}
		if g, err := config.LoadGame(p); err == nil {
			info.Title, info.Domain, info.GameID, info.Levels = g.Title, g.Domain, g.GameID, len(g.Levels)
		}
		resp.Games = append(resp.Games, info)
	}
	game, levels, err := listLevels(cur)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Game = &GameInfo{Path: filepath.ToSlash(cur), Title: game.Title, Domain: game.Domain, GameID: game.GameID, Levels: len(game.Levels), Current: true}
		resp.Levels = levels
		resp.Assets = listAssets(cur, game)
	}
	if resp.Levels == nil {
		resp.Levels = []LevelInfo{}
	}
	if resp.Assets == nil {
		resp.Assets = []AssetInfo{}
	}
	if j := u.jobs.Last(); j != nil {
		resp.Job = jobStatusOf(j)
	}
	writeJSON(w, http.StatusOK, resp)
}

func jobStatusOf(j *Job) *jobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	st := &jobStatus{ID: j.ID, Cmd: j.Cmd, Title: j.Title, Done: j.Done, Error: j.Error}
	if j.Done {
		st.Finished = j.Finished.Format("15:04")
	}
	return st
}

func (u *UI) handleSelectGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нужен path"))
		return
	}
	if err := u.deps.Emu.SetGame(req.Path); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	u.deps.Actions.SaveLastGame(req.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "playPath": u.deps.Emu.PlayPath()})
}

// ---------------------------------------------------------------- уровень: чтение/запись

func (u *UI) handleLevel(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	d, err := loadLevel(u.deps.Emu.GamePath(), n)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (u *UI) handleSaveCodes(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var codes []config.Code
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&codes); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("bad json: %w", err))
		return
	}
	if err := saveCodes(u.deps.Emu.GamePath(), n, codes); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (u *UI) handleSaveConf(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var conf config.Level
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&conf); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("bad json: %w", err))
		return
	}
	if err := saveConf(u.deps.Emu.GamePath(), n, &conf); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (u *UI) handleSaveRaw(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := saveRaw(u.deps.Emu.GamePath(), n, r.PathValue("which"), string(body)); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------- превью

func previewOptions(q map[string][]string) emu.PreviewOptions {
	o := emu.DefaultPreview()
	get := func(k string, def bool) bool {
		v, ok := q[k]
		if !ok || len(v) == 0 {
			return def
		}
		return v[0] == "1" || v[0] == "true"
	}
	o.Task, o.Sectors, o.Hints, o.Penalties, o.Bonuses, o.Fog = get("task", true), get("sectors", true), get("hints", true), get("penalties", true), get("bonuses", true), get("fog", false)
	return o
}

func (u *UI) handlePreview(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	html, _, err := u.deps.Emu.Preview(n, previewOptions(r.URL.Query()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

func (u *UI) handlePreviewChecks(w http.ResponseWriter, r *http.Request) {
	n, err := u.levelNum(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	_, checks, err := u.deps.Emu.Preview(n, emu.DefaultPreview())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if checks == nil {
		checks = []emu.PreviewCheck{}
	}
	writeJSON(w, http.StatusOK, checks)
}

// ---------------------------------------------------------------- команды

type runRequest struct {
	Cmd    string `json:"cmd"`
	Levels []int  `json:"levels,omitempty"`
	Name   string `json:"name,omitempty"`
	Send   string `json:"send,omitempty"`
	PID    int    `json:"pid,omitempty"`
	Pact   int    `json:"pact,omitempty"`
}

func (u *UI) handleRun(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("bad json: %w", err))
		return
	}
	gamePath := u.deps.Emu.GamePath()
	a := u.deps.Actions
	var fn func(log *Job) error
	title := ""
	switch req.Cmd {
	case "go":
		title = "Залить уровни"
		if len(req.Levels) > 0 {
			title += fmt.Sprintf(" %v", req.Levels)
		}
		fn = func(j *Job) error { return a.Go(gamePath, req.Levels, j) }
	case "assets":
		title = "Залить ассеты"
		fn = func(j *Job) error { return a.Assets(gamePath, j) }
	case "validate":
		title = "Проверить конфиги"
		fn = func(j *Job) error { return a.Validate(gamePath, j) }
	case "check":
		title = "Проверить залитое"
		fn = func(j *Job) error { return a.Check(gamePath, j) }
	case "snapshot":
		title = "Снимок реальной страницы"
		if req.Name == "" {
			req.Name = "snapshot-" + time.Now().Format("150405")
		}
		fn = func(j *Job) error { return a.Snapshot(gamePath, req.Name, req.Send, req.PID, req.Pact, j) }
	default:
		writeErr(w, http.StatusBadRequest, fmt.Errorf("неизвестная команда %q", req.Cmd))
		return
	}
	job, err := u.jobs.Start(req.Cmd, title, func(j *Job) error {
		fmt.Fprintf(j, "▶ %s · %s\n", title, filepath.ToSlash(gamePath))
		return fn(j)
	})
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": job.ID})
}

func (u *UI) handleJob(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	j, ok := u.jobs.Get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("задание %d не найдено", id))
		return
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	lines, total := j.Lines(from)
	st := jobStatusOf(j)
	writeJSON(w, http.StatusOK, map[string]any{"job": st, "from": from, "total": total, "lines": lines})
}

func (u *UI) handleAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Login == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нужны логин и пароль"))
		return
	}
	if err := u.deps.Actions.Auth(strings.TrimSpace(req.Login), req.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "login": strings.TrimSpace(req.Login)})
}

func (u *UI) handleNewGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
		GameID int    `json:"gameId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Domain == "" || req.GameID == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нужны name, domain и gameId"))
		return
	}
	path, err := u.deps.Actions.NewGame(req.Name, req.Domain, req.GameID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": filepath.ToSlash(path)})
}

func (u *UI) handleNewLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir    string `json:"dir"`
		Number int    `json:"number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Dir == "" || req.Number <= 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нужны dir и number"))
		return
	}
	if strings.ContainsAny(req.Dir, `/\..`) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("папка — простое имя без слэшей"))
		return
	}
	if err := u.deps.Actions.NewLevel(u.deps.Emu.GamePath(), req.Dir, req.Number); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------- снимки и сверка

func (u *UI) snapshotsDir() string { return filepath.Join(gameDir(u.deps.Emu.GamePath()), "snapshots") }

func (u *UI) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	entries, _ := os.ReadDir(u.snapshotsDir())
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
			names = append(names, strings.TrimSuffix(e.Name(), ".html"))
		}
	}
	writeJSON(w, http.StatusOK, names)
}

// handleDiff сравнивает снимок реальной страницы с текущей страницей эмулятора.
func (u *UI) handleDiff(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("плохое имя снимка"))
		return
	}
	real, err := os.ReadFile(filepath.Join(u.snapshotsDir(), name+".html"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	req, _ := http.NewRequest(http.MethodGet, u.deps.Emu.PlayPath(), nil)
	rec := &responseRecorder{header: http.Header{}}
	u.deps.Emu.Handler().ServeHTTP(rec, req)
	if rec.status >= 300 && rec.status < 400 {
		writeErr(w, http.StatusConflict, fmt.Errorf("эмулятор переадресует на %s (уровень пройден?) — откройте нужный уровень в панели и повторите", rec.header.Get("Location")))
		return
	}
	writeJSON(w, http.StatusOK, emu.Diff(string(real), rec.body.String()))
}

// responseRecorder — минимальный ResponseWriter для внутреннего запроса к эмулятору.
type responseRecorder struct {
	header http.Header
	status int
	body   strings.Builder
}

func (r *responseRecorder) Header() http.Header         { return r.header }
func (r *responseRecorder) WriteHeader(status int)      { r.status = status }
func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }
