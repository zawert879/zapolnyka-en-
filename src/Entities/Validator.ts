import colors from 'colors'
import type { Code } from './CodesParser'
import type { Playwright } from './Playwright'

type PreparedLevel = {
  relPath: string;
  config: { level: number; domain: string; gameId: number };
  codes: Code[];
}

type LevelData = {
  sectorCount: number;
  sectorNames: string[];          // from divSectorName elements, ordered by sector index
  sectorFirstAnswers: string[];   // first answer of each sector, ordered by sector index
  bonusCount: number;
  bonusNames: string[];           // parsed from lblBonusNum text, ordered by bonus index
  bonusFirstAnswers: string[];    // first answer of each bonus, ordered by bonus index
}

export class Validator {
  constructor(
    private playwright: Playwright,
    private domain: string,
    private gameId: number,
  ) {}

  async run(prepared: PreparedLevel[]): Promise<boolean> {
    console.log('\n=== валидация GameScenario ===')

    const page = await this.playwright.mainPage()
    await page.goto(`https://${this.domain}/GameScenario.aspx?gid=${this.gameId}`, { waitUntil: 'load' })

    const levelList = await page.evaluate((): Array<{
      levelNum: number;
      sectorCount: number;
      sectorNames: string[];
      sectorFirstAnswers: string[];
      bonusCount: number;
      bonusNames: string[];
      bonusFirstAnswers: string[];
    }> => {
      const result: Array<{
        levelNum: number;
        sectorCount: number;
        sectorNames: string[];
        sectorFirstAnswers: string[];
        bonusCount: number;
        bonusNames: string[];
        bonusFirstAnswers: string[];
      }> = []

      for (const anchor of Array.from(document.querySelectorAll('a[name]'))) {
        const levelNum = parseInt(anchor.getAttribute('name') ?? '', 10)
        if (isNaN(levelNum)) continue

        const scenarioBlock = anchor.parentElement?.parentElement?.querySelector('.scenarioBlock')
        if (!scenarioBlock) continue

        // ── Sectors ──────────────────────────────────────────────────────────
        // Count unique SectorsRepeater_ctlN indices (divSectorName absent when sector is unnamed)
        const sectorIdxSet = new Set<number>()
        const sectorNameMap = new Map<number, string>()
        const sectorAnswerMap = new Map<number, string>()

        for (const el of Array.from(scenarioBlock.querySelectorAll('[id*="_SectorsRepeater_ctl"]'))) {
          const m = el.id.match(/_SectorsRepeater_ctl(\d+)_/)
          if (!m) continue
          const idx = parseInt(m[1])
          sectorIdxSet.add(idx)

          if (el.id.includes('_divSectorName')) {
            sectorNameMap.set(idx, el.textContent?.trim() ?? '')
          }
          if (el.id.includes('_lblLevelAnswer') && !sectorAnswerMap.has(idx)) {
            // first answer wins
            sectorAnswerMap.set(idx, el.textContent?.trim() ?? '')
          }
        }

        const sectorIndices = [...sectorIdxSet].sort((a, b) => a - b)
        const sectorNames = sectorIndices.map(i => sectorNameMap.get(i) ?? '')
        const sectorFirstAnswers = sectorIndices.map(i => sectorAnswerMap.get(i) ?? '')

        // ── Bonuses ──────────────────────────────────────────────────────────
        // Bonus repeater indices are even (ctl00, ctl02, ctl04...)
        const bonusIdxSet = new Set<number>()
        const bonusNameMap = new Map<number, string>()
        const bonusAnswerMap = new Map<number, string>()

        for (const el of Array.from(scenarioBlock.querySelectorAll('[id*="_LevelBonusesRepeater_ctl"]'))) {
          const m = el.id.match(/_LevelBonusesRepeater_ctl(\d+)_/)
          if (!m) continue
          const idx = parseInt(m[1])
          bonusIdxSet.add(idx)

          if (el.id.includes('_lblBonusNum')) {
            const text = el.textContent?.trim() ?? ''
            const nameMatch = text.match(/"([^"]+)"/)
            bonusNameMap.set(idx, nameMatch ? nameMatch[1] : text)
          }
          if (el.id.includes('_lblBonusAnswer') && !bonusAnswerMap.has(idx)) {
            bonusAnswerMap.set(idx, el.textContent?.trim() ?? '')
          }
        }

        const bonusIndices = [...bonusIdxSet].sort((a, b) => a - b)
        const bonusNames = bonusIndices.map(i => bonusNameMap.get(i) ?? '')
        const bonusFirstAnswers = bonusIndices.map(i => bonusAnswerMap.get(i) ?? '')

        result.push({
          levelNum,
          sectorCount: sectorIdxSet.size,
          sectorNames,
          sectorFirstAnswers,
          bonusCount: bonusIdxSet.size,
          bonusNames,
          bonusFirstAnswers,
        })
      }

      return result
    })

