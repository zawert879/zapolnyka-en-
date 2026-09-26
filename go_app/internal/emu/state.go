// Package emu — локальный эмулятор play-страницы en.cx: рендерит уровень из
// конфигов zapolnyaka так, как это делает движок, и даёт dev-панель для
// управления временем и введёнными кодами.
package emu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"zapolnyaka/encx"
)

// Entry — факт ввода кода: ответ, кем, когда и на каком уровне введён.
type Entry struct {
	Answer string    `json:"answer"`
	Login  string    `json:"login,omitempty"`
	At     time.Time `json:"at"`
	Level  int       `json:"level"`
}

// LevelState — состояние симуляции одного уровня.
//
// Время уровня: elapsed = Offset + (now − StartedAt), если уровень не на паузе.
// Пауза копит прошедшее в Offset и обнуляет StartedAt; возобновление ставит
// StartedAt = now. Подсказки — чисто производные от elapsed, поэтому откат
// ползунка их прячет; открытые штрафные подсказки — явное состояние.
type LevelState struct {
	StartedAt     time.Time         `json:"startedAt"`
	Offset        time.Duration     `json:"offset"`
	Paused        bool              `json:"paused"`
	Sectors       map[int]Entry     `json:"sectors"`       // индекс сектора (среди секторов уровня) → ввод
	OpenedPenalty map[int]int       `json:"openedPenalty"` // индекс штрафной подсказки → 1 запрошена, 2 открыта
	History       []encx.CodeAction `json:"history"`
	Attempts      int               `json:"attempts"`
	Passed        bool              `json:"passed"`
	PassedBy      string            `json:"passedBy,omitempty"` // "codes" | "timeout"
}

// GameState — состояние всей симуляции.
type GameState struct {
	Levels      map[int]*LevelState `json:"levels"`
	Bonuses     map[string]Entry    `json:"bonuses"` // ключ BonusKey(owner, idx): мультиуровневые бонусы остаются введёнными
	Current     int                 `json:"current"`
	AutoAdvance bool                `json:"autoAdvance"`
}

// BonusKey — стабильный ключ бонуса: уровень-владелец и индекс записи в его codes.
func BonusKey(ownerLevel, index int) string {
	return fmt.Sprintf("%d:%d", ownerLevel, index)
}

func newLevelState(now time.Time) *LevelState {
	return &LevelState{
		StartedAt:     now,
		Sectors:       map[int]Entry{},
		OpenedPenalty: map[int]int{},
	}
}

// Level возвращает состояние уровня, создавая его при первом обращении
// (момент создания = старт уровня, как в движке при переходе команды).
func (gs *GameState) Level(n int, now time.Time) *LevelState {
	if gs.Levels == nil {
		gs.Levels = map[int]*LevelState{}
	}
	ls, ok := gs.Levels[n]
	if !ok {
		ls = newLevelState(now)
		gs.Levels[n] = ls
	}
	if ls.Sectors == nil {
		ls.Sectors = map[int]Entry{}
	}
	if ls.OpenedPenalty == nil {
		ls.OpenedPenalty = map[int]int{}
	}
	if gs.Bonuses == nil {
		gs.Bonuses = map[string]Entry{}
	}
	return ls
}

// Elapsed — сколько времени уровня прошло к моменту now.
func (ls *LevelState) Elapsed(now time.Time) time.Duration {
	d := ls.Offset
	if !ls.Paused && !ls.StartedAt.IsZero() {
		d += now.Sub(ls.StartedAt)
	}
	if d < 0 {
		d = 0
	}
	return d
}

// Pause останавливает время уровня.
func (ls *LevelState) Pause(now time.Time) {
	if ls.Paused {
		return
	}
	ls.Offset = ls.Elapsed(now)
	ls.StartedAt = time.Time{}
	ls.Paused = true
}

// Resume запускает время уровня с текущего значения.
func (ls *LevelState) Resume(now time.Time) {
	if !ls.Paused {
		return
	}
	ls.Paused = false
	ls.StartedAt = now
}

// SetElapsed выставляет прошедшее время уровня (ползунок в панели).
func (ls *LevelState) SetElapsed(d time.Duration, now time.Time) {
	if d < 0 {
		d = 0
	}
	ls.Offset = d
	if ls.Paused {
		ls.StartedAt = time.Time{}
	} else {
		ls.StartedAt = now
	}
}

// ResetLevel сбрасывает уровень n и бонусы, введённые на нём.
func (gs *GameState) ResetLevel(n int, now time.Time) {
	if gs.Levels != nil {
		delete(gs.Levels, n)
	}
	for k, e := range gs.Bonuses {
		if e.Level == n {
			delete(gs.Bonuses, k)
		}
	}
	gs.Level(n, now)
}

// ResetAll сбрасывает всю симуляцию, сохраняя настройку автоперехода.
func (gs *GameState) ResetAll() {
	auto := gs.AutoAdvance
	*gs = GameState{
		Levels:      map[int]*LevelState{},
		Bonuses:     map[string]Entry{},
		AutoAdvance: auto,
	}
}

// Store — потокобезопасное хранилище состояния с персистенцией в JSON-файл.
type Store struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
	gs   GameState
}

// NewStore загружает состояние из path (если файл есть). Пустой path — без персистенции.
func NewStore(path string, now func() time.Time) (*Store, error) {
	if now == nil {
		now = time.Now
	}
	s := &Store{path: path, now: now, gs: GameState{
		Levels:      map[int]*LevelState{},
		Bonuses:     map[string]Entry{},
		AutoAdvance: true,
	}}
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("emu: read state %s: %w", path, err)
	}
	var gs GameState
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("emu: parse state %s: %w", path, err)
	}
	if gs.Levels == nil {
		gs.Levels = map[int]*LevelState{}
	}
	if gs.Bonuses == nil {
		gs.Bonuses = map[string]Entry{}
	}
	s.gs = gs
	return s, nil
}

// Now — текущее время (подменяется в тестах).
func (s *Store) Now() time.Time { return s.now() }

// Update выполняет fn под блокировкой и сохраняет состояние, если fn вернула nil.
func (s *Store) Update(fn func(gs *GameState, now time.Time) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.gs, s.now()); err != nil {
		return err
	}
	return s.persist()
}

// Read выполняет fn под блокировкой без сохранения. Мутировать состояние в fn нельзя.
func (s *Store) Read(fn func(gs *GameState, now time.Time)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.gs, s.now())
}

// Snapshot возвращает копию состояния (для JSON-ответов панели).
func (s *Store) Snapshot() GameState {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(s.gs)
	var cp GameState
	_ = json.Unmarshal(data, &cp)
	return cp
}

func (s *Store) persist() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.gs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
