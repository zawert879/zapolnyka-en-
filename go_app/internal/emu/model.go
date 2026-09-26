package emu

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"zapolnyaka/encx"
	"zapolnyaka/internal/config"
)

// Идентификаторы сущностей — синтетические, но стабильные (нужны для pid= и API).
func levelID(n int) int          { return 1000000 + n }
func sectorID(n, i int) int      { return n*1000 + i + 1 }
func helpID(n, i int) int        { return n*1000 + 100 + i + 1 }
func penaltyHelpID(n, i int) int { return n*1000 + 200 + i + 1 }
func bonusID(owner, idx int) int { return owner*1000 + 500 + idx + 1 }
func penaltyIndexFromID(n, id int) (int, bool) {
	i := id - (n*1000 + 200 + 1)
	return i, i >= 0 && i < 100 && id > n*1000+200
}

// CodeRow — строка кода для dev-панели.
type CodeRow struct {
	Key         string   `json:"key"`  // сектор: "s:<idx>", бонус: "b:<owner>:<idx>"
	Kind        string   `json:"kind"` // "sector" | "bonus"
	Type        string   `json:"type"` // тип записи codes
	SectorName  string   `json:"sectorName,omitempty"`
	BonusName   string   `json:"bonusName,omitempty"`
	Answers     []string `json:"answers"`
	Entered     bool     `json:"entered"`
	OwnerLevel  int      `json:"ownerLevel,omitempty"`
	Foreign     bool     `json:"foreign,omitempty"` // бонус другого уровня (по levels)
	Time        int      `json:"time,omitempty"`
	Negative    bool     `json:"negative,omitempty"`
	SectorIndex int      `json:"sectorIndex"`
	BonusIndex  int      `json:"bonusIndex"`
}

// HintView — обычная подсказка.
type HintView struct {
	Number int
	HelpID int
	Text   template.HTML
	Time   int // секунда уровня, на которой появляется
	Remain int // секунд до появления (0 — показана)
	Shown  bool
}

// PenaltyView — штрафная подсказка.
type PenaltyView struct {
	Number  int
	HelpID  int
	Index   int
	Text    template.HTML
	Time    int
	Remain  int
	State   int // 0 — не открыта, 2 — открыта
	Penalty int // секунд штрафа
	Comment string
}

// BonusView — бонус.
type BonusView struct {
	Key        string
	Number     int
	BonusID    int
	Name       string
	Task       template.HTML
	Help       template.HTML
	Answered   bool
	Answer     string
	AnsweredAt time.Time
	Login      string
	Award      int // секунд
	Negative   bool
	Foreign    bool
}

// SectorView — сектор.
type SectorView struct {
	Index      int
	Order      int
	SectorID   int
	Name       string
	Answered   bool
	Answer     string
	AnsweredAt time.Time
	Login      string
	UserID     int
}

// HistoryView — запись истории ответов (новые сверху).
type HistoryView struct {
	Answer     string
	Correct    bool
	Kind       int // 1 — уровень/сектор, 2 — бонус
	Negative   bool
	Time       string
	Login      string
	UserID     int
	OtherLevel int  // номер уровня, если запись сделана на другом уровне (показывается «(N)»)
	JustNow    bool // запись сделана этим же запросом (неверный ответ подсвечивается)
}

// LevelNav — уровень в списке панели.
type LevelNav struct {
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Current bool   `json:"current"`
	Started bool   `json:"started"`
}

// Request — то, что зависит от конкретного запроса, а не от состояния:
// результат только что введённого кода, запрос подтверждения штрафной подсказки,
// перенесённые с прошлого уровня записи истории (ответ на POST, прошедший уровень).
type Request struct {
	AnswerValue   string            // значение в поле ответа (неверный ответ остаётся в поле)
	Notice        string            // «Ответ или код верный/неверный»
	NoticeCorrect bool
	JustNow       map[int]bool      // ActionId записей этого запроса
	Carry         []encx.CodeAction // записи прошлого уровня, показанные на странице нового
	ConfirmIndex  int               // 1-based индекс штрафной подсказки, ждущей подтверждения
	RawQuery      string            // query текущего запроса: движок протаскивает его в ссылки EN/RU футера
}

