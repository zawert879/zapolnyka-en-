import { Command } from 'commander'
import dotenv from 'dotenv'
import fs from 'fs'
import path from 'path'
import YAML from 'yaml'

dotenv.config()
import { CodesParser, type Code } from './Entities/CodesParser'
import { Zapolnyaka } from './Entities/Zapolnyaka'
import { Playwright } from './Entities/Playwright'
import { Config } from './config'
import { type Data } from './data'
import { parseConfigFile } from './utils'

const program = new Command()

program
  .name('zapolnyaka')
  .version('0.0.1')

type PreparedLevel = {
  relPath: string;
  config: Data.Config.Self;
  codes: Code[];
  body?: string;
}

program.command('go')
  .argument('<game>', 'path to game.json')
  .action(async (gamePath: string) => {
    // === PHASE 1: validate everything upfront ===
    const gameAbsPath = path.resolve(process.cwd(), gamePath)
    if (!fs.existsSync(gameAbsPath)) {
      console.error(`конфиг игры не найден: ${gameAbsPath}`)
      process.exit(1)
    }

    const gameParse = await Config.Game.safeParseAsync(parseConfigFile(gameAbsPath))
    if (!gameParse.success) {
      console.error(`невалидный конфиг игры (${gamePath}):`)
      console.error(gameParse.error.format())
      process.exit(1)
    }

    const game = gameParse.data
    const gameDir = path.dirname(gameAbsPath)

    const login = process.env.EN_LOGIN
    const password = process.env.EN_PASSWORD
    if (!login || !password) {
      console.error('в .env не заданы EN_LOGIN / EN_PASSWORD')
      process.exit(1)
    }

    console.log(`игра: ${game.domain} gameId=${game.gameId}, уровней: ${game.levels.length}`)
    console.log('=== проверка всех уровней ===')

    const prepared: PreparedLevel[] = []
    const errors: string[] = []
    const codesParser = new CodesParser()

    for (const levelRelPath of game.levels) {
      const levelAbsPath = path.resolve(gameDir, levelRelPath)
      const levelDir = path.dirname(levelAbsPath)

      if (!fs.existsSync(levelAbsPath)) {
        errors.push(`[${levelRelPath}] файл не найден`)
        continue
      }

      let levelConf
      try {
        const levelParse = await Config.Level.safeParseAsync(parseConfigFile(levelAbsPath))
        if (!levelParse.success) {
          errors.push(`[${levelRelPath}] невалидный conf:\n${JSON.stringify(levelParse.error.format(), null, 2)}`)
          continue
        }

        levelConf = levelParse.data
      } catch (e) {
        errors.push(`[${levelRelPath}] ошибка парсинга conf: ${(e as Error).message}`)
        continue
      }

      // codes
      const codesPath = path.resolve(levelDir, levelConf.codes)
      if (!fs.existsSync(codesPath)) {
        errors.push(`[${levelRelPath}] файл с кодами не найден: ${levelConf.codes}`)
        continue
      }

      let codes: Code[]
      try {
        codes = codesParser.go(codesPath)
      } catch (e) {
        errors.push(`[${levelRelPath}] невалидный codes:\n${(e as Error).message}`)
        continue
      }

      // body (optional)
      let body: string | undefined
      if (levelConf.body) {
        const bodyPath = path.resolve(levelDir, levelConf.body)
        if (!fs.existsSync(bodyPath)) {
          errors.push(`[${levelRelPath}] файл тела не найден: ${levelConf.body}`)
          continue
        }

        body = fs.readFileSync(bodyPath, { encoding: 'utf-8' })
      }

      const fullConfig = {
        login,
        password,
        domain: game.domain,
        gameId: game.gameId,
        ...levelConf,
      }

      prepared.push({ relPath: levelRelPath, config: fullConfig, codes, body })
      console.log(`  [${levelRelPath}] OK — уровень ${levelConf.level}, кодов ${codes.length}${body !== undefined ? `, тело ${body.length} симв.` : ''}`)
    }

    if (errors.length > 0) {
      console.error(`\n=== ошибок проверки: ${errors.length} — прерывание ===`)
      for (const e of errors) console.error(e)
      process.exit(1)
    }

    console.log(`\n=== все ${prepared.length} уровней валидны — запуск браузера ===`)

    // === PHASE 2: browser work ===
    const playwright = new Playwright()
    let loggedIn = false

    for (const p of prepared) {
      console.log(`\n=== уровень ${p.config.level} (${p.relPath}) ===`)
      const zapolnyaka = new Zapolnyaka(playwright, p.config)
      if (!loggedIn) {
        await zapolnyaka.auth()
        loggedIn = true
      }

      await zapolnyaka.processLevel(p.codes, p.body)
    }

    await playwright.stop()
  })

