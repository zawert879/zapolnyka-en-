package cmd

import (
	"fmt"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"

	"github.com/charmbracelet/lipgloss"
)

var actInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

// ActionGo uploads all levels and saves game path to history.
func ActionGo(gamePath string) error {
	if err := RunGo(gamePath); err != nil {
		return err
	}
	hist := LoadHistory()
	hist.LastGame = gamePath
	SaveHistory(hist)
	return nil
}

// ActionValidate prints the parsed config without a browser.
func ActionValidate(gamePath string) error {
	return RunValidate(gamePath)
}

// ActionCheck compares GameScenario.aspx with the config.
func ActionCheck(gamePath string) error {
	return RunCheck(gamePath)
}

// ActionAuth saves credentials to the history file.
func ActionAuth(login, password string) error {
	hist := LoadHistory()
	hist.Login = login
	hist.Password = password
	SaveHistory(hist)
	logger.Println("  ✔ Учётные данные сохранены")
	fmt.Println(actInfoStyle.Render("  ✔ Учётные данные сохранены в .zapolnyaka.json"))
	return nil
}

// ActionCode appends a code entry to the level's codes file.
func ActionCode(gamePath, levelRelPath string, code config.Code) error {
	if err := RunCode(gamePath, levelRelPath, code); err != nil {
		return err
	}
	hist := LoadHistory()
	hist.LastGame = gamePath
	hist.LastLevel = levelRelPath
	SaveHistory(hist)
	return nil
}
