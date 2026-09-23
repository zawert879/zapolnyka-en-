package zapolnyaka

import (
	"context"
	"fmt"
	"slices"
	"zapolnyaka/encx"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"
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
	task := ""
	if code.Task != nil {
		task = *code.Task
	}
	help := ""
	if code.Help != nil {
		help = *code.Help
	}

	levelID, levelIDs, err := z.bonusLevels(level, code)
	if err != nil {
		return err
	}
	multi := levelID == -1 || len(levelIDs) > 1

	h, m, s := utils.SecondsToHMS(*code.Time)

	b := encx.AdminBonus{
		Name:         bonusName,
		Task:         task,
		Hint:         help,
		LevelID:      levelID,
		LevelIDs:     levelIDs,
		Answers:      chunks[0],
		AwardHours:   h,
		AwardMinutes: m,
		AwardSeconds: s,
		Negative:     code.Type.IsPenalty(),
	}

	if err := z.client.AdminCreateBonus(ctx, z.gameID, level, b); err != nil {
		return fmt.Errorf("create bonus: %w", err)
	}

	if len(chunks) == 1 && !multi {
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

	if multi {
		// Мультиуровневый бонус виден на страницах других уровней:
		// cleanLevel следующих уровней в этом запуске не должен его удалить.
		z.createdBonuses[bonusID] = true
	}
	if len(chunks) == 1 {
		return nil
	}

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

// bonusLevels resolves the code's `levels` spec against the game's levels.
// Returns (levelID, levelIDs): levelID == -1 means "all levels";
// non-empty levelIDs is an explicit set; otherwise the bonus plays on `level` only.
func (z *Zapolnyaka) bonusLevels(level int, code config.Code) (int, []int, error) {
	if code.Levels == nil {
		return z.levelDbIds[level], nil, nil
	}
	f, err := config.ParseLevelSpec(*code.Levels)
	if err != nil {
		return 0, nil, err
	}
	if f.All {
		logger.Printf("    уровни бонуса %q → все уровни\n", *code.Levels)
		return -1, nil, nil
	}
	nums := make([]int, 0, len(z.levelDbIds))
	for n := range z.levelDbIds {
		nums = append(nums, n)
	}
	matched := f.Resolve(nums)
	if len(matched) == 0 {
		return 0, nil, fmt.Errorf("levels %q: ни один уровень игры не подходит", *code.Levels)
	}
	ids := make([]int, 0, len(matched))
	for _, n := range matched {
		ids = append(ids, z.levelDbIds[n])
	}
	logger.Printf("    уровни бонуса %q → %v\n", *code.Levels, matched)
	return ids[0], ids, nil
}
