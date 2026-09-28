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
	// cmd.FindWorkDir); если игр нет нигде — cmd.NewWorkDir, и интерфейс
	// откроется без игры с формой «Создать игру» (data/ появится при создании).
	// С явным путём всё остаётся относительно текущей папки.
	if gamePath == "" {
		dir, _, ok := cmd.FindWorkDir()
		if !ok {
			var err error
			if dir, err = cmd.NewWorkDir(); err != nil {
				cmd.MessageBox("zapolnyaka", "Не удалось подготовить рабочую папку: "+err.Error())
				os.Exit(1)
			}
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
