package zapolnyaka

import (
	"context"
	"zapolnyaka/encx"
)

// setTaskBody uploads the HTML task body to the level.
func (z *Zapolnyaka) setTaskBody(ctx context.Context, level int, html string) error {
	return z.client.AdminCreateTask(ctx, z.gameID, level, encx.AdminTask{
		Text:      html,
		ReplaceNl: false,
	})
}
