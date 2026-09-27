package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// mkGame создаёт в dir игру data/<name>/game.yml.
func mkGame(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, "data", name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "game.yml"), []byte("domain: demo.en.cx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHasGame(t *testing.T) {
	empty := t.TempDir()
	if hasGame(empty) {
		t.Error("пустая папка: игр быть не должно")
	}
	if hasGame(filepath.Join(empty, "нет-такой")) {
		t.Error("несуществующая папка: игр быть не должно")
	}

	// data/ есть, но game.yml в ней нет.
	noGame := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noGame, "data", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hasGame(noGame) {
		t.Error("data/ без game.yml: игр быть не должно")
	}

	withData := t.TempDir()
	mkGame(t, withData, "g1")
	if !hasGame(withData) {
		t.Error("data/g1/game.yml не найден")
	}

	// Игра вне data/, на неё указывает история (путь относительно папки).
	withHist := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withHist, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withHist, "examples", "game.yml"), []byte("domain: demo.en.cx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withHist, historyFile), []byte(`{"lastGame":"examples/game.yml"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasGame(withHist) {
		t.Error("игра из истории не найдена")
	}

	// История указывает на удалённую игру, в data/ пусто.
	stale := t.TempDir()
	if err := os.WriteFile(filepath.Join(stale, historyFile), []byte(`{"lastGame":"data/gone/game.yml"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasGame(stale) {
		t.Error("история с несуществующей игрой: игр быть не должно")
	}
}

func TestPickWorkDir(t *testing.T) {
	exe, cwd, home := t.TempDir(), t.TempDir(), t.TempDir()

	if dir, ok := pickWorkDir(exe, cwd, home); ok {
		t.Errorf("игр нет нигде, а выбрана %q", dir)
	}

	mkGame(t, home, "g")
	if dir, ok := pickWorkDir(exe, cwd, home); !ok || dir != home {
		t.Errorf("игра только в домашней подпапке: выбрана %q, ok=%v", dir, ok)
	}

	mkGame(t, cwd, "g")
	if dir, _ := pickWorkDir(exe, cwd, home); dir != cwd {
		t.Errorf("текущая папка важнее домашней: выбрана %q", dir)
	}

	mkGame(t, exe, "g")
	if dir, _ := pickWorkDir(exe, cwd, home); dir != exe {
		t.Errorf("папка бинаря важнее остальных: выбрана %q", dir)
	}

	// Неизвестная папка бинаря ("") пропускается.
	if dir, _ := pickWorkDir("", cwd, home); dir != cwd {
		t.Errorf("пустой кандидат: выбрана %q", dir)
	}
}

func TestProgramDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("пути в тесте — unix")
	}
	for _, tc := range []struct{ exe, want string }{
		{"/Users/u/PET/EN/zapolnyaka-app", "/Users/u/PET/EN"},
		{"/Users/u/PET/EN/Zapolnyaka.app/Contents/MacOS/zapolnyaka-app", "/Users/u/PET/EN"},
		{"/Applications/Zapolnyaka.app/Contents/MacOS/zapolnyaka-app", "/Applications"},
		// Папка MacOS не внутри бандла — обычный бинарь.
		{"/opt/tools/MacOS/zapolnyaka-app", "/opt/tools/MacOS"},
		{"/private/var/folders/ab/T/AppTranslocation/1F2E/d/Zapolnyaka.app/Contents/MacOS/zapolnyaka-app", ""},
	} {
		if got := programDir(tc.exe); got != tc.want {
			t.Errorf("programDir(%q) = %q, нужно %q", tc.exe, got, tc.want)
		}
	}
}
