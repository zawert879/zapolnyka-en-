package emu

import (
	"testing"
	"time"

	"zapolnyaka/internal/config"
)

func TestMatch(t *testing.T) {
	sectors := []config.Code{
		{Type: config.CodeTypeSector, Answers: []string{"Alpha"}},
		{Type: config.CodeTypeSectorBonus, Answers: []string{"combo"}},
	}
	bonuses := []config.BonusRef{
		{OwnerLevel: 1, Index: 1, Code: config.Code{Type: config.CodeTypeSectorBonus, Answers: []string{"combo"}}},
		{OwnerLevel: 1, Index: 2, Code: config.Code{Type: config.CodeTypeBonus, Answers: []string{"beta"}}},
	}
	gs := &GameState{}
	ls := gs.Level(1, time.Now())

	m := Match("  ALPHA ", sectors, bonuses, ls, gs)
	if len(m.Sectors) != 1 || m.Sectors[0] != 0 || len(m.Bonuses) != 0 || !m.Correct() {
		t.Fatalf("alpha: %+v", m)
	}

	m = Match("combo", sectors, bonuses, ls, gs)
	if len(m.Sectors) != 1 || m.Sectors[0] != 1 || len(m.Bonuses) != 1 || m.Bonuses[0] != "1:1" {
		t.Fatalf("combo must hit sector and bonus: %+v", m)
	}

	m = Match("beta", sectors, bonuses, ls, gs)
	if len(m.Sectors) != 0 || len(m.Bonuses) != 1 || m.Bonuses[0] != "1:2" {
		t.Fatalf("beta: %+v", m)
	}

	if m := Match("nope", sectors, bonuses, ls, gs); m.Correct() {
		t.Fatalf("nope must be incorrect: %+v", m)
	}
	if m := Match("   ", sectors, bonuses, ls, gs); m.Correct() {
		t.Fatalf("blank must be incorrect")
	}

	ls.Sectors[0] = Entry{Answer: "alpha"}
	m = Match("alpha", sectors, bonuses, ls, gs)
	if len(m.Sectors) != 0 || len(m.AlreadySectors) != 1 || !m.Correct() {
		t.Fatalf("already closed sector: %+v", m)
	}
}
