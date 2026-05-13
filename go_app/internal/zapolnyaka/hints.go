package zapolnyaka

import (
	"fmt"
	"zapolnyaka/pkg/utils"
)

// addHint adds a regular auto-opening hint.
func (z *Zapolnyaka) addHint(level, timeSeconds int, text string) error {
	url := z.url(fmt.Sprintf(
		"Administration/Games/PromptEdit.aspx?gid=%d&level=%d",
		z.gameID, level,
	))
	if err := z.gotoSafe(url); err != nil {
		return err
	}
	h, m, s := utils.SecondsToHMS(timeSeconds)
	if err := z.setFieldsViaDom(map[string]string{
		"NewPromptTimeoutDays":    "0",
		"NewPromptTimeoutHours":   fmt.Sprintf("%d", h),
		"NewPromptTimeoutMinutes": fmt.Sprintf("%d", m),
		"NewPromptTimeoutSeconds": fmt.Sprintf("%d", s),
		"NewPrompt":               text,
	}); err != nil {
		return err
	}
	return z.submitForm(`input[name="btnAdd"]`)
}

// addPenaltyHint adds a penalty-gated hint.
func (z *Zapolnyaka) addPenaltyHint(level, timeSeconds int, text string, penalty *int, comment *string) error {
	url := z.url(fmt.Sprintf(
		"Administration/Games/PromptEdit.aspx?gid=%d&level=%d&penalty=1",
		z.gameID, level,
	))
	if err := z.gotoSafe(url); err != nil {
		return err
	}
	h, m, s := utils.SecondsToHMS(timeSeconds)
	fields := map[string]string{
		"NewPromptTimeoutDays":    "0",
		"NewPromptTimeoutHours":   fmt.Sprintf("%d", h),
		"NewPromptTimeoutMinutes": fmt.Sprintf("%d", m),
		"NewPromptTimeoutSeconds": fmt.Sprintf("%d", s),
		"NewPrompt":               text,
	}
	if comment != nil {
		fields["txtPenaltyComment"] = *comment
	}
	if penalty != nil {
		ph, pm, ps := utils.SecondsToHMS(*penalty)
		fields["PenaltyPromptHours"] = fmt.Sprintf("%d", ph)
		fields["PenaltyPromptMinutes"] = fmt.Sprintf("%d", pm)
		fields["PenaltyPromptSeconds"] = fmt.Sprintf("%d", ps)
	}
	// Request confirmation checkbox
	if _, err := z.page.Eval(`() => {
		const cb = document.querySelector('input[name="chkRequestPenaltyConfirm"]');
		if (cb && !cb.checked) cb.click();
	}`); err != nil {
		return fmt.Errorf("enable penalty confirm: %w", err)
	}
	if err := z.setFieldsViaDom(fields); err != nil {
		return err
	}
	return z.submitForm(`input[name="btnAdd"]`)
}