// langHref — ссылка переключения языка в футере: параметры текущего запроса в исходном
// порядке, lang вставляется вторым (после первого параметра), как делает движок:
// «?pid=X&pact=2» → «?pid=X&lang=en&pact=2», без query → «?lang=en».
func langHref(playPath, rawQuery, lang string) string {
	var parts []string
	for _, p := range strings.Split(rawQuery, "&") {
		if p == "" || strings.HasPrefix(p, "lang=") {
			continue
		}
		parts = append(parts, p)
	}
	at := 1
	if len(parts) < 1 {
		at = 0
	}
	parts = append(parts[:at], append([]string{"lang=" + lang}, parts[at:]...)...)
	return playPath + "?" + strings.ReplaceAll(strings.Join(parts, "&"), "&", "&amp;")
}

// Env — окружение рендера: откуда брать файлы движка, кто играет.
type Env struct {
	EngineBase string // https://world.en.cx или /engine (офлайн)
	ToastrBase string
	Login      string
	UserID     int
}

// DefaultEnv — как на реальной странице.
func DefaultEnv() Env {
	return Env{EngineBase: "https://world.en.cx", ToastrBase: "//cdnjs.cloudflare.com/ajax/libs/toastr.js/latest", Login: "player"}
}

// OfflineEnv — файлы движка из встроенной копии.
func OfflineEnv() Env {
	e := DefaultEnv()
	e.EngineBase, e.ToastrBase = "/engine", "/engine/toastr"
	return e
}

// View — всё, что нужно шаблону play.html и панели.
type View struct {
	// шапка/скелет
	EngineBase, ToastrBase string
	EngineVer              string
	GameID                 int
	GameTitle              string
	Topic                  int
	PlayPath               string
	Rnd                    string
	LangHrefEN, LangHrefRU string

	Number        int
	LevelID       int
	LevelName     string
	LevelNameHTML template.HTML
	LevelsTotal   int
	Prev, Next    int
	IsLast        bool

	AnswerAttr template.HTMLAttr

	TaskHTML template.HTML

	Elapsed     int
	ElapsedText string
	Paused      bool
	AutoAdvance bool
	Passed      bool
	PassedBy    string

	Timeout       int
	TimeoutRemain int
	TimeoutAward  int

	RequiredSectors int
	PassedSectors   int
	SectorsLeft     int
	PassedBonuses   int // бонусов, взятых на этом уровне
	Sectors         []SectorView
	Hints           []HintView
	Penalties       []PenaltyView
	Bonuses         []BonusView
	History         []HistoryView

	Notice        string
	NoticeCorrect bool
	ConfirmIndex  int

	// отрендеренные блоки (render.go)
	HistoryHTML, TimerHTML, SectorsHTML, TaskBlockHTML template.HTML
	HelpsHTML, PenaltiesHTML, BonusesHTML, PanelHTML    template.HTML

	Codes   []CodeRow
	Levels  []LevelNav
	Missing []string

	Model *encx.Level // модель в формате ?json=1
}

// panelData — то, что панель получает через /api/level/{n}.
type panelData struct {
	Level       int        `json:"level"`
	LevelName   string     `json:"levelName"`
	PlayPath    string     `json:"playPath"`
	Levels      []LevelNav `json:"levels"`
	Elapsed     int        `json:"elapsed"`
	Paused      bool       `json:"paused"`
	AutoAdvance bool       `json:"autoAdvance"`
	Passed      bool       `json:"passed"`
	PassedBy    string     `json:"passedBy,omitempty"`
	Timeout     int        `json:"timeout"`
	SliderMax   int        `json:"sliderMax"`
	Codes       []CodeRow  `json:"codes"`
	Penalties   []struct {
		Index   int    `json:"index"`
		HelpID  int    `json:"helpId"`
		State   int    `json:"state"`
		Remain  int    `json:"remain"`
		Penalty int    `json:"penalty"`
		Text    string `json:"text"`
	} `json:"penalties"`
	Hints []struct {
		Number int  `json:"number"`
		Time   int  `json:"time"`
		Shown  bool `json:"shown"`
	} `json:"hints"`
	Required int      `json:"required"`
	Closed   int      `json:"closed"`
	Missing  []string `json:"missing,omitempty"`
}