program.command('level')
  .argument('<game>', 'path to game.json/game.yml')
  .argument('<dir>', 'folder name inside data/ (numeric → also used as en.cx level number)')
  .argument('[level]', 'en.cx level number (required if dir is non-numeric)')
  .option('-f, --format <fmt>', 'config format: json | yml (overrides game.defaultFormat, default yml)')
  .option('-b, --bare', 'minimal template without comments and placeholder optional fields')
  .action(async (gamePath: string, dirArg: string, levelArg: string | undefined, opts: { format?: string; bare?: boolean }) => {
    const gameAbsPath = path.resolve(process.cwd(), gamePath)
    if (!fs.existsSync(gameAbsPath)) {
      console.error(`конфиг игры не найден: ${gameAbsPath}`)
      process.exit(1)
    }

    const gameParse = await Config.Game.safeParseAsync(parseConfigFile(gameAbsPath))
    if (!gameParse.success) {
      console.error(`невалидный конфиг игры (${gamePath}):`)
      console.error(gameParse.error.format())
      process.exit(1)
    }

    const game = gameParse.data

    // level: explicit levelArg wins; else if dir is digits-only — use it; else error
    const levelSource = levelArg ?? (/^\d+$/.test(dirArg) ? dirArg : undefined)
    if (levelSource === undefined) {
      console.error(`укажи номер уровня 3-м аргументом (папка "${dirArg}" не число)`)
      process.exit(1)
    }

    const level = Number(levelSource)
    if (!Number.isInteger(level) || level <= 0) {
      console.error(`некорректный номер уровня: ${levelSource}`)
      process.exit(1)
    }

    const fmt = (opts.format ?? game.defaultFormat ?? 'yml').toLowerCase()
    if (fmt !== 'json' && fmt !== 'yml') {
      console.error(`некорректный формат --format: ${fmt} (ожидается json или yml)`)
      process.exit(1)
    }

    const gameDir = path.dirname(gameAbsPath)
    const dirName = dirArg
    if (/[\\/]/.test(dirName)) {
      console.error(`имя папки не должно содержать слэшей: ${dirName}`)
      process.exit(1)
    }

    const levelDir = path.resolve(gameDir, dirName)
    if (fs.existsSync(levelDir)) {
      console.error(`папка уровня уже существует: ${levelDir}`)
      process.exit(1)
    }

    const confName = `conf.${fmt}`
    const codesName = `codes.${fmt}`

    const bare = Boolean(opts.bare)

    // conf: for yml — hand-built with section comments (files / level settings);
    // for json — plain JSON (no comments supported)
    let confOut: string
    if (fmt === 'yml') {
      if (bare) {
        confOut = [
          '# файлы',
          `codes: ${codesName}`,
          'body: task.html',
          'clean: true',
          '',
          '# настройки уровня',
          `level: ${level}`,
          'autopass: 0',
          '',
        ].join('\n')
      } else {
        confOut = [
          '# файлы',
          `codes: ${codesName}`,
          'body: task.html',
          'clean: true                  # очистить уровень перед заливкой',
          '',
          '# настройки уровня',
          `level: ${level}`,
          'autopass: 0                  # автопереход через N секунд (0 — выкл)',
          '#autopassPenalty: 0          # штраф при автопереходе, сек',
          '#name: "Название уровня"     # «Название уровня» в админке',
          '#comment: "комментарий"      # «Комментарий к уровню» в админке',
          '#sectorsToClose: 1           # «Условие прохождения»: сколько секторов закрыть (по умолчанию — все)',
          '#hints:                      # обычные подсказки',
          '#  - time: 600',
          '#    text: "Подсказка 1"',
          '#penaltyHints:               # штрафные подсказки',
          '#  - time: 0',
          '#    text: "Спойлер"',
          '#    penalty: 300',
          '#    comment: "Откроется со штрафом 5 мин"',
          '',
        ].join('\n')
      }
    } else {
      confOut = `${JSON.stringify({
        codes: codesName,
        body: 'task.html',
        clean: true,
        level,
        autopass: 0,
      }, null, 2)}\n`
    }

    let codesOut: string
    if (fmt === 'yml') {
      codesOut = bare ? '' : [
        '# записи кодов — порядок в массиве = порядок создания в админке',
        '# типы: сектор | бонус | штраф | секторбонус | секторштраф',
        '# поля:',
        '#   type        — см. выше',
        '#   sectorName  — имя сектора в админке (обязательно для типов с сектором)',
        '#   bonusName   — имя бонуса в админке (обязательно для типов с бонусом/штрафом)',
        '#   answers     — массив кодов (минимум 1)',
        '#   time        — секунды, обязательно для бонусных типов',
        '#   help        — подсказка/HTML/скрипт в txtHelp (для бонусных типов)',
        '# sectorName и bonusName видны игроку сразу (в списках секторов/бонусов уровня).',
        '# Не клади туда ответ. Только help показывается после ввода верного кода.',
        '',
        '# --- примеры ---',
        '',
        '# чистый сектор',
        '# - type: сектор',
        '#   sectorName: "сектор 1"',
        '#   answers:',
        '#     - ответ1',
        '#     - ответ2',
        '',
        '# бонус (минус время)',
        '# - type: бонус',
        '#   bonusName: "бонус хлеб"',
        '#   answers:',
        '#     - хлеб',
        '#   time: 300              # -5 минут',
        '#   help: "молодец!"       # показать игроку после ввода (HTML/<script> допустимы)',
        '',
        '# штраф (плюс время)',
        '# - type: штраф',
        '#   bonusName: "ловушка кот"',
        '#   answers:',
        '#     - кот',
        '#     - кошка',
        '#   time: 600              # +10 минут',
        '#   help: "ловушка!"',
        '',
        '# сектор+бонус: закрывает сектор и даёт бонус одним кодом',
        '# - type: секторбонус',
        '#   sectorName: "сектор 3"   # видно сразу — нейтральное имя',
        '#   bonusName: "ингредиент"  # видно сразу — не клади ответ',
        '#   answers:',
        '#     - хлеб',
        '#   time: 0',
        '#   help: "молодец!"         # показывается только после ввода верного кода',
        '',
        '# сектор+штраф: закрытие сектора штрафит по времени',
        '# - type: секторштраф',
        '#   sectorName: "обманка"',
        '#   bonusName: "ловушка"',
        '#   answers:',
        '#     - обманка',
        '#   time: 300',
        '#   help: "это была ловушка"',
        '',
      ].join('\n')
    } else {
      codesOut = bare ? '[]\n' : `${JSON.stringify([{ type: 'сектор', sectorName: 'сектор 1', answers: ['ответ'] }], null, 2)}\n`
    }

    fs.mkdirSync(levelDir)
    fs.writeFileSync(path.join(levelDir, confName), confOut, 'utf-8')
    fs.writeFileSync(path.join(levelDir, codesName), codesOut, 'utf-8')
    fs.writeFileSync(path.join(levelDir, 'task.html'), '', 'utf-8')

    const relConf = `${dirName}/${confName}`
    if (!game.levels.includes(relConf)) {
      game.levels.push(relConf)
    }

    const gameExt = path.extname(gameAbsPath).toLowerCase()
    const gameOut = (gameExt === '.yml' || gameExt === '.yaml')
      ? YAML.stringify(game)
      : `${JSON.stringify(game, null, 2)}\n`
    fs.writeFileSync(gameAbsPath, gameOut, 'utf-8')

    console.log(`создан уровень ${level} (${fmt}):`)
    console.log(`  ${path.relative(process.cwd(), path.join(levelDir, confName))}`)
    console.log(`  ${path.relative(process.cwd(), path.join(levelDir, codesName))}`)
    console.log(`  ${path.relative(process.cwd(), path.join(levelDir, 'task.html'))}`)
    console.log(`добавлено в ${gamePath}: ${relConf}`)
  })

