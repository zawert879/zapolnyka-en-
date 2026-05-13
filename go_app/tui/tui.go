// Package tui contains the interactive terminal menu.
// It is only imported and called when the binary runs without CLI arguments.
package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"zapolnyaka/cmd"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

func isAborted(err error) bool { return err != nil && errors.Is(err, huh.ErrUserAborted) }

func newForm(groups ...*huh.Group) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc/ctrl+c", "назад"))
	return huh.NewForm(groups...).WithKeyMap(km).WithOutput(os.Stdout).WithInput(os.Stdin).
		WithProgramOptions(tea.WithAltScreen())
}

func newInlineForm(groups ...*huh.Group) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc/ctrl+c", "назад"))
	return huh.NewForm(groups...).WithKeyMap(km).WithOutput(os.Stdout).WithInput(os.Stdin)
}

// Run is the entry point for TUI mode. Called only when no CLI args are present.
func Run() {
	fmt.Println(titleStyle.Render("\n  🗂  zapolnyaka-en — автозаполнение уровней Encounter\n"))

	var lastErr error
	for {
		if lastErr != nil {
			fmt.Println(errStyle.Render("  ╔══════════════════════════════════════════╗"))
			fmt.Println(errStyle.Render("  ║  ❌ ОШИБКА:                              ║"))
			for _, line := range strings.Split(lastErr.Error(), "\n") {
				fmt.Println(errStyle.Render(fmt.Sprintf("  ║  %s", line)))
			}
			fmt.Println(errStyle.Render("  ╚══════════════════════════════════════════╝"))
			fmt.Println()
			lastErr = nil
		}

		var action string
		if err := newInlineForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Выберите действие").Options(
				huh.NewOption("🚀  Залить уровни", "go"),
				huh.NewOption("📋  Проверить конфиги", "validate"),
				huh.NewOption("🔍  Проверить залитое", "check"),
				huh.NewOption("➕  Добавить код в уровень", "code"),
				huh.NewOption("🎮  Создать конфиг игры", "game"),
				huh.NewOption("📝  Добавить уровень", "level"),
				huh.NewOption("🔑  Авторизоваться", "auth"),
				huh.NewOption("🧪  Тест (devLevel)", "selftest"),
				huh.NewOption("🚪  Выход", "exit"),
			).Value(&action),
		)).Run(); err != nil {
			break
		}

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = dispatch(action)
		}()
		if err != nil && !isAborted(err) {
			logger.Println("ERROR: " + err.Error())
			lastErr = err
		}
		fmt.Println()
	}
}

func dispatch(action string) error {
	switch action {
	case "go":
		return tuiGo()
	case "validate":
		return tuiValidate()
	case "check":
		return tuiCheck()
	case "code":
		return tuiCode()
	case "game":
		return tuiGame()
	case "level":
		return tuiLevel()
	case "auth":
		return tuiAuth()
	case "selftest":
		if err := cmd.ActionSelfTest(); err != nil {
			return err
		}
		return pause()
	case "exit":
		fmt.Println("До свидания!")
		os.Exit(0)
	}
	return nil
}

func tuiGo() error {
	hist := cmd.LoadHistory()
	gamePath, err := pickGame(hist)
	if err != nil || gamePath == "" {
		return err
	}
	fmt.Println()
	if err := cmd.ActionValidate(gamePath); err != nil {
		return err
	}
	var proceed bool
	if err := newInlineForm(huh.NewGroup(
		huh.NewConfirm().Title("Залить эти уровни?").Affirmative("Залить").Negative("Отмена").Value(&proceed),
	)).Run(); err != nil || !proceed {
		return err
	}
	return cmd.ActionGo(gamePath)
}

func tuiValidate() error {
	hist := cmd.LoadHistory()
	gamePath, err := pickGame(hist)
	if err != nil || gamePath == "" {
		return err
	}
	fmt.Println()
	if err := cmd.ActionValidate(gamePath); err != nil {
		return err
	}
	return pause()
}

func tuiCheck() error {
	hist := cmd.LoadHistory()
	gamePath, err := pickGame(hist)
	if err != nil || gamePath == "" {
		return err
	}
	fmt.Println()
	if err := cmd.ActionCheck(gamePath); err != nil {
		return err
	}
	return pause()
}

func tuiCode() error {
	hist := cmd.LoadHistory()
	gamePath, err := pickGame(hist)
	if err != nil || gamePath == "" {
		return err
	}
	levelPath, err := pickLevel(gamePath, hist)
	if err != nil || levelPath == "" {
		return err
	}
	var codeType string
	if err := newForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Тип кода").Options(
			huh.NewOption("сектор", "сектор"),
			huh.NewOption("бонус  (−время)", "бонус"),
			huh.NewOption("штраф  (+время)", "штраф"),
			huh.NewOption("секторбонус  (сектор + бонус)", "секторбонус"),
			huh.NewOption("секторштраф  (сектор + штраф)", "секторштраф"),
		).Value(&codeType),
	)).Run(); err != nil {
		return err
	}
	code, err := collectCodeFields(config.CodeType(codeType))
	if err != nil {
		return err
	}
	return cmd.ActionCode(gamePath, levelPath, code)
}