// Game — загруженные конфиги игры.
type Game struct {
	Conf     *config.Game
	Prepared []config.PreparedLevel
	byNum    map[int]*config.PreparedLevel
	numbers  []int
}

// NewGame индексирует подготовленные уровни по номеру.
func NewGame(conf *config.Game, prepared []config.PreparedLevel) *Game {
	g := &Game{Conf: conf, Prepared: prepared, byNum: map[int]*config.PreparedLevel{}}
	for i := range prepared {
		p := &prepared[i]
		g.byNum[p.Conf.Level] = p
		g.numbers = append(g.numbers, p.Conf.Level)
	}
	sort.Ints(g.numbers)
	return g
}

// Level возвращает уровень по номеру.
func (g *Game) Level(n int) (*config.PreparedLevel, bool) {
	p, ok := g.byNum[n]
	return p, ok
}

// Numbers — номера уровней по возрастанию.
func (g *Game) Numbers() []int { return g.numbers }

// First — первый уровень конфига.
func (g *Game) First() int {
	if len(g.numbers) == 0 {
		return 0
	}
	return g.numbers[0]
}

// Next — следующий по конфигу уровень после n (0 — n последний).
func (g *Game) Next(n int) int {
	for _, x := range g.numbers {
		if x > n {
			return x
		}
	}
	return 0
}

// Prev — предыдущий по конфигу уровень (0 — n первый).
func (g *Game) Prev(n int) int {
	prev := 0
	for _, x := range g.numbers {
		if x >= n {
			break
		}
		prev = x
	}
	return prev
}

// Max — наибольший номер уровня («Уровень N из M»).
func (g *Game) Max() int {
	if len(g.numbers) == 0 {
		return 0
	}
	return g.numbers[len(g.numbers)-1]
}

// Title — название игры для шапки.
func (g *Game) Title() string {
	if g.Conf.Title != "" {
		return g.Conf.Title
	}
	return fmt.Sprintf("Игра %d", g.Conf.GameID)
}

// checkAutopass помечает уровень пройденным по таймауту, если время вышло.
func checkAutopass(p *config.PreparedLevel, ls *LevelState, now time.Time) {
	if ls.Passed || p.Conf.Autopass == nil || *p.Conf.Autopass <= 0 {
		return
	}
	if ls.Elapsed(now) >= time.Duration(*p.Conf.Autopass)*time.Second {
		ls.Passed = true
		ls.PassedBy = "timeout"
	}
}

// requiredSectors — сколько секторов нужно закрыть.
func requiredSectors(p *config.PreparedLevel, total int) int {
	if p.Conf.SectorsToClose != nil && *p.Conf.SectorsToClose > 0 && *p.Conf.SectorsToClose < total {
		return *p.Conf.SectorsToClose
	}
	return total
}

