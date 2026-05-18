package zapolnyaka

import (
	"context"
	"fmt"
	"zapolnyaka/pkg/logger"
)

// Auth authenticates via the en.cx JSON API and populates level DB IDs.
func (z *Zapolnyaka) Auth() error {
	ctx := context.Background()
	resp, err := z.client.Login(ctx, z.login, z.password)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	if resp.Error != 0 {
		return fmt.Errorf("login failed: %s", resp.Message)
	}
	logger.Printf("  auth: авторизация успешна\n")

	levels, err := z.client.AdminGetLevels(ctx, z.gameID)
	if err != nil {
		return fmt.Errorf("get levels: %w", err)
	}
	for _, l := range levels {
		z.levelDbIds[l.Number] = l.ID
	}
	logger.Printf("  auth: получено уровней: %d\n", len(levels))
	return nil
}
