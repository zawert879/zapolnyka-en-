/* eslint-disable no-await-in-loop */
import ProgressBar from 'progress'
import { type Playwright } from './Playwright'
import { type Page } from '@playwright/test'
import { chunkArray, timeout } from '../utils'
import { type Code } from './CodesParser'
import { type Data } from '../data'

function secondsToHms(total: number): { h: number; m: number; s: number } {
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return { h, m, s }
}

export class Zapolnyaka {
  private _playwright: Playwright
  private _config: Data.Config.Self
  constructor(playwright: Playwright, config: Data.Config.Self) {
    this._playwright = playwright
    this._config = config
  }

  async auth() {
    const page = await this._playwright.mainPage()
    await page.goto(`https://${this._config.domain}/Login.aspx?return=%2fDefault.aspx&lang=ru`)
    await page.getByPlaceholder('логин или id').fill(this._config.login)
    await page.getByPlaceholder('пароль').fill(this._config.password)
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'load' }),
      page.getByRole('button', { name: 'Вход' }).click(),
    ])
  }

  private async gotoSafe(page: Page, url: string): Promise<void> {
    const MAX_RETRIES = 3
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      await page.goto(url)
      if (!page.url().includes('NotHumanRequest')) return

      console.warn(`\n⚠ бот-детект — логинимся и ждём 30 мин (попытка ${attempt}/${MAX_RETRIES})`)
      await this.auth()
      await timeout(30 * 60 * 1000)
    }

    throw new Error(`Бот-детект: не удалось открыть ${url} после ${MAX_RETRIES} попыток`)
  }

  async openLevel(level: number): Promise<Page> {
    const page = await this._playwright.mainPage()
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/LevelEditor.aspx?gid=${this._config.gameId}&level=${level}&swanswers=1`)
    return page
  }

  async processLevel(codes: Code[], body?: string) {
    const { level } = this._config
    let page = await this.openLevel(level)

    if (this._config.clean) {
      await this.cleanLevel(page)
      await timeout(1000)
    }

    if (this._config.name !== undefined || this._config.comment !== undefined) {
      await this.setLevelNameAndComment(page, this._config.name, this._config.comment)
      await timeout(1000)
    }

    if (this._config.autopass !== undefined) {
      await this.setAutopass(page, this._config.autopass, this._config.autopassPenalty)
      await timeout(1000)
    }

    if (body !== undefined) {
      await this.setTaskBody(page, body)
      await timeout(1000)
    }

    if (this._config.hints) {
      for (const hint of this._config.hints) {
        await this.addHint(page, hint.time, hint.text)
        await timeout(1000)
      }
    }

    if (this._config.penaltyHints) {
      for (const hint of this._config.penaltyHints) {
        await this.addPenaltyHint(page, hint.time, hint.text, hint.penalty, hint.comment)
        await timeout(1000)
      }
    }

    // Re-open LevelEditor so subsequent sector/bonus actions find the expected DOM
    page = await this.openLevel(level)
    await timeout(500)

    const bar = new ProgressBar('progress: [:bar] :current/:total :percent :etas', {
      total: codes.length,
      width: 30,
      complete: '=',
      incomplete: '-',
    })

    for (let i = 0; i < codes.length; i++) {
      const code = codes[i]
      const isSector = code.type === 'сектор' || code.type === 'секторбонус' || code.type === 'секторштраф'
      const isBonus = code.type !== 'сектор'
      const isPenalty = code.type === 'штраф' || code.type === 'секторштраф'

      if (isSector) {
        await this.addAnswersToSector(page, code.sectorName ?? '', code.answers)
      }

      if (isBonus) {
        const t = code.time ?? 0
        const hours = Math.floor(t / 3600)
        const minutes = Math.floor((t % 3600) / 60)
        const seconds = t % 60
        await this.addBonus({
          page,
          sector: code.bonusName ?? '',
          answers: code.answers,
          time: { hours, minutes, seconds },
          isNegative: isPenalty,
          level,
          helpBody: code.help,
        })
        await timeout(1000)
      }

      bar.tick()
      await timeout(2000)
    }

    if (this._config.sectorsToClose !== undefined) {
      await this.setSectorsToClose(page, this._config.sectorsToClose)
      await timeout(1000)
    }
  }

  // Set "Название уровня" и/или "Комментарий к уровню"
  async setLevelNameAndComment(page: Page, name?: string, comment?: string) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/NameCommentEdit.aspx?gid=${gameId}&level=${level}`)
    await page.waitForSelector('input[name="txtLevelName"]')

    await page.evaluate(({ n, c }: { n?: string; c?: string }) => {
      if (n !== undefined) {
        const el = document.querySelector('input[name="txtLevelName"]') as HTMLInputElement | null
        if (el) el.value = n
      }

      if (c !== undefined) {
        const el = document.querySelector('textarea[name="txtLevelComment"]') as HTMLTextAreaElement | null
        if (el) el.value = c
      }
    }, { n: name, c: comment })

    await this.submitForm(page, 'input[name="btnUpdate"]')
  }

  // Set "Условие прохождения": how many sectors must be closed to complete the level.
  // Pass a positive N for «выполнить N секторов», or 0/undefined-path (handled by caller) for «все».
  async setSectorsToClose(page: Page, required: number) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/LevelEditor.aspx?gid=${gameId}&level=${level}&swanswers=1`)
    await page.waitForSelector('input[name="txtRequiredSectorsCount"]', { state: 'attached' })

    await page.evaluate((n: number) => {
      const input = document.querySelector('input[name="txtRequiredSectorsCount"]') as HTMLInputElement | null
      const radios = Array.from(document.querySelectorAll('input[name="rbSectorCompleteType"]')) as HTMLInputElement[]
      const rbAll = radios.find(r => r.value === '1')
      const rbCustom = radios.find(r => r.value === '2')
      if (!input || !rbAll || !rbCustom) return

      if (n <= 0) {
        rbAll.checked = true
        rbCustom.checked = false
      } else {
        rbAll.checked = false
        rbCustom.checked = true
        input.removeAttribute('disabled')
        input.value = String(n)
        input.dispatchEvent(new Event('input', { bubbles: true }))
        input.dispatchEvent(new Event('change', { bubbles: true }))
      }
    }, required)

    await this.submitForm(page, 'input[name="pnlSettings_SectorsCompletionSettings_ctl00_btnSave"]')
  }

  async cleanLevel(page: Page) {
    const { gameId, level } = this._config
    const base = `https://${this._config.domain}`
    const editorUrl = `${base}/Administration/Games/LevelEditor.aspx?gid=${gameId}&level=${level}`

    // 1. Delete tasks
    const tids = await this.listIds(page, editorUrl, 'TaskEdit.aspx', 'tid')
    console.log(`cleanLevel: deleting ${tids.length} tasks`)
    for (const tid of tids) {
      await this.fetchUrl(page, `${base}/Administration/Games/TaskEdit.aspx?gid=${gameId}&level=${level}&action=TaskDelete&tid=${tid}`)
      await timeout(1000)
    }

    // 2. Delete bonuses
    const bids = await this.listIds(page, editorUrl, 'BonusEdit.aspx', 'bonus')
    console.log(`cleanLevel: deleting ${bids.length} bonuses`)
    for (const bid of bids) {
      await this.fetchUrl(page, `${base}/Administration/Games/BonusEdit.aspx?gid=${gameId}&level=${level}&bonus=${bid}&action=delete`)
      await timeout(1000)
    }

    // 3. Delete sectors
    const sids = await this.listSectorIds(page)
    console.log(`cleanLevel: deleting ${sids.length} sectors`)
    for (const sid of sids) {
      await this.fetchUrl(page, `${base}/Administration/Games/LevelEditor.aspx?gid=${gameId}&level=${level}&swanswers=1&delsector=${sid}`)
      await timeout(1000)
    }

    // 4. Delete hints — penalty hints need `&penalty=1` in the delete URL,
    // otherwise the server treats the request as "add new" and silently ignores it
    const hints = await this.listHints(page, editorUrl)
    console.log(`cleanLevel: deleting ${hints.length} hints`)
    for (const h of hints) {
      const penaltyParam = h.penalty ? '&penalty=1' : ''
      await this.fetchUrl(page, `${base}/Administration/Games/PromptEdit.aspx?gid=${gameId}&level=${level}&prid=${h.prid}${penaltyParam}&action=PromptDelete`)
      await timeout(1000)
    }

    // 5. Reset autopass to 0:00:00 (effectively disables it)
    await this.setAutopass(page, 0)
  }

  private async fetchUrl(page: Page, url: string): Promise<void> {
    const MAX_RETRIES = 3

    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      const { finalUrl } = await page.evaluate(async (u: string) => {
        const r = await fetch(u, { credentials: 'include' })
        return { finalUrl: r.url }
      }, url)

      if (!finalUrl.includes('NotHumanRequest')) return

      console.warn(`\n⚠ бот-детект — логинимся и ждём 30 мин (попытка ${attempt}/${MAX_RETRIES})`)
      await this.auth()
      await timeout(30 * 60 * 1000)
    }

    console.warn(`\n✗ не удалось выполнить запрос после ${MAX_RETRIES} попыток: ${url}`)
  }

  // Submit a form by clicking the button via direct DOM .click() (not playwright's locator),
  // then waiting for the resulting navigation. Avoids playwright's auto-retry which can
  // double-submit asp.net image-button postback forms.
  private async submitForm(page: Page, buttonSelector: string): Promise<void> {
    const navPromise = page.waitForNavigation({ waitUntil: 'load' })
    await page.evaluate((sel: string) => {
      const btn = document.querySelector(sel) as HTMLElement | null
      if (btn) btn.click()
    }, buttonSelector)
    await navPromise
  }

  private async listIds(page: Page, url: string, hrefContains: string, paramName: string): Promise<string[]> {
    await this.gotoSafe(page, url)
    return page.evaluate(({ hrefContains, paramName }: { hrefContains: string; paramName: string }) => {
      const links = Array.from(document.querySelectorAll('a'))
      const ids = new Set<string>()
      for (const a of links) {
        const href = a.href
        if (!href.includes(hrefContains)) continue
        if (href.includes('action=add')) continue
        const re = new RegExp(`${paramName}=(\\d+)`)
        const m = re.exec(href)
        if (m) ids.add(m[1])
      }
      return [...ids]
    }, { hrefContains, paramName })
  }

  private async listHints(page: Page, editorUrl: string): Promise<Array<{ prid: string; penalty: boolean }>> {
    await this.gotoSafe(page, editorUrl)
    return page.evaluate(() => {
      const links = Array.from(document.querySelectorAll('a')) as HTMLAnchorElement[]
      const seen = new Map<string, boolean>()
      for (const a of links) {
        const href = a.href
        if (!href.includes('PromptEdit.aspx')) continue
        const m = /prid=(\d+)/.exec(href)
        if (!m) continue
        const prid = m[1]
        const isPenalty = href.includes('penalty=1')
        // First occurrence wins; if penalty=1 link found later, prefer it
        if (!seen.has(prid) || isPenalty) {
          seen.set(prid, isPenalty)
        }
      }
      return [...seen.entries()].map(([prid, penalty]) => ({ prid, penalty }))
    })
  }

  private async listSectorIds(page: Page): Promise<string[]> {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/LevelEditor.aspx?gid=${gameId}&level=${level}&swanswers=1`)
    return page.evaluate(() => {
      const inputs = Array.from(document.querySelectorAll('input[id^="hdnSectorNames_"]')) as HTMLInputElement[]
      const ids: string[] = []
      for (const i of inputs) {
        const m = /^(\d+)\s*:/.exec(i.value)
        if (m) ids.push(m[1])
      }
      return ids
    })
  }

  async setTaskBody(page: Page, body: string) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/TaskEdit.aspx?gid=${gameId}&level=${level}`)
    await page.waitForSelector('textarea[name="inputTask"]')

    // Set body and uncheck "перенос строк" (chkReplaceNlToBr) in one evaluate —
    // otherwise en.cx replaces \n with <br> which breaks HTML/scripts inside the body
    await page.evaluate((b: string) => {
      const ta = document.querySelector('textarea[name="inputTask"]') as HTMLTextAreaElement | null
      if (ta) {
        ta.value = b
        ta.dispatchEvent(new Event('input', { bubbles: true }))
        ta.dispatchEvent(new Event('change', { bubbles: true }))
      }

      const nl = document.querySelector('input[name="chkReplaceNlToBr"]') as HTMLInputElement | null
      if (nl && nl.checked) {
        nl.checked = false
        nl.dispatchEvent(new Event('click', { bubbles: true }))
        nl.dispatchEvent(new Event('change', { bubbles: true }))
      }
    }, body)

    await this.submitForm(page, 'input[name="btnAdd"]')
  }

  async setAutopass(page: Page, seconds: number, penaltySeconds?: number) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/LevelEditor.aspx?gid=${gameId}&level=${level}&swautopass=1`)
    await page.waitForSelector('input[name="txtApHours"]')

    const { h, m, s } = secondsToHms(seconds)
    const pen = penaltySeconds !== undefined ? secondsToHms(penaltySeconds) : undefined

    // Set values directly in DOM, force-enable disabled fields, dispatch events to trigger any client-side JS
    await page.evaluate((data: { h: number; m: number; s: number; ph?: number; pm?: number; ps?: number }) => {
      const set = (name: string, val: string): void => {
        const el = document.querySelector(`input[name="${name}"]`) as HTMLInputElement | null
        if (!el) return
        el.removeAttribute('disabled')
        el.value = val
        el.dispatchEvent(new Event('input', { bubbles: true }))
        el.dispatchEvent(new Event('change', { bubbles: true }))
      }

      set('txtApHours', String(data.h))
      set('txtApMinutes', String(data.m))
      set('txtApSeconds', String(data.s))

      // Penalty must be enabled via the chkTimeoutPenalty checkbox — otherwise the
      // server ignores the penalty values even when posted with values
      const cb = document.querySelector('input[name="chkTimeoutPenalty"]') as HTMLInputElement | null
      if (data.ph !== undefined && data.pm !== undefined && data.ps !== undefined) {
        if (cb) {
          cb.checked = true
          cb.dispatchEvent(new Event('click', { bubbles: true }))
          cb.dispatchEvent(new Event('change', { bubbles: true }))
        }

        set('txtApPenaltyHours', String(data.ph))
        set('txtApPenaltyMinutes', String(data.pm))
        set('txtApPenaltySeconds', String(data.ps))
      } else if (cb) {
        cb.checked = false
        cb.dispatchEvent(new Event('click', { bubbles: true }))
        cb.dispatchEvent(new Event('change', { bubbles: true }))
      }
    }, { h, m, s, ph: pen?.h, pm: pen?.m, ps: pen?.s })

    // Save button is the image button inside the form containing txtApHours
    await this.submitForm(page, 'form:has(input[name="txtApHours"]) input[type="image"][alt="Сохранить"]')
  }

  async addHint(page: Page, seconds: number, text: string) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/PromptEdit.aspx?gid=${gameId}&level=${level}`)
    await page.waitForSelector('textarea[name="NewPrompt"]')

    const { h, m, s } = secondsToHms(seconds)

    await this.setFieldsViaDom(page, {
      NewPromptTimeoutDays: '0',
      NewPromptTimeoutHours: String(h),
      NewPromptTimeoutMinutes: String(m),
      NewPromptTimeoutSeconds: String(s),
      NewPrompt: text,
    })

    await this.submitForm(page, 'input[name="btnAdd"]')
  }

  async addPenaltyHint(page: Page, seconds: number, text: string, penaltySeconds?: number, comment?: string) {
    const { gameId, level } = this._config
    await this.gotoSafe(page, `https://${this._config.domain}/Administration/Games/PromptEdit.aspx?gid=${gameId}&level=${level}&penalty=1`)
    await page.waitForSelector('textarea[name="NewPrompt"]')

    const { h, m, s } = secondsToHms(seconds)

    const fields: Record<string, string> = {
      NewPromptTimeoutDays: '0',
      NewPromptTimeoutHours: String(h),
      NewPromptTimeoutMinutes: String(m),
      NewPromptTimeoutSeconds: String(s),
      NewPrompt: text,
    }

    if (comment !== undefined) {
      fields.txtPenaltyComment = comment
    }

    if (penaltySeconds !== undefined) {
      const pen = secondsToHms(penaltySeconds)
      fields.PenaltyPromptHours = String(pen.h)
      fields.PenaltyPromptMinutes = String(pen.m)
      fields.PenaltyPromptSeconds = String(pen.s)
    }

    await this.setFieldsViaDom(page, fields)
    await this.submitForm(page, 'input[name="btnAdd"]')
  }

  // Set form field values directly in the DOM in one round-trip — bypasses any
  // per-field asp.net AutoPostBack triggers (which would otherwise submit a partial form
  // for each playwright .fill() call and create duplicate records).
  private async setFieldsViaDom(page: Page, values: Record<string, string>): Promise<void> {
    await page.evaluate((data: Record<string, string>) => {
      for (const [name, val] of Object.entries(data)) {
        const el = document.querySelector(`[name="${name}"]`) as (HTMLInputElement | HTMLTextAreaElement | null)
        if (!el) continue
        el.value = val
      }
    }, values)
  }

  async addAnswersToSector(page: Page, sector: string, answers: string[]) {
    // Optional "показать" link — silently skip if absent
    try {
      await page.getByRole('link', { name: 'показать' }).click({ timeout: 1 })
    } catch {
      // expected when link isn't present on the page
    }

    await timeout(1000)

    const chunks = chunkArray(answers, 10)
    const first = chunks.slice(0, 1)[0]
    const others = chunks.slice(1)
    await this.addAnswersToNewSector(page, sector, first)
    await timeout(1000)
    for (const element of others) {
      await timeout(2000)
      await this.addAnswersToLastSector(page, element)
    }
  }

  private async addAnswersToNewSector(page: Page, sector: string, answers: string[]) {
    // Есть ограничение только 10 ответов
    await page.getByRole('link', { name: 'Добавить сектор' }).click()
    await page.locator('input[name="txtSectorName"]').first().click()

    await page.locator('input[name="txtSectorName"]').first().fill(sector)

    const loc = page.locator('#AnswersTable_ctl00_divSectorEdit')

    for (let i = 0; i < answers.length; i++) {
      await loc.locator(`input[name="txtAnswer_${i}"]`).fill(answers[i])
    }

    await page.getByRole('button', { name: 'Сохранить' }).click()
  }

  private async addAnswersToLastSector(page: Page, answers: string[]) {
    // Есть ограничение только 10 ответов
    const loc = page.locator('#AnswersTable_ctl00_divAnswersContainer > .noPadMarg')
    const text = await loc.last().innerHTML()

    const match = /id="divAnswersEdit_(\d+)"/.exec(text)
    if (match) {
      const id = match[1]
      await page.getByText('Добавить ответы').click()
      await page.locator('#ddlSector').selectOption(id.toString())
      const loc = page.locator('#AnswersTable_ctl00_NewAnswerEditor')
      for (let i = 0; i < answers.length; i++) {
        await loc.locator(`input[name="txtAnswer_${i}"]`).fill(answers[i])
      }

      await page.getByRole('button', { name: 'Сохранить' }).click()
    }
  }

  private async addBonus(
    options: {
      page: Page;
      sector: string;
      answers: string[];
      time: { hours: number; minutes: number; seconds: number };
      isNegative: boolean;
      level: number;
      helpBody?: string;
    },
  ) {
    const popupPromise = options.page.waitForEvent('popup')
    await options.page.getByRole('link', { name: 'Добавить' }).nth(2).click()
    const popup = await popupPromise
    const chunks = chunkArray(options.answers, 100)
    const first = chunks.slice(0, 1)[0]
    const others = chunks.slice(1)

    await this.addNewBonus({
      answers: first,
      isNegative: options.isNegative,
      level: options.level,
      popup,
      sector: options.sector,
      time: options.time,
      helpBody: options.helpBody,
    })
    await timeout(1000)
    for (const element of others) {
      await this.addAnswersToBonus(popup, element)
      await timeout(2000)
    }

    await popup.close()
  }

  private async addAnswersToBonus(popup: Page, answers: string[]) {
    await popup.getByRole('link', { name: 'Редактировать' }).last().click()
    await popup.getByRole('link', { name: '30' }).click()
    await popup.getByRole('link', { name: '30' }).click()
    await popup.getByRole('link', { name: '30' }).click()
    await popup.getByRole('link', { name: '30' }).click()
    for (let i = 0; i < answers.length; i++) {
      await popup.locator(`input[name="answer_-${i + 1}"]`).fill(answers[i])
    }

    await popup.getByRole('button', { name: 'Обновить' }).click()
  }

  private async addNewBonus(options: {
    popup: Page;
    sector: string;
    answers: string[];
    time: { hours: number; minutes: number; seconds: number };
    isNegative: boolean;
    level: number;
    helpBody?: string;
  }) {
    const { popup, answers, sector, level, time, isNegative, helpBody } = options

    // Date picker '30' clicks (UI hardcode for default 30 days/etc) — keep on locators
    await popup.getByRole('link', { name: '30' }).click()
    await popup.getByRole('link', { name: '30' }).click()
    await popup.getByRole('link', { name: '30' }).click()
    await timeout(500)

    // Build all text fields in one DOM-set to avoid AutoPostBack triggering between fills
    // (which caused locators to detach and "Target crashed" errors)
    const fields: Record<string, string> = {
      txtBonusName: sector,
      txtHelp: helpBody ?? answers[0],
      txtHours: time.hours.toString(),
      txtMinutes: time.minutes.toString(),
      txtSeconds: time.seconds.toString(),
    }
    for (let i = 0; i < answers.length; i++) {
      fields[`answer_-${i + 1}`] = answers[i]
    }

    await this.setFieldsViaDom(popup, fields)

    // Tick the level checkbox via DOM (also avoids AutoPostBack reload race)
    await popup.evaluate((lvl: number) => {
      const wrappers = document.querySelectorAll('#levels_container .levelWrapper')
      for (const w of Array.from(wrappers)) {
        const txt = (w.textContent ?? '').trim()
        // match exact level number as a token, not substring
        if (new RegExp(`(^|\\D)${lvl}(\\D|$)`).test(txt)) {
          const cb = w.querySelector('input[type="checkbox"]') as HTMLInputElement | null
          if (cb && !cb.checked) {
            cb.checked = true
            cb.dispatchEvent(new Event('click', { bubbles: true }))
            cb.dispatchEvent(new Event('change', { bubbles: true }))
          }
        }
      }
    }, level)

    if (isNegative) {
      await popup.evaluate(() => {
        const neg = document.querySelector('#negative') as HTMLInputElement | null
        if (neg) {
          if (neg.type === 'checkbox' || neg.type === 'radio') {
            neg.checked = true
            neg.dispatchEvent(new Event('click', { bubbles: true }))
            neg.dispatchEvent(new Event('change', { bubbles: true }))
          } else {
            (neg as HTMLElement).click()
          }
        }
      })
    }

    await timeout(500)
    await popup.getByRole('button', { name: 'Добавить' }).click()
  }
}