func fmtClock(d time.Duration) string {
	s := int(d.Seconds())
	h, m, sec := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// BuildView собирает представление уровня n по конфигам, состоянию и запросу.
// Состояние не мутирует, кроме ленивой проверки автоперехода (checkAutopass).
func BuildView(g *Game, n int, gs *GameState, now time.Time, rw *Rewriter, env Env, req Request) (*View, error) {
	p, ok := g.Level(n)
	if !ok {
		return nil, fmt.Errorf("уровень %d не найден в конфиге", n)
	}
	ls := gs.Level(n, now)
	checkAutopass(p, ls, now)
	elapsed := ls.Elapsed(now)
	elapsedSec := int(elapsed.Seconds())

	v := &View{
		EngineBase: env.EngineBase, ToastrBase: env.ToastrBase, EngineVer: EngineVer,
		GameID: g.Conf.GameID, GameTitle: HTMLEncode(g.Title()), Topic: g.Conf.Topic,
		PlayPath: PlayPath(g.Conf.GameID), Rnd: randRnd(),
		LangHrefEN: langHref(PlayPath(g.Conf.GameID), req.RawQuery, "en"),
		LangHrefRU: langHref(PlayPath(g.Conf.GameID), req.RawQuery, "ru"),
		Number: n, LevelID: levelID(n), LevelName: deref(p.Conf.Name),
		LevelNameHTML: template.HTML(HTMLEncode(deref(p.Conf.Name))),
		LevelsTotal:   g.Max(), Prev: g.Prev(n), Next: g.Next(n), IsLast: g.Next(n) == 0,
		AnswerAttr:   template.HTMLAttr(`value="` + HTMLEncode(req.AnswerValue) + `"`),
		Elapsed:      elapsedSec, ElapsedText: fmtClock(elapsed),
		Paused:       ls.Paused, AutoAdvance: gs.AutoAdvance, Passed: ls.Passed, PassedBy: ls.PassedBy,
		Timeout:      derefInt(p.Conf.Autopass), TimeoutAward: derefInt(p.Conf.AutopassPenalty),
		Notice:       req.Notice, NoticeCorrect: req.NoticeCorrect, ConfirmIndex: req.ConfirmIndex,
	}
	if v.Timeout > 0 {
		v.TimeoutRemain = v.Timeout - elapsedSec
		if v.TimeoutRemain < 0 {
			v.TimeoutRemain = 0
		}
	}
	v.TaskHTML = template.HTML(rw.Content(p.Body))

	// Сектора.
	sectors := SectorCodes(p.Codes)
	for i, c := range sectors {
		sv := SectorView{Index: i, Order: i + 1, SectorID: sectorID(n, i), Name: deref(c.SectorName)}
		if e, done := ls.Sectors[i]; done {
			sv.Answered, sv.Answer, sv.AnsweredAt, sv.Login, sv.UserID = true, e.Answer, e.At, loginOr(e.Login, env.Login), env.UserID
			v.PassedSectors++
		}
		v.Sectors = append(v.Sectors, sv)
		v.Codes = append(v.Codes, CodeRow{
			Key: fmt.Sprintf("s:%d", i), Kind: "sector", Type: string(c.Type),
			SectorName: deref(c.SectorName), BonusName: deref(c.BonusName),
			Answers: c.Answers, Entered: sv.Answered, SectorIndex: i, BonusIndex: -1,
			Time: derefInt(c.Time), Negative: c.Type.IsPenalty(),
		})
	}
	v.RequiredSectors = requiredSectors(p, len(sectors))
	v.SectorsLeft = v.RequiredSectors - v.PassedSectors
	if v.SectorsLeft < 0 {
		v.SectorsLeft = 0
	}

	// Подсказки.
	for i, h := range p.Conf.Hints {
		hv := HintView{Number: i + 1, HelpID: helpID(n, i), Time: h.Time, Remain: h.Time - elapsedSec}
		if hv.Remain <= 0 {
			hv.Remain, hv.Shown = 0, true
			hv.Text = template.HTML(rw.Content(h.Text))
		}
		v.Hints = append(v.Hints, hv)
	}
	for i, h := range p.Conf.PenaltyHints {
		pv := PenaltyView{
			Number: i + 1, HelpID: penaltyHelpID(n, i), Index: i, Time: h.Time,
			Remain: h.Time - elapsedSec, State: ls.OpenedPenalty[i],
			Penalty: derefInt(h.Penalty), Comment: deref(h.Comment),
		}
		if pv.Remain < 0 {
			pv.Remain = 0
		}
		if pv.State == 2 {
			pv.Text = template.HTML(rw.Content(h.Text))
		}
		v.Penalties = append(v.Penalties, pv)
	}

	// Бонусы: свои + чужие по levels (в порядке создания).
	for bi, b := range config.BonusesForLevel(g.Prepared, n) {
		key := BonusKey(b.OwnerLevel, b.Index)
		bv := BonusView{
			Key: key, Number: bi + 1, BonusID: bonusID(b.OwnerLevel, b.Index),
			Name: deref(b.Code.BonusName), Task: template.HTML(rw.Content(deref(b.Code.Task))),
			Award: derefInt(b.Code.Time), Negative: b.Code.Type.IsPenalty(), Foreign: b.OwnerLevel != n,
		}
		if e, done := gs.Bonuses[key]; done {
			bv.Answered, bv.Answer, bv.AnsweredAt, bv.Login = true, e.Answer, e.At, loginOr(e.Login, env.Login)
			bv.Help = template.HTML(rw.Content(deref(b.Code.Help)))
			if e.Level == n {
				v.PassedBonuses++
			}
		}
		v.Bonuses = append(v.Bonuses, bv)
		if b.Code.Type.HasSector() && b.OwnerLevel == n {
			continue // секторбонус уже есть в Codes строкой сектора
		}
		v.Codes = append(v.Codes, CodeRow{
			Key: "b:" + key, Kind: "bonus", Type: string(b.Code.Type),
			SectorName: deref(b.Code.SectorName), BonusName: deref(b.Code.BonusName),
			Answers: b.Code.Answers, Entered: bv.Answered, OwnerLevel: b.OwnerLevel,
			Foreign: b.OwnerLevel != n, Time: bv.Award, Negative: bv.Negative,
			SectorIndex: -1, BonusIndex: b.Index,
		})
	}

	// История: перенесённые записи прошлого уровня, затем свои, новые сверху.
	var actions []encx.CodeAction
	actions = append(actions, req.Carry...)
	actions = append(actions, ls.History...)
	for i := len(actions) - 1; i >= 0; i-- {
		a := actions[i]
		hv := HistoryView{Answer: a.Answer, Correct: a.IsCorrect, Kind: a.Kind, Negative: a.Negative,
			Time: a.LocDateTime, Login: loginOr(a.Login, env.Login), UserID: env.UserID, JustNow: req.JustNow[a.ActionId]}
		if a.LevelNumber != n {
			hv.OtherLevel = a.LevelNumber
		}
		v.History = append(v.History, hv)
	}

	// Список уровней для панели.
	for _, num := range g.Numbers() {
		lp, _ := g.Level(num)
		nav := LevelNav{Number: num, Name: deref(lp.Conf.Name), Current: num == n}
		if st, ok := gs.Levels[num]; ok {
			nav.Started = true
			nav.Passed = st.Passed
		}
		v.Levels = append(v.Levels, nav)
	}
	v.Missing = rw.Missing()
	sort.Strings(v.Missing)

	// Блоки разметки движка.
	v.HistoryHTML = template.HTML(renderHistory(v))
	v.TimerHTML = template.HTML(renderTimer(v))
	v.SectorsHTML = template.HTML(renderSectors(v))
	v.TaskBlockHTML = template.HTML(renderTask(v))
	v.HelpsHTML = template.HTML(renderHelps(v))
	v.PenaltiesHTML = template.HTML(renderPenalties(v))
	v.BonusesHTML = template.HTML(renderBonuses(v))
	v.PanelHTML = template.HTML("\n<!--emu--><div id=\"emu-panel-host\"></div><script src=\"/emu/panel.js\" defer></script>")

	v.Model = buildModel(v, ls)
	return v, nil
}

func loginOr(login, def string) string {
	if login != "" {
		return login
	}
	return def
}

// buildModel собирает модель уровня в формате ответа ?json=1 движка.
func buildModel(v *View, ls *LevelState) *encx.Level {
	m := &encx.Level{
		LevelId:              v.LevelID,
		Number:               v.Number,
		Name:                 v.LevelName,
		Timeout:              v.Timeout,
		TimeoutSecondsRemain: v.TimeoutRemain,
		TimeoutAward:         -v.TimeoutAward, // движок отдаёт штраф автоперехода отрицательным
		IsPassed:             v.Passed,
		Tasks:                []encx.LevelTask{},
		Messages:             []encx.AdminMessage{},
		Sectors:              []encx.Sector{},
		Helps:                []encx.Help{},
		Bonuses:              []encx.Bonus{},
		PenaltyHelps:         []encx.Help{},
		MixedActions:         []encx.CodeAction{},
	}
	// Уровень с одним сектором движок отдаёт без секторов и без условия прохождения.
	if len(v.Sectors) > 1 {
		m.RequiredSectorsCount, m.PassedSectorsCount, m.SectorsLeftToClose = v.RequiredSectors, v.PassedSectors, v.SectorsLeft
		for _, s := range v.Sectors {
			sec := encx.Sector{SectorId: s.SectorID, Order: s.Order, Name: s.Name, IsAnswered: s.Answered}
			if s.Answered {
				sec.Answer = encx.SectorAnswer{Answer: s.Answer, Login: s.Login, UserId: s.UserID}
			}
			m.Sectors = append(m.Sectors, sec)
		}
	}
	m.PassedBonusesCount = v.PassedBonuses
	for i := len(ls.History) - 1; i >= 0; i-- {
		m.MixedActions = append(m.MixedActions, ls.History[i])
	}
	if !ls.StartedAt.IsZero() {
		m.StartTime = &encx.DateTime{Timestamp: ls.StartedAt.Unix()}
	}
	if v.TaskHTML != "" {
		t := encx.LevelTask{TaskText: string(v.TaskHTML), TaskTextFormatted: string(v.TaskHTML)}
		m.Tasks = append(m.Tasks, t)
		m.Task = &t
	}
	for _, h := range v.Hints {
		hh := encx.Help{HelpId: h.HelpID, Number: h.Number, RemainSeconds: h.Remain}
		if h.Shown {
			t := string(h.Text)
			hh.HelpText = &t
		}
		m.Helps = append(m.Helps, hh)
	}
	for _, ph := range v.Penalties {
		hh := encx.Help{HelpId: ph.HelpID, Number: ph.Number, IsPenalty: true, Penalty: ph.Penalty, RequestConfirm: ph.Comment != "", PenaltyHelpState: ph.State, RemainSeconds: ph.Remain}
		if ph.Comment != "" {
			c := ph.Comment
			hh.PenaltyComment = &c
		}
		if ph.State == 2 {
			t := string(ph.Text)
			hh.HelpText = &t
		}
		m.PenaltyHelps = append(m.PenaltyHelps, hh)
	}
	for _, b := range v.Bonuses {
		bb := encx.Bonus{BonusId: b.BonusID, Name: b.Name, Number: b.Number, Task: string(b.Task), Help: string(b.Help), IsAnswered: b.Answered, AwardTime: b.Award, Negative: b.Negative}
		if b.Answered {
			bb.Answer = encx.SectorAnswer{Answer: b.Answer, Login: b.Login}
		}
		m.Bonuses = append(m.Bonuses, bb)
	}
	return m
}

// ModelJSON — модель уровня в JSON (для ?json=1 и панели).
func (v *View) ModelJSON() []byte {
	b, _ := json.Marshal(v.Model)
	return b
}

// Panel — данные для dev-панели.
func (v *View) Panel() panelData {
	pd := panelData{
		Level: v.Number, LevelName: v.LevelName, PlayPath: v.PlayPath, Levels: v.Levels,
		Elapsed: v.Elapsed, Paused: v.Paused, AutoAdvance: v.AutoAdvance,
		Passed: v.Passed, PassedBy: v.PassedBy, Timeout: v.Timeout,
		Codes: v.Codes, Required: v.RequiredSectors, Closed: v.PassedSectors, Missing: v.Missing,
	}
	if pd.Codes == nil {
		pd.Codes = []CodeRow{}
	}
	maxT := v.Timeout
	for _, h := range v.Hints {
		if h.Time > maxT {
			maxT = h.Time
		}
	}
	for _, ph := range v.Penalties {
		if ph.Time > maxT {
			maxT = ph.Time
		}
	}
	// Ползунок: до самого позднего события + 10 минут, минимум час.
	pd.SliderMax = maxT + 600
	if pd.SliderMax < 3600 {
		pd.SliderMax = 3600
	}
	if v.Elapsed > pd.SliderMax {
		pd.SliderMax = v.Elapsed + 600
	}
	for _, ph := range v.Penalties {
		pd.Penalties = append(pd.Penalties, struct {
			Index   int    `json:"index"`
			HelpID  int    `json:"helpId"`
			State   int    `json:"state"`
			Remain  int    `json:"remain"`
			Penalty int    `json:"penalty"`
			Text    string `json:"text"`
		}{ph.Index, ph.HelpID, ph.State, ph.Remain, ph.Penalty, strings.TrimSpace(stripTags(string(ph.Text)))})
	}
	for _, h := range v.Hints {
		pd.Hints = append(pd.Hints, struct {
			Number int  `json:"number"`
			Time   int  `json:"time"`
			Shown  bool `json:"shown"`
		}{h.Number, h.Time, h.Shown})
	}
	return pd
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string { return tagRe.ReplaceAllString(s, "") }
