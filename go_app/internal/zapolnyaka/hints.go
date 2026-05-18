package zapolnyaka

import (
	"context"
	"zapolnyaka/encx"
	"zapolnyaka/pkg/utils"
)

// addHint adds a regular auto-opening hint.
func (z *Zapolnyaka) addHint(ctx context.Context, level, timeSeconds int, text string) error {
	h, m, s := utils.SecondsToHMS(timeSeconds)
	return z.client.AdminCreateHint(ctx, z.gameID, level, encx.AdminHint{
		Text:    text,
		Hours:   h,
		Minutes: m,
		Seconds: s,
	})
}

// addPenaltyHint adds a penalty-gated hint.
func (z *Zapolnyaka) addPenaltyHint(ctx context.Context, level, timeSeconds int, text string, penalty *int, comment *string) error {
	h, m, s := utils.SecondsToHMS(timeSeconds)
	hint := encx.AdminHint{
		Text:           text,
		Hours:          h,
		Minutes:        m,
		Seconds:        s,
		IsPenalty:      true,
		RequestConfirm: true,
	}
	if penalty != nil {
		ph, pm, ps := utils.SecondsToHMS(*penalty)
		hint.PenaltyHours = ph
		hint.PenaltyMinutes = pm
		hint.PenaltySeconds = ps
	}
	if comment != nil {
		hint.PenaltyComment = *comment
	}
	return z.client.AdminCreateHint(ctx, z.gameID, level, hint)
}
