package emu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"zapolnyaka/encx"
	"zapolnyaka/internal/assets"
	"zapolnyaka/internal/config"
)

// Options — настройки сервера эмулятора.
type Options struct {
	GamePath string           // путь к game.yml
	Addr     string           // адрес прослушивания, по умолчанию 127.0.0.1:8090
	DataDir  string           // куда писать .emu-state.json и snapshots/ (по умолчанию папка game.yml)
	Dev      bool             // шаблоны и статику читать с диска на каждый запрос
	DevDir   string           // корень internal/emu на диске для Dev (по умолчанию — из исходников)
	Offline  bool             // CSS/JS движка из встроенной копии (/engine/…) вместо world.en.cx
	Login    string           // логин игрока в истории ответов
	Now      func() time.Time // источник времени (тесты)
	Logf     func(format string, args ...any)
}

// Server — HTTP-сервер эмулятора. Play-страница живёт по тому же пути, что и в
// движке: /gameengines/encounter/play/{gid}/ (форма POST'ит на него же, таймеры
// перезагружают его же). /play/{n} — dev-переход на уровень n.
type Server struct {
	opts  Options
	store *Store
	mux   *http.ServeMux
	tmpl  *templates
	gid   int
	env   Env
}

// templates — play.html рендерится text/template (html/template вырезает HTML-комментарии
// движка и переэкранирует значения; все подстановки уже HTML-кодированы), остальные
// служебные страницы — html/template.
type templates struct {
	play  *ttemplate.Template
	other *template.Template
}

// New создаёт сервер: загружает состояние, парсит шаблоны, настраивает маршруты.
func New(opts Options) (*Server, error) {
	if opts.GamePath == "" {
		return nil, errors.New("emu: не задан путь к game.yml")
	}
	abs, err := filepath.Abs(opts.GamePath)
	if err != nil {
		return nil, err
	}
	opts.GamePath = abs
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:8090"
	}
	if opts.DataDir == "" {
		opts.DataDir = filepath.Dir(abs)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...any) {}
	}
	if opts.Dev && opts.DevDir == "" {
		if _, file, _, ok := runtime.Caller(0); ok {
			opts.DevDir = filepath.Dir(file)
		}
	}
	conf, err := config.LoadGame(opts.GamePath)
	if err != nil {
		return nil, err
	}

	store, err := NewStore(filepath.Join(opts.DataDir, ".emu-state.json"), opts.Now)
	if err != nil {
		return nil, err
	}
	env := DefaultEnv()
	if opts.Offline {
		env = OfflineEnv()
	}
	if opts.Login != "" {
		env.Login = opts.Login
	}
	s := &Server{opts: opts, store: store, gid: conf.GameID, env: env}
	if !opts.Dev {
		t, err := parseTemplates(embedded)
		if err != nil {
			return nil, err
		}
		s.tmpl = t
	}
	s.routes()
	return s, nil
}

func parseTemplates(fsys fs.FS) (*templates, error) {
	play, err := ttemplate.New("play.html").ParseFS(fsys, "templates/play.html")
	if err != nil {
		return nil, err
	}
	other, err := template.New("").ParseFS(fsys, "templates/error.html", "templates/finished.html", "templates/snapshots.html")
	if err != nil {
		return nil, err
	}
	return &templates{play: play, other: other}, nil
}

// templates возвращает шаблоны (в Dev — перечитывает с диска).
func (s *Server) templates() (*templates, error) {
	if !s.opts.Dev {
		return s.tmpl, nil
	}
	return parseTemplates(os.DirFS(s.opts.DevDir))
}

// static — файловая система статики (в Dev — с диска).
func (s *Server) static() fs.FS {
	if s.opts.Dev {
		if sub, err := fs.Sub(os.DirFS(s.opts.DevDir), "static"); err == nil {
			return sub
		}
	}
	sub, _ := fs.Sub(embedded, "static")
	return sub
}

// Handler — корневой обработчик (для httptest и ListenAndServe).
func (s *Server) Handler() http.Handler { return s.mux }

// Addr — адрес сервера.
func (s *Server) Addr() string { return s.opts.Addr }

// PlayPath — канонический путь play-страницы.
func (s *Server) PlayPath() string { return PlayPath(s.gid) }

// URL — адрес play-страницы для открытия в браузере.
func (s *Server) URL() string { return "http://" + s.opts.Addr + s.PlayPath() }

