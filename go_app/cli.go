package main

// cli.go — argument parsing layer, NO huh/bubbletea imports.
// Dispatches to cmd.Action*; on completion the process exits.

import (
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

// runCLI parses os.Args, runs the requested action, then exits.
// The TUI menu is never initialized in this path.
func runCLI() {
	action := os.Args[1]
	hist := cmd.LoadHistory()
	gamePath := hist.LastGame
	if len(os.Args) > 2 {
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

	default:
		cliDie(fmt.Sprintf(
			"Неизвестное действие: %q\n  Доступные: auth <login> <pass> | go [game.yml] | assets [game.yml] | validate [game.yml] | check [game.yml] | version",
			action,
		))
	}

	if err != nil {
		logger.Println("ERROR: " + err.Error())
		fmt.Fprintln(os.Stderr, cliErrStyle.Render("  ❌ "+err.Error()))
		os.Exit(1)
	}
}

func cliDie(msg string) {
	fmt.Fprintln(os.Stderr, cliErrStyle.Render("  ❌ "+msg))
	os.Exit(1)
}