func tuiGame() error {
	var name, domain, gameIDStr string
	existing := cmd.ScanGames("data")
	if len(existing) > 0 {
		names := make([]string, len(existing))
		for i, g := range existing {
			names[i] = filepath.Base(filepath.Dir(g))
		}
		fmt.Println(infoStyle.Render("  Существующие игры: " + strings.Join(names, ", ")))
	}
	if err := newForm(huh.NewGroup(
		huh.NewInput().Title("Имя игры (создаст data/{имя}/game.yml)").Placeholder("myGame").Value(&name).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("имя обязательно")
				}
				return nil
			}),
		huh.NewInput().Title("Домен платформы").Placeholder("tech.en.cx").Value(&domain),
		huh.NewInput().Title("ID игры на en.cx (можно оставить 0)").Placeholder("82000").Value(&gameIDStr),
	)).Run(); err != nil {
		return err
	}
	if domain == "" {
		domain = "tech.en.cx"
	}
	gameID, _ := strconv.Atoi(gameIDStr)
	return cmd.RunGame(strings.TrimSpace(name), domain, gameID)
}

func tuiLevel() error {
	hist := cmd.LoadHistory()
	gamePath, err := pickGame(hist)
	if err != nil || gamePath == "" {
		return err
	}
	gameDir := filepath.Dir(gamePath)
	existing := cmd.ScanLevelFolders(gameDir)
	if len(existing) > 0 {
		fmt.Println(infoStyle.Render("  Существующие папки: " + strings.Join(existing, ", ")))
	}
	var dir, levelStr string
	if err := newForm(huh.NewGroup(
		huh.NewInput().Title("Имя новой папки").Placeholder("5").Value(&dir).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("имя папки обязательно")
				}
				return nil
			}),
		huh.NewInput().Title("Номер уровня на en.cx (пустой = имя папки)").Value(&levelStr),
	)).Run(); err != nil {
		return err
	}
	levelNum, _ := strconv.Atoi(levelStr)
	if err := cmd.RunLevel(gamePath, strings.TrimSpace(dir), levelNum, ""); err != nil {
		return err
	}
	hist.LastGame = gamePath
	cmd.SaveHistory(hist)
	return nil
}

func tuiAuth() error {
	hist := cmd.LoadHistory()
	login, password := hist.Login, hist.Password
	if err := newForm(huh.NewGroup(
		huh.NewInput().Title("Логин (en.cx)").Placeholder("логин или id").Value(&login).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("логин обязателен")
				}
				return nil
			}),
		huh.NewInput().Title("Пароль").Password(true).Value(&password).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("пароль обязателен")
				}
				return nil
			}),
	)).Run(); err != nil {
		return err
	}
	return cmd.ActionAuth(strings.TrimSpace(login), password)
}

func pause() error {
	var ok bool
	_ = newInlineForm(huh.NewGroup(
		huh.NewConfirm().Title("").Affirmative("ОК").Negative("").Value(&ok),
	)).Run()
	return nil
}

func pickGame(hist cmd.History) (string, error) {
	games := cmd.ScanGames("data")
	if len(games) == 0 {
		val := hist.LastGame
		err := newForm(huh.NewGroup(
			huh.NewInput().Title("Путь к game.yml").Placeholder("data/myGame/game.yml").Value(&val),
		)).Run()
		return val, err
	}
	selected := hist.LastGame
	found := false
	for _, g := range games {
		if g == selected {
			found = true
			break
		}
	}
	if !found {
		selected = games[0]
	}
	opts := make([]huh.Option[string], len(games))
	for i, g := range games {
		opts[i] = huh.NewOption(g, g)
	}
	err := newForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Выберите игру").Options(opts...).Value(&selected),
	)).Run()
	return selected, err
}

func pickLevel(gamePath string, hist cmd.History) (string, error) {
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return "", err
	}
	if len(game.Levels) == 0 {
		return "", fmt.Errorf("в игре %s нет уровней — сначала добавь уровень", gamePath)
	}
	selected := hist.LastLevel
	found := false
	for _, l := range game.Levels {
		if l == selected {
			found = true
			break
		}
	}
	if !found {
		selected = game.Levels[0]
	}
	opts := make([]huh.Option[string], len(game.Levels))
	for i, l := range game.Levels {
		opts[i] = huh.NewOption(l, l)
	}
	err = newForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Выберите уровень").Options(opts...).Value(&selected),
	)).Run()
	return selected, err
}

func collectCodeFields(ct config.CodeType) (config.Code, error) {
	code := config.Code{Type: ct}
	var sectorName, bonusName, answersStr, timeStr, helpStr string
	var fields []huh.Field
	if ct.HasSector() {
		fields = append(fields, huh.NewInput().Title("Имя сектора").Value(&sectorName))
	}
	if ct.HasBonus() {
		fields = append(fields, huh.NewInput().Title("Имя бонуса").Value(&bonusName))
	}
	fields = append(fields,
		huh.NewInput().Title("Ответы (через запятую)").Placeholder("ответ1, ответ2").Value(&answersStr).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("минимум один ответ")
				}
				return nil
			}),
	)
	if ct.HasBonus() {
		fields = append(fields,
			huh.NewInput().Title("Время (секунды)").Placeholder("300").Value(&timeStr).
				Validate(func(s string) error {
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("введите целое число секунд")
					}
					return nil
				}),
			huh.NewInput().Title("Подсказка (help) — опционально").Value(&helpStr),
		)
	}
	if err := newForm(huh.NewGroup(fields...)).Run(); err != nil {
		return code, err
	}
	if sectorName != "" {
		code.SectorName = &sectorName
	}
	if bonusName != "" {
		code.BonusName = &bonusName
	}
	for _, a := range strings.Split(answersStr, ",") {
		if a = strings.TrimSpace(a); a != "" {
			code.Answers = append(code.Answers, a)
		}
	}
	if timeStr != "" {
		t, _ := strconv.Atoi(timeStr)
		code.Time = &t
	}
	if helpStr != "" {
		code.Help = &helpStr
	}
	return code, nil
}
