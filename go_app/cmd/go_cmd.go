package cmd

import (
	"fmt"
	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"
	"zapolnyaka/pkg/logger"
)

// RunGo uploads all levels from the given game config file.
func RunGo(gamePath string) error {
	login, password, err := resolveCredentials()
	if err != nil {
		return err
	}

	game, prepared, err := config.LoadAll(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(prepared) == 0 {
		return fmt.Errorf("в конфиге %s нет уровней — добавьте их командой 'Добавить уровень'", gamePath)
	}

	// Rewrite {{asset}} placeholders in level content to their uploaded URLs.
	manifest, err := loadManifest(manifestPath(assetsDirFor(gamePath, game)))
	if err != nil {
		return fmt.Errorf("load asset manifest: %w", err)
	}
	if err := substituteAssets(prepared, manifest, game.GameID); err != nil {
		return err
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
