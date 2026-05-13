package zapolnyaka

import (
	"fmt"
	"regexp"
	"strings"
)

type hintRef struct {
	prid    string
	penalty bool
}

// cleanLevel deletes all tasks, bonuses, sectors, hints and disables autopass.
func (z *Zapolnyaka) cleanLevel(level int) error {
	gid := z.gameID

	// Delete tasks
	taskIDs, err := z.listIDs(
		z.url(fmt.Sprintf("Administration/Games/TaskEdit.aspx?gid=%d&level=%d", gid, level)),
		"TaskEdit.aspx", "tid",
	)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	for _, tid := range taskIDs {
		u := z.url(fmt.Sprintf(
			"Administration/Games/TaskEdit.aspx?gid=%d&level=%d&action=TaskDelete&tid=%s",
			gid, level, tid,
		))
		if err := z.fetchURL(u); err != nil {
			return fmt.Errorf("delete task %s: %w", tid, err)
		}
	}

	// Delete bonuses — IDs are in javascript:GameEditor() hrefs on the full LevelEditor page.
	// BonusEdit.aspx without a bonus param returns window.close() and can't be scraped directly.
	bonusIDs, err := z.listIDs(
		z.url(fmt.Sprintf("Administration/Games/LevelEditor.aspx?gid=%d&level=%d", gid, level)),
		"BonusEdit.aspx", "bonus",
	)
	if err != nil {
		return fmt.Errorf("list bonuses: %w", err)
	}
	for _, bid := range bonusIDs {
		u := z.url(fmt.Sprintf(
			"Administration/Games/BonusEdit.aspx?gid=%d&level=%d&bonus=%s&action=delete",
			gid, level, bid,
		))
		if err := z.fetchURL(u); err != nil {
			return fmt.Errorf("delete bonus %s: %w", bid, err)
		}
	}

	// Delete sectors
	sectorIDs, err := z.listSectorIDs(level)
	if err != nil {
		return fmt.Errorf("list sectors: %w", err)
	}
	for _, sid := range sectorIDs {
		u := z.url(fmt.Sprintf(
			"Administration/Games/LevelEditor.aspx?gid=%d&level=%d&swanswers=1&delsector=%s",
			gid, level, sid,
		))
		if err := z.fetchURL(u); err != nil {
			return fmt.Errorf("delete sector %s: %w", sid, err)
		}
	}

	// Delete hints
	hints, err := z.listHints(
		z.url(fmt.Sprintf("Administration/Games/PromptEdit.aspx?gid=%d&level=%d", gid, level)),
	)
	if err != nil {
		return fmt.Errorf("list hints: %w", err)
	}
	for _, h := range hints {
		penalty := ""
		if h.penalty {
			penalty = "&penalty=1"
		}
		u := z.url(fmt.Sprintf(
			"Administration/Games/PromptEdit.aspx?gid=%d&level=%d&prid=%s%s&action=PromptDelete",
			gid, level, h.prid, penalty,
		))
		if err := z.fetchURL(u); err != nil {
			return fmt.Errorf("delete hint %s: %w", h.prid, err)
		}
	}

	// Disable autopass
	if err := z.setAutopass(level, 0, nil); err != nil {
		return fmt.Errorf("disable autopass: %w", err)
	}

	return nil
}

// listIDs opens url and collects all unique values of the given query
// parameter from links whose href contains hrefContains (excluding action=add).
func (z *Zapolnyaka) listIDs(url, hrefContains, paramName string) ([]string, error) {
	if err := z.gotoSafe(url); err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(paramName) + `=(\d+)`)
	html, err := z.page.MustElement("body").HTML()
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var ids []string
	// Find all href attributes
	hrefRe := regexp.MustCompile(`href="([^"]*` + regexp.QuoteMeta(hrefContains) + `[^"]*)"`)
	for _, match := range hrefRe.FindAllStringSubmatch(html, -1) {
		href := match[1]
		if strings.Contains(href, "action=add") {
			continue
		}
		m := re.FindStringSubmatch(href)
		if m == nil {
			continue
		}
		id := m[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// listSectorIDs reads sector IDs from the hidden fields on LevelEditor.
func (z *Zapolnyaka) listSectorIDs(level int) ([]string, error) {
	if err := z.openLevel(level); err != nil {
		return nil, err
	}
	elements, err := z.page.Elements(`input[id^="hdnSectorNames_"]`)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`^(\d+)\s*:`)
	seen := map[string]bool{}
	var ids []string
	for _, el := range elements {
		val, err := el.Attribute("value")
		if err != nil || val == nil {
			continue
		}
		m := re.FindStringSubmatch(*val)
		if m == nil {
			continue
		}
		id := m[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// listHints scrapes the hints page and returns prid + penalty flag for each.
func (z *Zapolnyaka) listHints(url string) ([]hintRef, error) {
	if err := z.gotoSafe(url); err != nil {
		return nil, err
	}
	html, err := z.page.MustElement("body").HTML()
	if err != nil {
		return nil, err
	}
	hrefRe := regexp.MustCompile(`href="([^"]*PromptEdit\.aspx[^"]*prid=(\d+)[^"]*)"`)
	seen := map[string]bool{}
	var refs []hintRef
	for _, match := range hrefRe.FindAllStringSubmatch(html, -1) {
		href := match[1]
		prid := match[2]
		if seen[prid] {
			continue
		}
		seen[prid] = true
		refs = append(refs, hintRef{
			prid:    prid,
			penalty: strings.Contains(href, "penalty=1"),
		})
	}
	return refs, nil
}
