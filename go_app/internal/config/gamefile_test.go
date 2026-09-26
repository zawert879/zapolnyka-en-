package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gameWithDisabled = `domain: tech.en.cx
gameId: 82817
levels:
    - 1/conf.yml
    - 2doezd/conf.yml   # выезд
    # - 4doezd/conf.yml
    #- 5/conf.yml
    # - "6/conf.json"
    # просто комментарий - не уровень
    # - todo
defaultFormat: yml
# - 9/conf.yml   (вне блока levels — не считается)
`

func TestDisabledLevels(t *testing.T) {
	got := DisabledLevels([]byte(gameWithDisabled))
	want := []string{"4doezd/conf.yml", "5/conf.yml", "6/conf.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	if DisabledLevels([]byte("domain: x\nlevels: [1/conf.yml]\n")) != nil {
		t.Fatal("flow style must give nothing")
	}
}

func TestToggleLevel(t *testing.T) {
	out, found, err := ToggleLevel([]byte(gameWithDisabled), "4doezd/conf.yml", true)
	if err != nil || !found {
		t.Fatalf("enable: %v %v", found, err)
	}
	if !strings.Contains(string(out), "\n    - 4doezd/conf.yml\n") || !strings.Contains(string(out), "    #- 5/conf.yml") {
		t.Fatalf("enable result:\n%s", out)
	}
	out, found, err = ToggleLevel(out, "2doezd/conf.yml", false)
	if err != nil || !found {
		t.Fatalf("disable: %v %v", found, err)
	}
	if !strings.Contains(string(out), "\n    # - 2doezd/conf.yml   # выезд\n") {
		t.Fatalf("disable result:\n%s", out)
	}
	if _, found, _ := ToggleLevel(out, "nope/conf.yml", true); found {
		t.Fatal("unknown level must not be found")
	}
	// Комментарий вне блока levels не трогаем.
	if _, found, _ := ToggleLevel(out, "9/conf.yml", true); found {
		t.Fatal("line outside levels block must be ignored")
	}
	// CRLF сохраняется.
	crlf := strings.ReplaceAll(gameWithDisabled, "\n", "\r\n")
	out, _, _ = ToggleLevel([]byte(crlf), "4doezd/conf.yml", true)
	if !strings.Contains(string(out), "    - 4doezd/conf.yml\r\n") {
		t.Fatalf("crlf lost:\n%q", out)
	}
}

func TestLoadGameDisabledAndLoadAllWithDisabled(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("game.yml", "domain: tech.en.cx\ngameId: 1\nlevels:\n  - 1/conf.yml\n  # - 2/conf.yml\n  # - 3/conf.yml\n")
	write("1/conf.yml", "level: 1\nbody: task.html\n")
	write("1/task.html", "one")
	write("2/conf.yml", "level: 2\n")
	// 3 — битый конфиг: выключенный уровень с ошибкой не должен ронять загрузку.
	write("3/conf.yml", "level: [\n")

	g, err := LoadGame(filepath.Join(dir, "game.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Levels) != 1 || strings.Join(g.Disabled, ",") != "2/conf.yml,3/conf.yml" {
		t.Fatalf("game: %+v", g)
	}
	_, prepared, err := LoadAll(filepath.Join(dir, "game.yml"))
	if err != nil || len(prepared) != 1 || prepared[0].Disabled {
		t.Fatalf("LoadAll: %v %+v", err, prepared)
	}
	_, all, err := LoadAllWithDisabled(filepath.Join(dir, "game.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Disabled || !all[1].Disabled || all[1].Conf.Level != 2 || all[0].ConfRel != "1/conf.yml" || all[1].ConfRel != "2/conf.yml" {
		t.Fatalf("LoadAllWithDisabled: %+v", all)
	}
	if err := SetLevelEnabled(filepath.Join(dir, "game.yml"), "2/conf.yml", true); err != nil {
		t.Fatal(err)
	}
	g, _ = LoadGame(filepath.Join(dir, "game.yml"))
	if len(g.Levels) != 2 || len(g.Disabled) != 1 {
		t.Fatalf("after enable: %+v", g)
	}
	if err := SetLevelEnabled(filepath.Join(dir, "game.yml"), "2/conf.yml", true); err == nil {
		t.Fatal("enabling an already enabled level must fail")
	}
}
