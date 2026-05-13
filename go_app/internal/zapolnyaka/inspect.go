package zapolnyaka

import (
	"encoding/json"
	"fmt"
)

// ScenarioLevel holds scraped data for one level from GameScenario.aspx.
type ScenarioLevel struct {
	LevelNum           int      `json:"levelNum"`
	SectorCount        int      `json:"sectorCount"`
	SectorNames        []string `json:"sectorNames"`
	SectorFirstAnswers []string `json:"sectorFirstAnswers"`
	BonusCount         int      `json:"bonusCount"`
	BonusNames         []string `json:"bonusNames"`
	BonusFirstAnswers  []string `json:"bonusFirstAnswers"`
}

// ScrapeScenario opens GameScenario.aspx once and returns data for all levels.
func (z *Zapolnyaka) ScrapeScenario() (map[int]*ScenarioLevel, error) {
	url := z.url(fmt.Sprintf("GameScenario.aspx?gid=%d", z.gameID))
	if err := z.gotoSafe(url); err != nil {
		return nil, fmt.Errorf("navigate to GameScenario: %w", err)
	}

	res, err := z.page.Eval(`() => {
		const result = [];
		for (const anchor of document.querySelectorAll('a[name]')) {
			const levelNum = parseInt(anchor.getAttribute('name') ?? '', 10);
			if (isNaN(levelNum)) continue;

			const scenarioBlock = anchor.parentElement?.parentElement?.querySelector('.scenarioBlock');
			if (!scenarioBlock) continue;

			// ── Sectors ─────────────────────────────────────────────────────
			const sectorIdxSet = new Set();
			const sectorNameMap = new Map();
			const sectorAnswerMap = new Map();
			for (const el of scenarioBlock.querySelectorAll('[id*="_SectorsRepeater_ctl"]')) {
				const m = el.id.match(/_SectorsRepeater_ctl(\d+)_/);
				if (!m) continue;
				const idx = parseInt(m[1]);
				sectorIdxSet.add(idx);
				if (el.id.includes('_divSectorName'))
					sectorNameMap.set(idx, el.textContent?.trim() ?? '');
				if (el.id.includes('_lblLevelAnswer') && !sectorAnswerMap.has(idx))
					sectorAnswerMap.set(idx, el.textContent?.trim() ?? '');
			}
			const si = [...sectorIdxSet].sort((a, b) => a - b);

			// ── Bonuses ──────────────────────────────────────────────────────
			const bonusIdxSet = new Set();
			const bonusNameMap = new Map();
			const bonusAnswerMap = new Map();
			for (const el of scenarioBlock.querySelectorAll('[id*="_LevelBonusesRepeater_ctl"]')) {
				const m = el.id.match(/_LevelBonusesRepeater_ctl(\d+)_/);
				if (!m) continue;
				const idx = parseInt(m[1]);
				bonusIdxSet.add(idx);
				if (el.id.includes('_lblBonusNum')) {
					const text = el.textContent?.trim() ?? '';
					const nm = text.match(/"([^"]+)"/);
					bonusNameMap.set(idx, nm ? nm[1] : text);
				}
				if (el.id.includes('_lblBonusAnswer') && !bonusAnswerMap.has(idx))
					bonusAnswerMap.set(idx, el.textContent?.trim() ?? '');
			}
			const bi = [...bonusIdxSet].sort((a, b) => a - b);

			result.push({
				levelNum,
				sectorCount: sectorIdxSet.size,
				sectorNames:        si.map(i => sectorNameMap.get(i) ?? ''),
				sectorFirstAnswers: si.map(i => sectorAnswerMap.get(i) ?? ''),
				bonusCount: bonusIdxSet.size,
				bonusNames:        bi.map(i => bonusNameMap.get(i) ?? ''),
				bonusFirstAnswers: bi.map(i => bonusAnswerMap.get(i) ?? ''),
			});
		}
		return result;
	}`)
	if err != nil {
		return nil, fmt.Errorf("eval GameScenario: %w", err)
	}

	var levels []ScenarioLevel
	if err := json.Unmarshal([]byte(res.Value.JSON("", "")), &levels); err != nil {
		return nil, fmt.Errorf("parse scenario data: %w", err)
	}

	m := make(map[int]*ScenarioLevel, len(levels))
	for i := range levels {
		m[levels[i].LevelNum] = &levels[i]
	}
	return m, nil
}
