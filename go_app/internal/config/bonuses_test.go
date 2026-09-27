package config

import "testing"

func TestBonusesForLevelSkipsDisabledOwner(t *testing.T) {
	all := "*"
	lvl := func(n int) *Level { return &Level{Level: n} }
	prepared := []PreparedLevel{
		{Conf: lvl(1), Disabled: true, Codes: []Code{{Type: "бонус", Levels: &all}}},
		{Conf: lvl(2), Codes: []Code{{Type: "бонус"}}},
	}
	if got := BonusesForLevel(prepared, 2); len(got) != 1 || got[0].OwnerLevel != 2 {
		t.Fatalf("level 2: want only own bonus, got %+v", got)
	}
	if got := BonusesForLevel(prepared, 1); len(got) != 1 || got[0].OwnerLevel != 1 {
		t.Fatalf("level 1 (disabled): want its own bonus, got %+v", got)
	}
	prepared[0].Disabled = false
	if got := BonusesForLevel(prepared, 2); len(got) != 2 {
		t.Fatalf("level 2 with enabled owner: want 2 bonuses, got %+v", got)
	}
}
