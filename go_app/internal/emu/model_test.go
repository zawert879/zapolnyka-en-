package emu

import (
	"strings"
	"testing"
	"time"

	"zapolnyaka/internal/config"
)

const fixtureGame = "testdata/game/game.yml"

func loadFixture(t *testing.T) (*Game, *Rewriter) {
	t.Helper()
	conf, prepared, err := config.LoadAll(fixtureGame)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return NewGame(conf, prepared), NewRewriter("testdata/game/assets", nil)
}

func TestBuildViewHintsBodyAndTime(t *testing.T) {
	g, rw := loadFixture(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	gs := &GameState{}

	v, err := BuildView(g, 1, gs, now, rw, DefaultEnv(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if v.LevelsTotal != 3 || v.Next != 2 || v.Prev != 0 {
		t.Fatalf("nav: total=%d next=%d prev=%d", v.LevelsTotal, v.Next, v.Prev)
	}
	if len(v.Hints) != 2 || !v.Hints[0].Shown || v.Hints[1].Shown || v.Hints[1].Remain != 120 {
		t.Fatalf("hints: %+v", v.Hints)
	}
	body := string(v.TaskHTML)
	if !strings.Contains(body, "/assets/design.css") || strings.Contains(body, "<link") || !strings.Contains(body, "Level one body") {
		t.Fatalf("body: %q", body)
	}
	if v.RequiredSectors != 1 || len(v.Sectors) != 1 || len(v.Bonuses) != 1 || len(v.Codes) != 2 {
		t.Fatalf("counts: req=%d sectors=%d bonuses=%d codes=%d", v.RequiredSectors, len(v.Sectors), len(v.Bonuses), len(v.Codes))
	}
	if v.Model == nil || v.Model.Number != 1 || len(v.Model.Helps) != 2 || v.Model.Helps[0].HelpText == nil || v.Model.Helps[1].HelpText != nil {
		t.Fatalf("model helps: %+v", v.Model.Helps)
	}

	// Перемотка на 130 секунд показывает вторую подсказку; назад — прячет.
	ls := gs.Level(1, now)
	ls.SetElapsed(130*time.Second, now)
	v, _ = BuildView(g, 1, gs, now, rw, DefaultEnv(), Request{})
	if !v.Hints[1].Shown || string(v.Hints[1].Text) != "через две минуты" {
		t.Fatalf("hint 2 must be shown at 130s: %+v", v.Hints[1])
	}
	ls.SetElapsed(10*time.Second, now)
	v, _ = BuildView(g, 1, gs, now, rw, DefaultEnv(), Request{})
	if v.Hints[1].Shown {
		t.Fatalf("hint 2 must hide when time goes back")
	}
}

func TestBuildViewMultiLevelBonusAndAutopass(t *testing.T) {
	g, rw := loadFixture(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	gs := &GameState{}

	v2, err := BuildView(g, 2, gs, now, rw, DefaultEnv(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if v2.RequiredSectors != 1 || len(v2.Sectors) != 2 || v2.SectorsLeft != 1 {
		t.Fatalf("sectorsToClose: req=%d sectors=%d left=%d", v2.RequiredSectors, len(v2.Sectors), v2.SectorsLeft)
	}
	if len(v2.Bonuses) != 1 || v2.Bonuses[0].Foreign {
		t.Fatalf("level 2 bonuses: %+v", v2.Bonuses)
	}
	if v2.Timeout != 600 || v2.TimeoutRemain != 600 || v2.TimeoutAward != 60 {
		t.Fatalf("autopass: %d/%d/%d", v2.Timeout, v2.TimeoutRemain, v2.TimeoutAward)
	}
	if len(v2.Penalties) != 1 || v2.Penalties[0].State != 0 || v2.Penalties[0].Penalty != 60 || v2.Penalties[0].Comment != "sure?" {
		t.Fatalf("penalty: %+v", v2.Penalties)
	}

	v3, _ := BuildView(g, 3, gs, now, rw, DefaultEnv(), Request{})
	if len(v3.Bonuses) != 2 {
		t.Fatalf("level 3 must see own bonus + multi-level bonus: %+v", v3.Bonuses)
	}
	foreign := 0
	for _, b := range v3.Bonuses {
		if b.Foreign {
			foreign++
		}
	}
	if foreign != 1 || len(v3.Codes) != 2 {
		t.Fatalf("foreign=%d codes=%d", foreign, len(v3.Codes))
	}

	// Автопереход по времени.
	ls2 := gs.Level(2, now)
	ls2.SetElapsed(600*time.Second, now)
	v2, _ = BuildView(g, 2, gs, now, rw, DefaultEnv(), Request{})
	if !v2.Passed || v2.PassedBy != "timeout" || v2.TimeoutRemain != 0 {
		t.Fatalf("autopass must pass the level: passed=%v by=%q remain=%d", v2.Passed, v2.PassedBy, v2.TimeoutRemain)
	}

	pd := v2.Panel()
	if pd.SliderMax < 600+600 || pd.Timeout != 600 || len(pd.Penalties) != 1 {
		t.Fatalf("panel: %+v", pd)
	}
}
