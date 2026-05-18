package zapolnyaka

import (
	"context"
	"zapolnyaka/encx"
	"zapolnyaka/pkg/utils"
)

// setLevelNameAndComment saves the level name and/or comment.
func (z *Zapolnyaka) setLevelNameAndComment(ctx context.Context, level int, name, comment *string) error {
	n := ""
	if name != nil {
		n = *name
	}
	c := ""
	if comment != nil {
		c = *comment
	}
	return z.client.AdminUpdateComment(ctx, z.gameID, level, n, c)
}

// setAutopass configures (or disables when seconds==0) the autopass timer.
func (z *Zapolnyaka) setAutopass(ctx context.Context, level, seconds int, penaltySeconds *int) error {
	h, m, s := utils.SecondsToHMS(seconds)
	settings := encx.AdminLevelSettings{
		AutopassHours:   h,
		AutopassMinutes: m,
		AutopassSeconds: s,
	}
	if penaltySeconds != nil && *penaltySeconds > 0 {
		ph, pm, ps := utils.SecondsToHMS(*penaltySeconds)
		settings.TimeoutPenalty = true
		settings.PenaltyHours = ph
		settings.PenaltyMinutes = pm
		settings.PenaltySeconds = ps
	}
	return z.client.AdminUpdateAutopass(ctx, z.gameID, level, settings)
}

// setSectorsToClose sets the sector completion threshold for the level.
func (z *Zapolnyaka) setSectorsToClose(ctx context.Context, level, required int) error {
	return z.client.AdminSetSectorsToClose(ctx, z.gameID, level, required)
}