// ListenAndServe слушает адрес до отмены ctx.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("emu: порт занят или недоступен (%s): %w", s.opts.Addr, err)
	}
	srv := &http.Server{Handler: s.logged(s.mux)}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) logged(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.opts.Logf("emu %s %s", r.Method, r.URL.RequestURI())
		h.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	m := http.NewServeMux()
	play := s.PlayPath() // …/play/{gid}/
	m.HandleFunc("GET /{$}", s.handleIndex)
	m.HandleFunc("GET "+play+"{$}", s.handlePlay)
	m.HandleFunc("POST "+play+"{$}", s.handleAnswer)
	m.HandleFunc("GET "+strings.TrimSuffix(play, "/"), func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, play, http.StatusFound) // движок тоже редиректит на слэш
	})
	m.HandleFunc("GET /play/{n}", s.handleGoto)
	m.HandleFunc("GET /finished", s.handleFinished)
	m.HandleFunc("GET /assets/{name}", s.handleAsset)
	m.HandleFunc("GET /engine/", s.handleEngine)
	m.HandleFunc("GET /emu/{file}", s.handleEmuStatic)
	m.HandleFunc("GET /api/level/{n}", s.handleAPILevel)
	m.HandleFunc("POST /api/state/{n}", s.handleAPIState)
	m.HandleFunc("GET /api/mtime", s.handleMtime)
	m.HandleFunc("OPTIONS /api/snapshot", s.handleSnapshotOptions)
	m.HandleFunc("POST /api/snapshot", s.handleSnapshot)
	m.HandleFunc("GET /snapshots/{$}", s.handleSnapshotList)
	m.HandleFunc("GET /snapshots/{file}", s.handleSnapshotFile)
	s.mux = m
}

// ---------------------------------------------------------------- загрузка игры

// loaded — игра и переписыватель контента, перечитываемые на каждый запрос
// (это и есть live reload: правки конфигов/ассетов видны сразу).
type loaded struct {
	game *Game
	rw   *Rewriter
}

func (s *Server) load() (*loaded, error) {
	conf, prepared, err := config.LoadAll(s.opts.GamePath)
	if err != nil {
		return nil, err
	}
	dir := assets.DirFor(s.opts.GamePath, conf)
	manifest, err := assets.LoadManifest(assets.ManifestPath(dir))
	if err != nil {
		return nil, err
	}
	return &loaded{game: NewGame(conf, prepared), rw: NewRewriter(dir, manifest)}, nil
}

func (s *Server) levelNum(r *http.Request) (int, error) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("плохой номер уровня %q", r.PathValue("n"))
	}
	return n, nil
}

// current — текущий уровень симуляции (первый уровень конфига, если не задан).
func (s *Server) current(l *loaded) int {
	n := 0
	s.store.Read(func(gs *GameState, _ time.Time) { n = gs.Current })
	if _, ok := l.game.Level(n); !ok {
		n = l.game.First()
	}
	return n
}

// ---------------------------------------------------------------- страницы

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.PlayPath(), http.StatusFound)
}

