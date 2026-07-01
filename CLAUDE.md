# CLAUDE.md

## Что это

**zapolnyaka-en** — утилита для автозаполнения уровней Encounter (en.cx) через Playwright. Читает конфиги (game + level + codes) и вбивает всё в LevelEditor через headful браузер: тело задания, коды, бонусы, сектора, подсказки, автопереход, условие прохождения.

## Стек

- TypeScript + ts-node
- Playwright (chromium, headless: false)
- commander (CLI), zod (валидация), yaml (YAML-конфиги)
- cli-table + colors + progress (вывод в терминал)

## Запуск

```bash
yarn install
yarn dev go data/game.yml     # dev-режим через ts-node
yarn build && yarn start go data/game.yml  # production
```

## CLI-команды

- `go <game>` — залить все уровни из game-конфига
- `assets <game>` — залить ассеты (css/js/картинки) из папки `assetsDir` (по умолчанию `assets/`) в «Файлы для игры» en.cx под UUID-именами (маппинг в `assets/.manifest.json`). В контенте ссылаешься плейсхолдером `{{имя}}`, `go` подставляет URL; префикс `~` у файла = оставить читаемое имя без uuid. Только Go-версия (`go_app/`)
- `game <path> [-d domain] [-g gameId] [-b]` — создать новый game-конфиг
- `level <game> <dir> [level] [-f json|yml] [-b]` — создать шаблон уровня и добавить в game

## Структура

```
zapolnyaka-en/
├── src/
│   ├── index.ts              # точка входа, CLI (commander)
│   ├── config.ts             # Zod-схемы: Game, Level, Hint, PenaltyHint
│   ├── data.ts               # загрузка данных
│   ├── utils.ts              # parseConfigFile(), chunkArray(), timeout()
│   └── Entities/
│       ├── Zapolnyaka.ts     # основная логика заполнения через Playwright
│       └── CodesParser.ts    # CodeSchema, CodesSchema (zod-валидация codes)
├── data/                     # рабочие конфиги игр (в .gitignore)
├── examples/                 # примеры конфигов (2 игры, 6 уровней)
├── forAI.md                  # подробная техдокументация
└── README.md                 # пользовательская документация
```

## Конфиги: JSON и YAML

Оба формата поддерживаются для всех конфигов (game, conf, codes). Парсер выбирает по расширению файла. Можно мешать — в game.yml ссылаться на conf.json и наоборот.

## Ключевые файлы для понимания кода

- **`forAI.md`** — полная техсправка: все поля конфигов, типы кодов, селекторы Playwright, URL-эндпоинты en.cx, логика очистки, лимиты, подводные камни. **Читай первым при любой работе с кодом.**
- **`src/config.ts`** — Zod-схемы всех конфигов
- **`src/Entities/CodesParser.ts`** — схема и валидация codes.json/codes.yml
- **`src/Entities/Zapolnyaka.ts`** — основной класс, вся Playwright-логика

## Типы кодов en.cx

| Тип | Что создаёт |
|---|---|
| `сектор` | только сектор |
| `бонус` | бонус (−время) |
| `штраф` | штрафной бонус (+время) |
| `секторбонус` | сектор + бонус (один набор кодов) |
| `секторштраф` | сектор + штраф (один набор кодов) |

## Важные правила

- **Креды только в `.env`** (`EN_LOGIN` / `EN_PASSWORD`), никогда в конфигах
- **Все временные поля — в секундах** (300 = 5 мин, 1800 = 30 мин)
- **`sectorName` и `bonusName` видны игроку сразу** — не клади туда спойлеры
- **`help` — единственное поле, видимое только после ввода кода** — для текста, HTML, `<script>`
- **Селекторы привязаны к русской локали** en.cx
- **`sectorsToClose`** ставится после создания секторов
- **`data/` в .gitignore** — содержит рабочие конфиги конкретных игр

## Язык

Весь контент, документация и общение — на русском.
