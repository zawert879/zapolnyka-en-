package zapolnyaka

import "fmt"

// setTaskBody uploads the HTML task body to the level.
func (z *Zapolnyaka) setTaskBody(level int, html string) error {
	url := z.url(fmt.Sprintf(
		"Administration/Games/TaskEdit.aspx?gid=%d&level=%d",
		z.gameID, level,
	))
	if err := z.gotoSafe(url); err != nil {
		return err
	}
	// Disable "replace newlines with <br>" to preserve HTML structure.
	if _, err := z.page.Eval(`() => {
		const cb = document.querySelector('input[name="chkReplaceNlToBr"]');
		if (cb && cb.checked) cb.click();
	}`); err != nil {
		return fmt.Errorf("uncheck ReplaceNlToBr: %w", err)
	}
	if err := z.setFieldsViaDom(map[string]string{"inputTask": html}); err != nil {
		return err
	}
	return z.submitForm(`input[name="btnAdd"]`)
}