program.command('game')
  .argument('<path>', 'путь к создаваемому game.yml / game.json (формат — по расширению)')
  .option('-d, --domain <domain>', 'домен en.cx (tech.en.cx / rnd.en.cx / ...)', 'tech.en.cx')
  .option('-g, --gameId <id>', 'id игры в en.cx')
  .option('-b, --bare', 'минимальный шаблон без комментариев')
  .action((outPath: string, opts: { domain: string; gameId?: string; bare?: boolean }) => {
    const abs = path.resolve(process.cwd(), outPath)
    if (fs.existsSync(abs)) {
      console.error(`файл уже существует: ${abs}`)
      process.exit(1)
    }

    const ext = path.extname(abs).toLowerCase()
    let fmt: 'yml' | 'json'
    if (ext === '.yml' || ext === '.yaml') fmt = 'yml'
    else if (ext === '.json') fmt = 'json'
    else {
      console.error(`неизвестное расширение ${ext} — используй .yml или .json`)
      process.exit(1)
      return
    }

    const gameId = opts.gameId !== undefined ? Number(opts.gameId) : 0
    if (opts.gameId !== undefined && (!Number.isInteger(gameId) || gameId <= 0)) {
      console.error(`некорректный --gameId: ${opts.gameId}`)
      process.exit(1)
    }

    const bare = Boolean(opts.bare)

    let out: string
    if (fmt === 'yml') {
      out = bare
        ? [
          `domain: ${opts.domain}`,
          `gameId: ${gameId}`,
          'defaultFormat: yml',
          'levels: []',
          '',
        ].join('\n')
        : [
          '# подключение (логин/пароль — в .env: EN_LOGIN / EN_PASSWORD)',
          `domain: ${opts.domain}`,
          `gameId: ${gameId}                # id игры в en.cx`,
          '',
          '# формат по умолчанию для команды `level` (json | yml)',
          'defaultFormat: yml',
          '',
          '# уровни — относительные пути к conf.*, порядок = порядок заливки',
          'levels: []',
          '',
        ].join('\n')
    } else {
      out = `${JSON.stringify({
        domain: opts.domain,
        gameId,
        defaultFormat: 'yml',
        levels: [],
      }, null, 2)}\n`
    }

    const dir = path.dirname(abs)
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(abs, out, 'utf-8')

    console.log(`создан ${path.relative(process.cwd(), abs)}`)
    if (gameId === 0) console.log('  ⚠ gameId не задан — укажи флагом -g или правкой файла')
    if (!process.env.EN_LOGIN || !process.env.EN_PASSWORD) {
      console.log('  ⚠ не забудь создать .env с EN_LOGIN / EN_PASSWORD')
    }
  })

program.parse()
