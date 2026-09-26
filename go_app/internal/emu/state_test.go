package emu

import (
	"path/filepath"
	"testing"
	"time"
)

func TestElapsedPauseResumeSet(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ls := newLevelState(t0)

	if got := ls.Elapsed(t0.Add(10 * time.Second)); got != 10*time.Second {
		t.Fatalf("elapsed after 10s = %v", got)
	}
	ls.Pause(t0.Add(10 * time.Second))
	if got := ls.Elapsed(t0.Add(50 * time.Second)); got != 10*time.Second {
		t.Fatalf("paused elapsed = %v, want 10s", got)
	}
	ls.Resume(t0.Add(50 * time.Second))
	if got := ls.Elapsed(t0.Add(65 * time.Second)); got != 25*time.Second {
		t.Fatalf("resumed elapsed = %v, want 25s", got)
	}
	ls.SetElapsed(600*time.Second, t0.Add(65*time.Second))
	if got := ls.Elapsed(t0.Add(70 * time.Second)); got != 605*time.Second {
		t.Fatalf("set elapsed = %v, want 605s", got)
	}
	ls.Pause(t0.Add(70 * time.Second))
	ls.SetElapsed(0, t0.Add(80*time.Second))
	if got := ls.Elapsed(t0.Add(100 * time.Second)); got != 0 {
		t.Fatalf("paused+set elapsed = %v, want 0", got)
	}
}

func TestStorePersistAndReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	s, err := NewStore(path, clock)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(gs *GameState, now time.Time) error {
		ls := gs.Level(6, now)
		ls.Sectors[0] = Entry{Answer: "x", At: now, Level: 6}
		gs.Bonuses[BonusKey(6, 1)] = Entry{Answer: "b", At: now, Level: 6}
		gs.Bonuses[BonusKey(7, 0)] = Entry{Answer: "c", At: now, Level: 7}
		gs.AutoAdvance = false
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	s2, err := NewStore(path, clock)
	if err != nil {
		t.Fatal(err)
	}
	s2.Read(func(gs *GameState, _ time.Time) {
		if gs.Levels[6] == nil || gs.Levels[6].Sectors[0].Answer != "x" {
			t.Fatalf("sector not persisted: %+v", gs.Levels[6])
		}
		if len(gs.Bonuses) != 2 || gs.AutoAdvance {
			t.Fatalf("bonuses/autoAdvance not persisted: %+v", gs)
		}
	})

	_ = s2.Update(func(gs *GameState, now time.Time) error { gs.ResetLevel(6, now); return nil })
	s2.Read(func(gs *GameState, _ time.Time) {
		if len(gs.Levels[6].Sectors) != 0 {
			t.Fatalf("sectors not reset")
		}
		if _, ok := gs.Bonuses[BonusKey(6, 1)]; ok {
			t.Fatalf("bonus of level 6 not reset")
		}
		if _, ok := gs.Bonuses[BonusKey(7, 0)]; !ok {
			t.Fatalf("bonus of level 7 must survive reset of level 6")
		}
	})
}
