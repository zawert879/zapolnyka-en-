package cmd

import (
	"fmt"
	"os"
	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"
	"zapolnyaka/pkg/logger"

	"github.com/joho/godotenv"
)

// RunGo uploads all levels from the given game config file.
func RunGo(gamePath string) error {
	hist := LoadHistory()
	login := hist.Login
	password := hist.Password

	// Fall back to .env if not stored in history
	if login == "" || password == "" {
		_ = godotenv.Load()
		login = os.Getenv("EN_LOGIN")
		password = os.Getenv("EN_PASSWORD")
	}
	if login == "" || password == "" {
		return fmt.Errorf("учётные данные не заданы — используйте пункт «Авторизоваться» или .env (EN_LOGIN / EN_PASSWORD)")
	}

	game, prepared, err := config.LoadAll(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(prepared) == 0 {
		return fmt.Errorf("в конфиге %s нет уровней — добавьте их командой 'Добавить уровень'", gamePath)
	}

	logger.Printf("📂 Игра: %s  ID: %d  Уровней: %d\n", game.Domain, game.GameID, len(prepared))
	z, err := zapolnyaka.New(login, password, game.Domain, game.GameID, game.Delays)
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}

	logger.Println("🔑 Авторизация...")
	if err := z.Auth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	for _, p := range prepared {
		if err := z.ProcessLevel(p); err != nil {
			return fmt.Errorf("level %d: %w", p.Conf.Level, err)
		}
	}

	logger.Println("\n🎉 Все уровни залиты!")
	return nil
}
