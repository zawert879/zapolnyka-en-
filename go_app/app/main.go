// zapolnyaka-app — оконная сборка: двойной клик открывает веб-интерфейс в
// отдельном окне Chromium (Edge/Chrome, режим --app), без терминала и вкладок.
// На Windows собирается с -ldflags "-H windowsgui" (без консольного окна).
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
	closeLog := logger.Init("zapolnyaka.log")
	defer closeLog()

	args := os.Args[1:]
	gamePath := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		gamePath, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("zapolnyaka-app", flag.ContinueOnError)
	port := fs.Int("port", 0, "порт (по умолчанию свободный)")
	offline := fs.Bool("offline", false, "CSS/JS движка из встроенной копии")
	_ = fs.Parse(args)

	if gamePath == "" {
		gamePath = cmd.DefaultGamePath()
	}
	if gamePath == "" {
		msg := "Не найдено ни одной игры.\n\nПоложите рядом с программой папку data\\<игра>\\ с game.yml (или укажите путь к game.yml аргументом) и запустите снова."
		logger.Println("app: " + msg)
		cmd.MessageBox("zapolnyaka", msg)
		os.Exit(1)
	}
	if err := cmd.ActionApp(gamePath, cmd.AppOptions{Port: *port, Offline: *offline}, version); err != nil {
		logger.Println("app ERROR: " + err.Error())
		cmd.MessageBox("zapolnyaka", "Ошибка: "+err.Error()+"\n\nПодробности в zapolnyaka.log")
		os.Exit(1)
	}
}
