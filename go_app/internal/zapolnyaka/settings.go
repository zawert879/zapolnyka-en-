package zapolnyaka

import (
	"fmt"
	"zapolnyaka/pkg/utils"
)

// setLevelNameAndComment saves the level name and/or comment.
func (z *Zapolnyaka) setLevelNameAndComment(level int, name, comment *string) error {
	url := z.url(fmt.Sprintf(
		"Administration/Games/NameCommentEdit.aspx?gid=%d&level=%d",
		z.gameID, level,
	))
	if err := z.gotoSafe(url); err != nil {
		return err
	}
	fields := map[string]string{}
	if name != nil {
		fields["txtLevelName"] = *name
	}
	if comment != nil {
		fields["txtLevelComment"] = *comment
	}
	if len(fields) == 0 {
		return nil
	}
	if err := z.setFieldsViaDom(fields); err != nil {
		return err
	}
	return z.submitForm(`input[name="btnUpdate"]`)
}

// setAutopass configures (or disables when seconds==0) the autopass timer.
func (z *Zapolnyaka) setAutopass(level, seconds int, penaltySeconds *int) error {
	url := z.url(fmt.Sprintf(
		"Administration/Games/LevelEditor.aspx?gid=%d&level=%d&swautopass=1",
		z.gameID, level,
	))
	if err := z.gotoSafe(url); err != nil {
		return err
	}

	h, m, s := utils.SecondsToHMS(seconds)
	fields := map[string]string{
		"txtApHours":   fmt.Sprintf("%d", h),
		"txtApMinutes": fmt.Sprintf("%d", m),
		"txtApSeconds": fmt.Sprintf("%d", s),
	}

	if penaltySeconds != nil && *penaltySeconds > 0 {
		ph, pm, ps := utils.SecondsToHMS(*penaltySeconds)
		// Enable the penalty checkbox first, then set the values
		if _, err := z.page.Eval(`() => {
			const cb = document.querySelector('input[name="chkTimeoutPenalty"]');
			if (cb && !cb.checked) cb.click();
		}`); err != nil {
			return fmt.Errorf("enable penalty checkbox: %w", err)
		}
		fields["txtApPenaltyHours"] = fmt.Sprintf("%d", ph)
		fields["txtApPenaltyMinutes"] = fmt.Sprintf("%d", pm)
		fields["txtApPenaltySeconds"] = fmt.Sprintf("%d", ps)
	}

	if err := z.setFieldsViaDom(fields); err != nil {
		return err
	}
	return z.submitForm(`form:has(input[name="txtApHours"]) input[type="image"][alt="Сохранить"]`)
}

// setSectorsToClose sets the sector completion threshold for the level.
// required > 0 means "close N sectors"; 0 means "close all".
func (z *Zapolnyaka) setSectorsToClose(level, required int) error {
	if err := z.openLevel(level); err != nil {
		return err
	}
	radioVal := "1" // all sectors
	if required > 0 {
		radioVal = "2"
	}
	if _, err := z.page.Eval(`(v, n) => {
		const radios = document.querySelectorAll('input[name="rbSectorCompleteType"]');
		for (const r of radios) { if (r.value === v) { r.checked = true; break; } }
		if (v === '2') {
			const cnt = document.querySelector('input[name="txtRequiredSectorsCount"]');
			if (cnt) cnt.value = n;
		}
	}`, radioVal, fmt.Sprintf("%d", required)); err != nil {
		return err
	}
	return z.submitForm(`input[name="pnlSettings_SectorsCompletionSettings_ctl00_btnSave"]`)
}
