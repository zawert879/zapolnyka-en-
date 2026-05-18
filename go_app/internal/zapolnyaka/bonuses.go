package zapolnyaka

import (
	"context"
	"fmt"
	"slices"
	"zapolnyaka/encx"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/utils"
)

const maxAnswersPerBonus = 100

// addBonus creates a bonus/penalty via the Admin HTTP API.
func (z *Zapolnyaka) addBonus(ctx context.Context, level int, code config.Code) error {
	if code.Time == nil {
		return fmt.Errorf("addBonus: time is required")
	}
	chunks := utils.ChunkSlice(code.Answers, maxAnswersPerBonus)
	if len(chunks) == 0 {
		return nil
	}

	bonusName := ""
	if code.BonusName != nil {
		bonusName = *code.BonusName
	}
	help := ""
	if code.Help != nil {
		help = *code.Help
	}

	h, m, s := utils.SecondsToHMS(*code.Time)

	b := encx.AdminBonus{
		Name:         bonusName,
		Hint:         help,
		LevelID:      z.levelDbIds[level],
		Answers:      chunks[0],
		AwardHours:   h,
		AwardMinutes: m,
		AwardSeconds: s,
		Negative:     code.Type.IsPenalty(),
	}

	if err := z.client.AdminCreateBonus(ctx, z.gameID, level, b); err != nil {
		return fmt.Errorf("create bonus: %w", err)
	}

	if len(chunks) == 1 {
		return nil
	}

	// Fetch the ID of the newly created bonus (last in the list)
	bids, err := z.client.AdminGetBonusIds(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("get bonus ids: %w", err)
	}
	if len(bids) == 0 {
		return fmt.Errorf("no bonus IDs found after creation")
	}
	bonusID := bids[len(bids)-1]

	// Add remaining chunks by updating with the full accumulated answer list
	cur, err := z.client.AdminGetBonus(ctx, z.gameID, level, bonusID)
	if err != nil {
		return fmt.Errorf("get bonus %d: %w", bonusID, err)
	}
	all := slices.Clone(chunks[0])
	for _, chunk := range chunks[1:] {
		all = append(all, chunk...)
		cur.Answers = all
		if err := z.client.AdminUpdateBonus(ctx, z.gameID, level, bonusID, *cur); err != nil {
			return fmt.Errorf("update bonus %d: %w", bonusID, err)
		}
	}
	return nil
}