// handleGoto — dev-переход на уровень n: делает его текущим и ведёт на play-страницу.
func (s *Server) handleGoto(w http.ResponseWriter, r *http.Request) {
	n, err := s.levelNum(r)
	if err != nil {
		s.errorPage(w, err)
		return
	}
	l, err := s.load()
	if err != nil {
		s.errorPage(w, err)
		return
	}
	if _, ok := l.game.Level(n); !ok {
		s.notFoundLevel(w, l, n)
		return
	}
	_ = s.store.Update(func(gs *GameState, now time.Time) error {
		gs.Current = n
		gs.Level(n, now)
		return nil
	})
	http.Redirect(w, r, s.PlayPath(), http.StatusSeeOther)
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	l, err := s.load()
	if err != nil {
		s.errorPage(w, err)
		return
	}
	n := s.current(l)
	if n == 0 {
		s.errorPage(w, fmt.Errorf("в конфиге %s нет уровней", s.opts.GamePath))
		return
	}
	q := r.URL.Query()
	req := Request{RawQuery: r.URL.RawQuery}

	// Штрафная подсказка: ?pact=2&pid= — запрос подтверждения (страница с «Согласен»),
	// ?pact=1&pid= — взять. Как в движке, обе отдаются сразу, без редиректа.
	if pid := q.Get("pid"); pid != "" {
		id, _ := strconv.Atoi(pid)
		pact, _ := strconv.Atoi(q.Get("pact"))
		idx, err := s.penaltyAction(l, n, id, pact)
		if err != nil {
			s.errorPage(w, err)
			return
		}
		if pact == 2 {
			req.ConfirmIndex = idx + 1
		}
	}

	var v *View
	redirect := ""
	err = s.store.Update(func(gs *GameState, now time.Time) error {
		gs.Current = n
		v, err = BuildView(l.game, n, gs, now, l.rw, s.env, req)
		if err != nil {
			return err
		}
		ls := gs.Level(n, now)
		if ls.Passed && gs.AutoAdvance {
			redirect = s.advance(l.game, gs, n, now)
		}
		return nil
	})
	if err != nil {
		s.errorPage(w, err)
		return
	}
	if redirect != "" {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if q.Get("json") == "1" {
		s.writeModelJSON(w, l, v)
		return
	}
	s.render(w, "play.html", v)
}

// writeModelJSON — ответ ?json=1 в формате GameModel движка.
func (s *Server) writeModelJSON(w http.ResponseWriter, l *loaded, v *View) {
	model := encx.GameModel{
		GameId: l.game.Conf.GameID, GameTitle: l.game.Title(), Login: s.env.Login,
		Levels: []encx.LevelSummary{}, Level: v.Model,
		EngineAction: &encx.EngineAction{GameId: l.game.Conf.GameID, LevelAction: &encx.ActionResult{}, BonusAction: &encx.ActionResult{}, PenaltyAction: &encx.PenaltyActionResult{}},
	}
	writeJSON(w, http.StatusOK, model)
}

// advance переводит симуляцию на следующий уровень (или на финиш) и возвращает URL.
func (s *Server) advance(g *Game, gs *GameState, n int, now time.Time) string {
	next := g.Next(n)
	if next == 0 {
		return "/finished"
	}
	gs.Current = next
	gs.Level(next, now) // старт таймера следующего уровня
	return s.PlayPath()
}

// penaltyAction обрабатывает ?pid=&pact=: pact=1 открывает подсказку, pact=2 ничего
// не меняет (страница подтверждения). Возвращает индекс подсказки.
func (s *Server) penaltyAction(l *loaded, n, id, pact int) (int, error) {
	p, _ := l.game.Level(n)
	idx, ok := penaltyIndexFromID(n, id)
	if !ok || idx >= len(p.Conf.PenaltyHints) {
		return 0, fmt.Errorf("штрафная подсказка %d не найдена", id)
	}
	h := p.Conf.PenaltyHints[idx]
	if pact != 1 {
		return idx, nil
	}
	return idx, s.store.Update(func(gs *GameState, now time.Time) error {
		ls := gs.Level(n, now)
		if int(ls.Elapsed(now).Seconds()) < h.Time {
			return nil // ещё недоступна
		}
		ls.OpenedPenalty[idx] = 2
		return nil
	})
}

// handleAnswer — POST формы ответа. Как движок, отвечает страницей сразу (без
// редиректа): уведомление «верный/неверный», неверный ответ остаётся в поле, а
// при прохождении уровня показывается следующий уровень с записью «(N)».
func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	l, err := s.load()
	if err != nil {
		s.errorPage(w, err)
		return
	}
	n := s.current(l)
	if n == 0 {
		s.errorPage(w, fmt.Errorf("в конфиге %s нет уровней", s.opts.GamePath))
		return
	}
	if err := r.ParseForm(); err != nil {
		s.errorPage(w, err)
		return
	}
	answer := r.PostForm.Get("LevelAction.Answer")
	if b := r.PostForm.Get("BonusAction.Answer"); b != "" {
		answer = b
	}

	var v *View
	err = s.store.Update(func(gs *GameState, now time.Time) error {
		ls := gs.Level(n, now)
		req := Request{JustNow: map[int]bool{}}
		if strings.TrimSpace(answer) != "" {
			m, added := s.applyAnswer(l.game, gs, ls, n, answer, s.env.Login, now)
			for _, a := range added {
				req.JustNow[a.ActionId] = true
			}
			if m.Correct() {
				req.Notice, req.NoticeCorrect = "Ответ или код верный", true
			} else {
				req.Notice, req.AnswerValue = "Ответ или код неверный", answer
			}
			if ls.Passed && gs.AutoAdvance {
				s.advance(l.game, gs, n, now)
				if gs.Current != n {
					req.Carry = added // записи прошлого уровня на странице нового
					n = gs.Current
				}
			}
		}
		v, err = BuildView(l.game, n, gs, now, l.rw, s.env, req)
		return err
	})
	if err != nil {
		s.errorPage(w, err)
		return
	}
	s.render(w, "play.html", v)
}

