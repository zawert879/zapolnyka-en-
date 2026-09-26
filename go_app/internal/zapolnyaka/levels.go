package zapolnyaka

import (
	"context"
	"fmt"
	"zapolnyaka/pkg/logger"
)

// EnsureLevels досоздаёт уровни в игре, если конфиг ссылается на номер больше,
// чем есть в админке (уровни en.cx нумеруются подряд с 1). После создания
// перечитывает карту «номер → ID».
func (z *Zapolnyaka) EnsureLevels(maxNum int) error {
	have := len(z.levelDbIds)
	if maxNum <= have {
		return nil
	}
	ctx := context.Background()
	missing := maxNum - have
	logger.Printf("  в игре %d уровней, в конфиге до %d — создаю %d\n", have, maxNum, missing)
	if err := z.client.AdminCreateLevels(ctx, z.gameID, missing); err != nil {
		return fmt.Errorf("create levels: %w", err)
	}
	levels, err := z.client.AdminGetLevels(ctx, z.gameID)
	if err != nil {
		return fmt.Errorf("get levels after create: %w", err)
	}
	for _, l := range levels {
		z.levelDbIds[l.Number] = l.ID
	}
	if len(levels) < maxNum {
		return fmt.Errorf("после создания в игре %d уровней, нужно %d — проверьте LevelManager.aspx", len(levels), maxNum)
	}
	logger.Printf("  уровней в игре: %d\n", len(levels))
	return nil
}

// LevelCount — сколько уровней в игре по данным админки (после Auth).
func (z *Zapolnyaka) LevelCount() int { return len(z.levelDbIds) }
