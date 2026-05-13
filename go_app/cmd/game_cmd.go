package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zapolnyaka/pkg/logger"
)

// RunGame creates data/{name}/game.yml with a commented template.
func RunGame(name, domain string, gameID int) error {
	path := filepath.Join("data", name, "game.yml")

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("файл уже существует: %s", path)
	}

	var content string
	content = strings.Join([]string{
		"# подключение (логин/пароль — в .env: EN_LOGIN / EN_PASSWORD)",
		fmt.Sprintf("domain: %s", domain),
		fmt.Sprintf("gameId: %d                # id игры в en.cx", gameID),
		"",
		"# формат по умолчанию для команды «Добавить уровень» (json | yml)",
		"defaultFormat: yml",
		"",
		"# уровни — относительные пути к conf.*, порядок = порядок заливки",
		"levels: []",
		"",
	}, "\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	logger.Printf("Создан конфиг игры: %s\n", path)
	if gameID == 0 {
		logger.Println("  gameId не задан — укажи в файле")
	}
	return nil
}
