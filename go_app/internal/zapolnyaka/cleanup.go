package zapolnyaka

import (
	"context"
	"fmt"
	"zapolnyaka/encx"
)

// cleanLevel deletes all tasks, bonuses, sectors, hints and disables autopass.
func (z *Zapolnyaka) cleanLevel(ctx context.Context, level int) error {
	// Delete tasks
	tids, err := z.client.AdminGetTaskIds(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	for _, tid := range tids {
		if err := z.client.AdminDeleteTask(ctx, z.gameID, level, tid); err != nil {
			return fmt.Errorf("delete task %d: %w", tid, err)
		}
	}

	// Delete bonuses
	bids, err := z.client.AdminGetBonusIds(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("list bonuses: %w", err)
	}
	for _, bid := range bids {
		if err := z.client.AdminDeleteBonus(ctx, z.gameID, level, bid); err != nil {
			return fmt.Errorf("delete bonus %d: %w", bid, err)
		}
	}

	// Delete sectors
	sectors, err := z.client.AdminGetSectorAnswers(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("list sectors: %w", err)
	}
	for _, s := range sectors {
		if err := z.client.AdminDeleteSector(ctx, z.gameID, level, s.ID); err != nil {
			return fmt.Errorf("delete sector %d: %w", s.ID, err)
		}
	}

	// Delete hints (check IsPenalty to pass penalty=1 for penalty hints)
	hids, err := z.client.AdminGetHintIds(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("list hints: %w", err)
	}
	for _, hid := range hids {
		h, err := z.client.AdminGetHint(ctx, z.gameID, level, hid)
		if err != nil {
			return fmt.Errorf("get hint %d: %w", hid, err)
		}
		isPenalty := h != nil && (h.IsPenalty || h.RequestConfirm)
		if err := z.client.AdminDeleteHintFull(ctx, z.gameID, level, hid, isPenalty); err != nil {
			return fmt.Errorf("delete hint %d: %w", hid, err)
		}
	}

	// Disable autopass
	return z.client.AdminUpdateAutopass(ctx, z.gameID, level, encx.AdminLevelSettings{})
}