// applyAnswer проверяет ответ по секторам и бонусам и обновляет состояние и историю.
// Возвращает результат и добавленные записи истории.
func (s *Server) applyAnswer(g *Game, gs *GameState, ls *LevelState, n int, answer, login string, now time.Time) (MatchResult, []encx.CodeAction) {
	p, _ := g.Level(n)
	checkAutopass(p, ls, now)
	sectors := SectorCodes(p.Codes)
	bonuses := config.BonusesForLevel(g.Prepared, n)
	m := Match(answer, sectors, bonuses, ls, gs)
	ls.Attempts++
	entry := Entry{Answer: strings.TrimSpace(answer), Login: login, At: now, Level: n}
	var added []encx.CodeAction
	add := func(a encx.CodeAction) {
		ls.History = append(ls.History, a)
		added = append(added, a)
	}

	// Порядок как у движка: сначала записи бонусов, потом секторов (в истории
	// новые сверху, поэтому запись сектора оказывается над записью бонуса).
	for _, key := range m.Bonuses {
		gs.Bonuses[key] = entry
		for _, b := range bonuses {
			if BonusKey(b.OwnerLevel, b.Index) == key {
				add(s.action(ls, n, 2, login, entry.Answer, true, b.Code.Type.IsPenalty(), now))
			}
		}
	}
	for _, i := range m.Sectors {
		ls.Sectors[i] = entry
		add(s.action(ls, n, 1, login, entry.Answer, true, false, now))
	}
	if !m.Correct() {
		add(s.action(ls, n, 1, login, entry.Answer, false, false, now))
	} else if len(m.Sectors)+len(m.Bonuses) == 0 {
		// код подошёл к уже закрытому — в историю как верный, без изменений
		add(s.action(ls, n, 1, login, entry.Answer, true, false, now))
	}

	if !ls.Passed && len(sectors) > 0 && len(ls.Sectors) >= requiredSectors(p, len(sectors)) {
		ls.Passed, ls.PassedBy = true, "codes"
	}
	return m, added
}

var actionSeq = 0

func (s *Server) action(ls *LevelState, n, kind int, login, answer string, correct, negative bool, now time.Time) encx.CodeAction {
	actionSeq++
	return encx.CodeAction{
		ActionId:    n*100000 + len(ls.History) + 1,
		LevelId:     levelID(n),
		LevelNumber: n,
		Kind:        kind,
		Login:       login,
		Answer:      answer,
		LocDateTime: now.Format("15:04:05"),
		IsCorrect:   correct,
		Negative:    negative,
	}
}

func (s *Server) handleFinished(w http.ResponseWriter, r *http.Request) {
	l, err := s.load()
	if err != nil {
		s.errorPage(w, err)
		return
	}
	s.render(w, "finished.html", map[string]any{
		"GameID": l.game.Conf.GameID,
		"Levels": l.game.Numbers(),
		"First":  l.game.First(),
	})
}

func (s *Server) notFoundLevel(w http.ResponseWriter, l *loaded, n int) {
	w.WriteHeader(http.StatusNotFound)
	s.render(w, "error.html", map[string]any{
		"Title":  fmt.Sprintf("Уровня %d нет в конфиге", n),
		"Error":  fmt.Sprintf("В %s есть уровни: %v", s.opts.GamePath, l.game.Numbers()),
		"Levels": l.game.Numbers(),
	})
}

