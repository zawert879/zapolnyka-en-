// zapolnyaka-app — оконная сборка: двойной клик открывает веб-интерфейс в
// отдельном окне Chromium (Edge/Chrome, режим --app), без терминала и вкладок.
// На Windows собирается с -ldflags "-H windowsgui" (без консольного окна),
// на macOS упаковывается в Zapolnyaka.app (packaging/macos).
//
//	zapolnyaka-app.exe [data/игра/game.yml] [--port N] [--offline]
package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"zapolnyaka/cmd"
	"zapolnyaka/pkg/logger"
)

// version задаётся при сборке: -ldflags="-X main.version=<tag>".
var version = "dev"

func main() {
	args := os.Args[1:]
	gamePath := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		gamePath, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("zapolnyaka-app", flag.ContinueOnError)
	port := fs.Int("port", 0, "порт (по умолчанию свободный)")
	offline := fs.Bool("offline", false, "CSS/JS движка из встроенной копии")
	_ = fs.Parse(args)

	// Игра не указана — рабочей становится папка, где игры есть (см.
	// cmd.FindWorkDir). С явным путём всё остаётся относительно текущей папки.
	if gamePath == "" {
		dir, places, ok := cmd.FindWorkDir()
		if !ok {
			cmd.MessageBox("zapolnyaka", noGamesMessage(places))
			os.Exit(1)
		}
		if err := os.Chdir(dir); err != nil {
			cmd.MessageBox("zapolnyaka", "Ошибка: "+err.Error())
			os.Exit(1)
		}
	}

	closeLog := logger.Init("zapolnyaka.log")
	defer closeLog()

	if gamePath == "" {
		gamePath = cmd.DefaultGamePath()
	}
	if err := cmd.ActionApp(gamePath, cmd.AppOptions{Port: *port, Offline: *offline}, version); err != nil {
		logger.Println("app ERROR: " + err.Error())
		cmd.MessageBox("zapolnyaka", "Ошибка: "+err.Error()+"\n\nПодробности в zapolnyaka.log")
		os.Exit(1)
	}
}

// noGamesMessage — что показать, когда игр нет ни в одной из папок places.
func noGamesMessage(places []string) string {
	game := filepath.Join("data", "<игра>")
	var b strings.Builder
	b.WriteString("Не найдено ни одной игры.\n\nПоложите папку " + game + " с game.yml в одну из папок:\n")
	for _, p := range places {
		b.WriteString("  • " + p + "\n")
	}
	b.WriteString("\nили укажите путь к game.yml аргументом и запустите снова.")
	return b.String()
}
