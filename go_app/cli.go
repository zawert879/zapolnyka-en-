package main

// cli.go — argument parsing layer, NO huh/bubbletea imports.
// Dispatches to cmd.Action*; on completion the process exits.

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"zapolnyaka/cmd"
	"zapolnyaka/pkg/logger"

	"github.com/charmbracelet/lipgloss"
)

var cliErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

// isCLIMode reports whether the binary was invoked with command-line arguments.
func isCLIMode() bool { return len(os.Args) > 1 }

const cliUsage = "auth <login> <pass> | go [game.yml] | assets [game.yml] | validate [game.yml] | check [game.yml] | emu [game.yml] [--port 8090] [--dev] [--no-open] | snapshot [game.yml] [--name X] [--send код] [--pid id] | version"

// runCLI parses os.Args, runs the requested action, then exits.
// The TUI menu is never initialized in this path.
func runCLI() {
	action := os.Args[1]
	hist := cmd.LoadHistory()
	gamePath := hist.LastGame
	if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		gamePath = os.Args[2]
	}

	var err error
	switch action {
	case "version", "--version", "-v":
		fmt.Println("zapolnyaka " + version)
		return

	case "auth":
		if len(os.Args) < 4 {
			cliDie("Использование: zapolnyaka.exe auth <логин> <пароль>")
		}
		err = cmd.ActionAuth(strings.TrimSpace(os.Args[2]), os.Args[3])

	case "go":
		if gamePath == "" {
			cliDie("Укажите путь: zapolnyaka.exe go data/myGame/game.yml")
		}
		_ = cmd.ActionValidate(gamePath)
		fmt.Println()
		err = cmd.ActionGo(gamePath)

	case "assets":
		if gamePath == "" {
			cliDie("Укажите путь: zapolnyaka.exe assets data/myGame/game.yml")
		}
		err = cmd.ActionAssets(gamePath)

	case "validate":
		if gamePath == "" {
			cliDie("Укажите путь: zapolnyaka.exe validate data/myGame/game.yml")
		}
		err = cmd.ActionValidate(gamePath)

	case "check":
		if gamePath == "" {
			cliDie("Укажите путь: zapolnyaka.exe check data/myGame/game.yml")
		}
		err = cmd.ActionCheck(gamePath)

	case "emu":
		path, rest := splitPositional(os.Args[2:])
		fs := flag.NewFlagSet("emu", flag.ContinueOnError)
		port := fs.Int("port", 8090, "порт эмулятора (слушает 127.0.0.1)")
		dev := fs.Bool("dev", false, "шаблоны и статику читать с диска (правка без пересборки)")
		noOpen := fs.Bool("no-open", false, "не открывать браузер")
		offline := fs.Bool("offline", false, "CSS/JS движка из встроенной копии (без world.en.cx)")
		if perr := fs.Parse(rest); perr != nil {
			cliDie("Использование: zapolnyaka.exe emu data/myGame/game.yml [--port 8090] [--dev] [--offline] [--no-open]")
		}
		if path == "" {
			path = hist.LastGame
		}
		if path == "" {
			cliDie("Укажите путь: zapolnyaka.exe emu data/myGame/game.yml [--port 8090] [--dev]")
		}
		err = cmd.ActionEmu(path, cmd.EmuOptions{Port: *port, Dev: *dev, Offline: *offline, OpenBrowser: !*noOpen})

	case "snapshot":
		path, rest := splitPositional(os.Args[2:])
		fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
		name := fs.String("name", "snapshot", "имя снимка (файлы snapshots/<имя>.html и .json)")
		send := fs.String("send", "", "ввести этот код перед снятием")
		pid := fs.Int("pid", 0, "взять штрафную подсказку с этим HelpId перед снятием")
		pact := fs.Int("pact", 1, "pact для штрафной подсказки (1 — запрос, 2 — подтверждение)")
		if perr := fs.Parse(rest); perr != nil {
			cliDie("Использование: zapolnyaka.exe snapshot data/myGame/game.yml [--name L06-before] [--send код] [--pid id] [--pact 1]")
		}
		if path == "" {
			path = hist.LastGame
		}
		if path == "" {
			cliDie("Укажите путь: zapolnyaka.exe snapshot data/myGame/game.yml [--name L06-before]")
		}
		err = cmd.ActionSnapshot(path, cmd.SnapshotOptions{Name: *name, Send: *send, PID: *pid, Pact: *pact})

	default:
		cliDie(fmt.Sprintf("Неизвестное действие: %q\n  Доступные: %s", action, cliUsage))
	}

	if err != nil {
		logger.Println("ERROR: " + err.Error())
		fmt.Fprintln(os.Stderr, cliErrStyle.Render("  ❌ "+err.Error()))
		os.Exit(1)
	}
}

// splitPositional отделяет первый позиционный аргумент (путь к game.yml) от флагов.
func splitPositional(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func cliDie(msg string) {
	fmt.Fprintln(os.Stderr, cliErrStyle.Render("  ❌ "+msg))
	os.Exit(1)
}