func (s *Server) errorPage(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	s.render(w, "error.html", map[string]any{
		"Title": "Ошибка эмулятора",
		"Error": err.Error(),
	})
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	t, err := s.templates()
	if err != nil {
		http.Error(w, "template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if name == "play.html" {
		err = t.play.ExecuteTemplate(w, name, data)
	} else {
		err = t.other.ExecuteTemplate(w, name, data)
	}
	if err != nil {
		s.opts.Logf("emu render %s: %v", name, err)
		fmt.Fprintf(w, "<pre>render %s: %s</pre>", name, template.HTMLEscapeString(err.Error()))
	}
}

// ---------------------------------------------------------------- статика

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\`) {
		http.NotFound(w, r)
		return
	}
	l, err := s.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	disk, ok := l.rw.DiskName(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, filepath.Join(l.rw.assetsDir, disk))
}

// handleEngine отдаёт встроенные файлы движка по тем же путям, что world.en.cx
// (/engine/css/v2/en/engines/engine.css, /engine/js/v2/Timer.js, …). Папка
// jQuery/ui на диске лежит как jquery/ui (Windows не различает регистр).
func (s *Server) handleEngine(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(s.static(), "engine")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/engine/")
	p = strings.Replace(p, "js/v2/jQuery/", "js/v2/jquery/", 1)
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + p
	r2.URL.RawQuery = ""
	http.FileServerFS(sub).ServeHTTP(w, r2)
}

func (s *Server) handleEmuStatic(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if strings.Contains(file, "/") || strings.HasPrefix(file, ".") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFileFS(w, r, s.static(), file)
}

// ---------------------------------------------------------------- API панели

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleAPILevel(w http.ResponseWriter, r *http.Request) {
	l, err := s.load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	n := 0
	if r.PathValue("n") == "current" {
		n = s.current(l)
	} else if n, err = s.levelNum(r); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var v *View
	err = s.store.Update(func(gs *GameState, now time.Time) error {
		v, err = BuildView(l.game, n, gs, now, l.rw, s.env, Request{})
		return err
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel": v.Panel(), "model": v.Model})
}

// stateAction — команда панели.
type stateAction struct {
	Action  string `json:"action"`
	Key     string `json:"key,omitempty"`     // toggleCode
	On      *bool  `json:"on,omitempty"`      // toggleCode / autoAdvance / pause
	Seconds int    `json:"seconds,omitempty"` // setElapsed
	Index   int    `json:"index"`             // openHint / closeHint
	Level   int    `json:"level,omitempty"`   // goto
}

func (s *Server) handleAPIState(w http.ResponseWriter, r *http.Request) {
	n, err := s.levelNum(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	l, err := s.load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	p, ok := l.game.Level(n)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("уровня %d нет в конфиге", n)})
		return
	}
	var a stateAction
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json: " + err.Error()})
		return
	}

	redirect := s.PlayPath()
	err = s.store.Update(func(gs *GameState, now time.Time) error {
		ls := gs.Level(n, now)
		switch a.Action {
		case "toggleCode":
			on := a.On == nil || *a.On
			if err := s.toggleCode(l.game, gs, ls, p, n, a.Key, on, now); err != nil {
				return err
			}
			if on && ls.Passed && gs.AutoAdvance {
				redirect = s.advance(l.game, gs, n, now)
			}
		case "setElapsed":
			ls.SetElapsed(time.Duration(a.Seconds)*time.Second, now)
			if ls.PassedBy == "timeout" {
				ls.Passed, ls.PassedBy = false, "" // откат времени снимает автопереход
			}
		case "pause":
			if a.On != nil && !*a.On {
				ls.Resume(now)
			} else {
				ls.Pause(now)
			}
		case "autoAdvance":
			gs.AutoAdvance = a.On == nil || *a.On
		case "openHint":
			if a.Index < 0 || a.Index >= len(p.Conf.PenaltyHints) {
				return fmt.Errorf("нет штрафной подсказки %d", a.Index)
			}
			ls.OpenedPenalty[a.Index] = 2
		case "closeHint":
			delete(ls.OpenedPenalty, a.Index)
		case "reset":
			gs.ResetLevel(n, now)
		case "resetAll":
			gs.ResetAll()
			gs.Current = l.game.First()
		case "goto":
			if _, ok := l.game.Level(a.Level); !ok {
				return fmt.Errorf("уровня %d нет в конфиге", a.Level)
			}
			gs.Current = a.Level
			gs.Level(a.Level, now)
		default:
			return fmt.Errorf("неизвестное действие %q", a.Action)
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": redirect})
}

// toggleCode вводит (on) или откатывает (off) запись codes по ключу панели:
// "s:<idx сектора>" или "b:<owner>:<idx записи>".
func (s *Server) toggleCode(g *Game, gs *GameState, ls *LevelState, p *config.PreparedLevel, n int, key string, on bool, now time.Time) error {
	var code *config.Code
	sectorIdx := -1
	bonusKey := ""
	parts := strings.Split(key, ":")
	switch {
	case len(parts) == 2 && parts[0] == "s":
		i, _ := strconv.Atoi(parts[1])
		sectors := SectorCodes(p.Codes)
		if i < 0 || i >= len(sectors) {
			return fmt.Errorf("нет сектора %d", i)
		}
		c := sectors[i]
		code, sectorIdx = &c, i
		if c.Type.HasBonus() {
			si := -1
			for ci, cc := range p.Codes {
				if cc.Type.HasSector() {
					si++
					if si == i {
						bonusKey = BonusKey(n, ci)
						break
					}
				}
			}
		}
	case len(parts) == 3 && parts[0] == "b":
		owner, _ := strconv.Atoi(parts[1])
		idx, _ := strconv.Atoi(parts[2])
		op, ok := g.Level(owner)
		if !ok || idx < 0 || idx >= len(op.Codes) {
			return fmt.Errorf("нет бонуса %s", key)
		}
		c := op.Codes[idx]
		code, bonusKey = &c, BonusKey(owner, idx)
	default:
		return fmt.Errorf("плохой ключ %q", key)
	}

	if on {
		if len(code.Answers) == 0 {
			return errors.New("у записи нет ответов")
		}
		s.applyAnswer(g, gs, ls, n, code.Answers[0], s.env.Login, now)
		return nil
	}

	// Откат: убрать ввод и записи истории с этими ответами.
	if sectorIdx >= 0 {
		delete(ls.Sectors, sectorIdx)
	}
	if bonusKey != "" {
		delete(gs.Bonuses, bonusKey)
	}
	kept := ls.History[:0]
	for _, h := range ls.History {
		if AnswerMatches(h.Answer, code.Answers) {
			continue
		}
		kept = append(kept, h)
	}
	ls.History = kept
	if ls.PassedBy == "codes" && len(ls.Sectors) < requiredSectors(p, len(SectorCodes(p.Codes))) {
		ls.Passed, ls.PassedBy = false, ""
	}
	return nil
}

// handleMtime — максимальное время изменения конфигов и ассетов (для live reload).
func (s *Server) handleMtime(w http.ResponseWriter, r *http.Request) {
	var latest int64
	touch := func(path string) {
		if fi, err := os.Stat(path); err == nil && fi.ModTime().UnixNano() > latest {
			latest = fi.ModTime().UnixNano()
		}
	}
	touch(s.opts.GamePath)
	if conf, err := config.LoadGame(s.opts.GamePath); err == nil {
		dir := filepath.Dir(s.opts.GamePath)
		for _, rel := range conf.Levels {
			confPath := filepath.Join(dir, rel)
			touch(confPath)
			if lvl, err := config.LoadLevel(confPath); err == nil {
				if lvl.Codes != nil {
					touch(filepath.Join(filepath.Dir(confPath), *lvl.Codes))
				}
				if lvl.Body != nil {
					touch(filepath.Join(filepath.Dir(confPath), *lvl.Body))
				}
			}
		}
		adir := assets.DirFor(s.opts.GamePath, conf)
		if entries, err := os.ReadDir(adir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					touch(filepath.Join(adir, e.Name()))
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"mtime": latest})
}

// ---------------------------------------------------------------- снимки реальной страницы

var snapshotNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

func (s *Server) snapshotsDir() string { return filepath.Join(s.opts.DataDir, "snapshots") }

func corsHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
	h.Set("Access-Control-Allow-Private-Network", "true")
	h.Set("Access-Control-Allow-Local-Network", "true")
	h.Set("Access-Control-Max-Age", "600")
}

func (s *Server) handleSnapshotOptions(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

// snapshotRequest — тело POST /api/snapshot от страницы en.cx (capture.js).
type snapshotRequest struct {
	Name string `json:"name"`
	HTML string `json:"html"`
	JSON string `json:"json"`
	URL  string `json:"url"`
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	var req snapshotRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json: " + err.Error()})
		return
	}
	if !snapshotNameRe.MatchString(req.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "плохое имя снимка (буквы, цифры, _ . -)"})
		return
	}
	if err := os.MkdirAll(s.snapshotsDir(), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	var files []string
	if req.HTML != "" {
		p := filepath.Join(s.snapshotsDir(), req.Name+".html")
		if err := os.WriteFile(p, []byte(req.HTML), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		files = append(files, p)
	}
	if req.JSON != "" {
		p := filepath.Join(s.snapshotsDir(), req.Name+".json")
		if err := os.WriteFile(p, []byte(req.JSON), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		files = append(files, p)
	}
	s.opts.Logf("emu snapshot %s (%d html, %d json) from %s", req.Name, len(req.HTML), len(req.JSON), req.URL)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "files": files})
}

func (s *Server) handleSnapshotList(w http.ResponseWriter, r *http.Request) {
	entries, _ := os.ReadDir(s.snapshotsDir())
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	s.render(w, "snapshots.html", map[string]any{"Dir": s.snapshotsDir(), "Files": names})
}

func (s *Server) handleSnapshotFile(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if !snapshotNameRe.MatchString(file) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, filepath.Join(s.snapshotsDir(), file))
}
