# Примеры конфигураций

Два примера игр, каждая со своими уровнями. Демонстрируют все типы кодов, оба формата конфигов (JSON / YAML) и разные фичи.

## Игра 1 — `game.json` / `game.yml`

Домен `tech.en.cx`, 4 уровня с базовыми типами кодов.

| Папка | Что демонстрирует |
|---|---|
| `1-sectors/` | Чистые секторы — простейший уровень с несколькими кодами |
| `2-bonuses/` | Бонусы и штрафы — коды, дающие или отнимающие время |
| `3-combo/` | Комбо-типы `секторбонус` и `секторштраф` с `sectorName` |
| `4-full-level/` | Всё вместе — подсказки, автопереход, штрафные подсказки |

```bash
yarn dev go examples/game.json    # JSON
yarn dev go examples/game.yml     # YAML
```

## Игра 2 — `game2.json` / `game2.yml`

Домен `quest.en.cx`, 2 уровня с продвинутыми фичами.

| Папка | Что демонстрирует |
|---|---|
| `5-script-help/` | `<script>` в поле `help` — интерактивное задание, код меняет страницу |
| `6-many-sectors/` | Много секторов + порог `sectorsToClose` (3 из 5) |

```bash
yarn dev go examples/game2.json   # JSON
yarn dev go examples/game2.yml    # YAML
```

## Форматы: JSON и YAML

Заполняка поддерживает оба формата. Выбирай что удобнее:

- **JSON** — стандарт, строгий синтаксис
- **YAML** — удобнее для уровней с подсказками, не нужны запятые и скобки

Формат определяется по расширению файла (`.json` / `.yml`).

## Глобальный конфиг игры

Логин/пароль берутся из `.env` (`EN_LOGIN` / `EN_PASSWORD`), а не из game-файла.

**game.json:**
```json
{
  "domain": "tech.en.cx",
  "gameId": 82000,
  "levels": [
    "1-sectors/conf.json",
    "2-bonuses/conf.json"
  ]
}
```

**game.yml:**
```yaml
domain: tech.en.cx
gameId: 82000
defaultFormat: yml

levels:
  - 1-sectors/conf.yml
  - 2-bonuses/conf.yml
```

Поле `defaultFormat` опционально — задаёт формат по умолчанию для команды `new`.

## Как использовать

1. Скопировать папку примера в `data/N/` (где N — номер уровня в en.cx)
2. Подключить `N/conf.json` (или `conf.yml`) в `game.json` / `game.yml` → массив `levels`
3. Создать `.env` с `EN_LOGIN` и `EN_PASSWORD`
4. Запустить `yarn dev go data/game.json` (или `game.yml`)

## Структура каждого примера

```
N/
├── conf.json    — конфиг уровня (JSON)
├── conf.yml     — конфиг уровня (YAML) — то же самое, другой формат
├── codes.json   — коды: секторы, бонусы, штрафы
├── task.html    — тело задания (опционально)
└── README.md    — описание примера
```