    const dataMap = new Map<number, LevelData>(
      levelList.map(item => [item.levelNum, {
        sectorCount: item.sectorCount,
        sectorNames: item.sectorNames,
        sectorFirstAnswers: item.sectorFirstAnswers,
        bonusCount: item.bonusCount,
        bonusNames: item.bonusNames,
        bonusFirstAnswers: item.bonusFirstAnswers,
      }]),
    )

    let allOk = true

    for (const p of prepared) {
      const levelNum = p.config.level
      const actual = dataMap.get(levelNum)

      const expectedSectors = p.codes.filter(c =>
        ['сектор', 'секторбонус', 'секторштраф'].includes(c.type),
      )
      const expectedBonuses = p.codes.filter(c =>
        ['бонус', 'штраф', 'секторбонус', 'секторштраф'].includes(c.type),
      )

      if (!actual) {
        console.log(`  уровень ${levelNum} (${p.relPath}): ${colors.red('✗ не найден на странице')}`)
        allOk = false
        continue
      }

      // ── Count checks ─────────────────────────────────────────────────────
      const sectorOk = actual.sectorCount === expectedSectors.length
      const bonusOk = actual.bonusCount === expectedBonuses.length

      const sTag = sectorOk
        ? colors.green(`секторов ${actual.sectorCount}/${expectedSectors.length} ✓`)
        : colors.red(`секторов ${actual.sectorCount}/${expectedSectors.length} ✗`)
      const bTag = bonusOk
        ? colors.green(`бонусов ${actual.bonusCount}/${expectedBonuses.length} ✓`)
        : colors.red(`бонусов ${actual.bonusCount}/${expectedBonuses.length} ✗`)

      console.log(`  уровень ${levelNum} (${p.relPath}): ${sTag}  ${bTag}`)

      // ── Name checks (only when page shows divSectorName / lblBonusNum) ──
      if (actual.sectorNames.some(n => n !== '')) {
        const missing = expectedSectors
          .filter(c => c.sectorName)
          .filter(c => !actual.sectorNames.includes(c.sectorName!))
        if (missing.length > 0) {
          console.log(colors.red(`    отсутствуют секторы: ${missing.map(c => c.sectorName).join(', ')}`))
          allOk = false
        }
      }
      if (actual.bonusNames.some(n => n !== '')) {
        const missing = expectedBonuses
          .filter(c => c.bonusName)
          .filter(c => !actual.bonusNames.includes(c.bonusName!))
        if (missing.length > 0) {
          console.log(colors.red(`    отсутствуют бонусы: ${missing.map(c => c.bonusName).join(', ')}`))
          allOk = false
        }
      }

      // ── Answer checks (positional: sector i ↔ code i) ────────────────────
      const sectorAnswerErrors: string[] = []
      for (let i = 0; i < Math.min(actual.sectorFirstAnswers.length, expectedSectors.length); i++) {
        const expected = expectedSectors[i].answers[0]
        const found = actual.sectorFirstAnswers[i]
        if (found.toLowerCase() !== expected.toLowerCase()) {
          sectorAnswerErrors.push(`сектор ${i + 1}: ожидалось "${expected}", найдено "${found}"`)
        }
      }
      if (sectorAnswerErrors.length > 0) {
        for (const e of sectorAnswerErrors) console.log(colors.red(`    ✗ ${e}`))
        allOk = false
      }

      const bonusAnswerErrors: string[] = []
      for (let i = 0; i < Math.min(actual.bonusFirstAnswers.length, expectedBonuses.length); i++) {
        const expected = expectedBonuses[i].answers[0]
        const found = actual.bonusFirstAnswers[i]
        if (found.toLowerCase() !== expected.toLowerCase()) {
          bonusAnswerErrors.push(`бонус ${i + 1}: ожидалось "${expected}", найдено "${found}"`)
        }
      }
      if (bonusAnswerErrors.length > 0) {
        for (const e of bonusAnswerErrors) console.log(colors.red(`    ✗ ${e}`))
        allOk = false
      }

      if (!sectorOk || !bonusOk) allOk = false
    }

    if (allOk) {
      console.log(colors.green('=== валидация OK ==='))
    } else {
      console.log(colors.red('=== валидация: есть расхождения ==='))
    }

    return allOk
  }
}
