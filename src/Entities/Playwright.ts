import { type Browser, chromium, type Page } from "@playwright/test"
import { timeout } from "../utils"

export class Playwright {
  private _browser: Browser | null = null
  private _mainPage: Page | null = null

  constructor() {
    void this.startBrowser()
  }

  async stop() {
    await this.closeBrowser()
  }

  async mainPage(): Promise<Page> {
    return new Promise(resolve => {
      (async () => {
        for (let i = 0; i < 500; i++) {
          if (this._mainPage) {
            resolve(this._mainPage)
            return
          }
          // eslint-disable-next-line no-await-in-loop
          await timeout(100)
        }
      })()
    })
  }

  private async startBrowser() {
    this._browser = await chromium.launch({ headless: false })
    this._mainPage = await this._browser.newPage()
    await this._mainPage.setViewportSize({ width: 1080, height: 1024 })
  }

  private async closeBrowser() {
    await this._browser?.close()
  }
}
