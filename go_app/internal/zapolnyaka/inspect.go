package zapolnyaka

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

// Compiled regexes for GameScenario.aspx parsing (mirrors Validator.ts logic).
var (
	// Level anchor: any <a> tag with a numeric name attribute.
	gsLevelAnchorRe = regexp.MustCompile(`(?i)<a\b[^>]*\bname="(\d+)"`)

	// Elements with _SectorsRepeater_ctlN_ in their id.
	gsSectorIdxRe  = regexp.MustCompile(`(?i)_SectorsRepeater_ctl(\d+)_`)
	gsSectorNameRe = regexp.MustCompile(`(?i)id="[^"]*_SectorsRepeater_ctl(\d+)_divSectorName[^"]*"[^>]*>([^<]*)`)
	gsSectorAnsRe  = regexp.MustCompile(`(?i)id="[^"]*_SectorsRepeater_ctl(\d+)_lblLevelAnswer[^"]*"[^>]*>([^<]*)`)

	// Elements with _LevelBonusesRepeater_ctlN_ in their id.
	gsBonusIdxRe   = regexp.MustCompile(`(?i)_LevelBonusesRepeater_ctl(\d+)_`)
	gsBonusNameRe  = regexp.MustCompile(`(?i)id="[^"]*_LevelBonusesRepeater_ctl(\d+)_lblBonusNum[^"]*"[^>]*>([^<]*)`)
	gsBonusAnsRe   = regexp.MustCompile(`(?i)id="[^"]*_LevelBonusesRepeater_ctl(\d+)_lblBonusAnswer[^"]*"[^>]*>([^<]*)`)

	// Bonus name may be wrapped in quotes: "BonusName"
	gsBonusQuoteRe = regexp.MustCompile(`"([^"]+)"`)
)

// ScrapeScenario fetches GameScenario.aspx in a single request and parses all level data.
// This mirrors the TypeScript Validator.ts logic exactly.
func (z *Zapolnyaka) ScrapeScenario() (map[int]*ScenarioLevel, error) {
	ctx := context.Background()
	u := fmt.Sprintf("https://%s/GameScenario.aspx?gid=%d", z.domain, z.gameID)
	body, err := z.client.GetPage(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("get game scenario: %w", err)
	}
	return parseGameScenario(body), nil
}

// parseGameScenario extracts per-level sector and bonus data from GameScenario.aspx HTML.
func parseGameScenario(htmlStr string) map[int]*ScenarioLevel {
	result := make(map[int]*ScenarioLevel)

	anchors := gsLevelAnchorRe.FindAllStringIndex(htmlStr, -1)
	anchorNums := gsLevelAnchorRe.FindAllStringSubmatch(htmlStr, -1)
	if len(anchors) == 0 {
		return result
	}

	for i, anchorNum := range anchorNums {
		levelNum, _ := strconv.Atoi(anchorNum[1])
		if levelNum <= 0 {
			continue
		}

		// HTML section for this level: from this anchor to the next.
		start := anchors[i][1]
		end := len(htmlStr)
		if i+1 < len(anchors) {
			end = anchors[i+1][0]
		}
		section := htmlStr[start:end]

		lvl := &ScenarioLevel{LevelNum: levelNum}

		// ── Sectors ──────────────────────────────────────────────────────────
		sectorIndices := uniqueCtlIndices(gsSectorIdxRe.FindAllStringSubmatch(section, -1))
		lvl.SectorCount = len(sectorIndices)

		sectorNameMap := make(map[int]string)
		for _, m := range gsSectorNameRe.FindAllStringSubmatch(section, -1) {
			idx, _ := strconv.Atoi(m[1])
			if _, seen := sectorNameMap[idx]; !seen {
				sectorNameMap[idx] = strings.TrimSpace(html.UnescapeString(m[2]))
			}
		}

		sectorAnsMap := make(map[int]string)
		for _, m := range gsSectorAnsRe.FindAllStringSubmatch(section, -1) {
			idx, _ := strconv.Atoi(m[1])
			if _, seen := sectorAnsMap[idx]; !seen {
				sectorAnsMap[idx] = strings.TrimSpace(html.UnescapeString(m[2]))
			}
		}

		sort.Ints(sectorIndices)
		for _, idx := range sectorIndices {
			lvl.SectorNames = append(lvl.SectorNames, sectorNameMap[idx])
			lvl.SectorFirstAnswers = append(lvl.SectorFirstAnswers, sectorAnsMap[idx])
		}

		// ── Bonuses ──────────────────────────────────────────────────────────
		bonusIndices := uniqueCtlIndices(gsBonusIdxRe.FindAllStringSubmatch(section, -1))
		lvl.BonusCount = len(bonusIndices)

		bonusNameMap := make(map[int]string)
		for _, m := range gsBonusNameRe.FindAllStringSubmatch(section, -1) {
			idx, _ := strconv.Atoi(m[1])
			if _, seen := bonusNameMap[idx]; !seen {
				text := strings.TrimSpace(html.UnescapeString(m[2]))
				// Bonus name is rendered as «"BonusName"» on the page.
				if qm := gsBonusQuoteRe.FindStringSubmatch(text); qm != nil {
					text = qm[1]
				}
				bonusNameMap[idx] = text
			}
		}

		bonusAnsMap := make(map[int]string)
		for _, m := range gsBonusAnsRe.FindAllStringSubmatch(section, -1) {
			idx, _ := strconv.Atoi(m[1])
			if _, seen := bonusAnsMap[idx]; !seen {
				bonusAnsMap[idx] = strings.TrimSpace(html.UnescapeString(m[2]))
			}
		}

		sort.Ints(bonusIndices)
		for _, idx := range bonusIndices {
			lvl.BonusNames = append(lvl.BonusNames, bonusNameMap[idx])
			lvl.BonusFirstAnswers = append(lvl.BonusFirstAnswers, bonusAnsMap[idx])
		}

		result[levelNum] = lvl
	}

	return result
}

// uniqueCtlIndices collects unique integer repeater indices from regex match groups.
func uniqueCtlIndices(matches [][]string) []int {
	seen := make(map[int]bool)
	var result []int
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		idx, _ := strconv.Atoi(m[1])
		if !seen[idx] {
			seen[idx] = true
			result = append(result, idx)
		}
	}
	return result
}
