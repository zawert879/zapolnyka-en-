package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// HomeWorkDir — подпапка домашнего каталога, где приложение ищет игры, если
// рядом с бинарём их нет (бинарь лежит в /Applications, в «Загрузках» и т.п.).
const HomeWorkDir = "zapolnyaka-en"

// FindWorkDir выбирает рабочую папку — ту, относительно которой лежат data/,
// .zapolnyaka.json и zapolnyaka.log. Нужен при запуске двойным кликом: текущая
// папка тогда не та, где программа (Finder стартует бинарь из домашней, а
// .app-бандл — из «/»). Порядок: папка бинаря, текущая, ~/zapolnyaka-en —
// берётся первая, где есть игра. Если игр нет нигде — ok=false, а places —
// папки, куда их можно положить (для сообщения пользователю).
func FindWorkDir() (dir string, places []string, ok bool) {
	exe, home := exeDir(), homeWorkDir()
	cwd, _ := os.Getwd()
	if dir, ok := pickWorkDir(exe, cwd, home); ok {
		return dir, nil, true
	}
	for _, d := range []string{exe, home} {
		if d != "" {
			places = append(places, d)
		}
	}
	return "", places, false
}

// NewWorkDir — рабочая папка для первого запуска, когда игр нет нигде: папка
// программы, если туда можно писать (переносная установка рядом с бинарём), иначе
// ~/zapolnyaka-en (бандл в /Applications, карантин macOS, Program Files). data/
// в ней появится, когда в интерфейсе создадут первую игру.
func NewWorkDir() (string, error) {
	exe := exeDir()
	if exe != "" && !isSystemAppDir(exe) && writable(exe) {
		return exe, nil
	}
	home := homeWorkDir()
	if home == "" {
		return "", os.ErrNotExist
	}
	return home, os.MkdirAll(home, 0o755)
}

// isSystemAppDir — общие папки программ, где заводить data/ не стоит.
func isSystemAppDir(dir string) bool {
	d := filepath.ToSlash(dir)
	if d == "/Applications" || strings.HasPrefix(d, "/Applications/") {
		return true
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if pf := os.Getenv(env); pf != "" && strings.HasPrefix(strings.ToLower(d), strings.ToLower(filepath.ToSlash(pf))) {
			return true
		}
	}
	return false
}

// writable — можно ли создавать файлы в dir.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".zapolnyaka-write-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// pickWorkDir — первая из папок, в которой есть игра.
func pickWorkDir(candidates ...string) (string, bool) {
	for _, d := range candidates {
		if d != "" && hasGame(d) {
			return d, true
		}
	}
	return "", false
}

// hasGame — то же условие, что в DefaultGamePath, но для произвольной папки:
// последняя игра из истории существует или в data/ есть хотя бы одна игра.
func hasGame(dir string) bool {
	if data, err := os.ReadFile(filepath.Join(dir, historyFile)); err == nil {
		var h History
		if json.Unmarshal(data, &h) == nil && h.LastGame != "" {
			p := h.LastGame
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
	}
	return len(ScanGames(filepath.Join(dir, "data"))) > 0
}

// exeDir — папка, в которой лежит программа: для .app-бандла — папка с самим
// бандлом. Пусто, если её не узнать.
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return programDir(exe)
}

// programDir — папка программы по пути к бинарю. Для бинаря внутри бандла
// (X.app/Contents/MacOS/бинарь) это папка, где лежит X.app. Бандл, запущенный
// из карантина без переноса, macOS исполняет из случайной read-only папки
// (App Translocation) — настоящей папки оттуда не узнать, возвращается "".
func programDir(exe string) string {
	if strings.Contains(exe, "/AppTranslocation/") {
		return ""
	}
	dir := filepath.Dir(exe)
	contents := filepath.Dir(dir)
	bundle := filepath.Dir(contents)
	if filepath.Base(dir) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(bundle, ".app") {
		return filepath.Dir(bundle)
	}
	return dir
}

func homeWorkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, HomeWorkDir)
}
