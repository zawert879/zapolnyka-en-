# zapolnyaka-en — Заполняка en.cx

Утилита для автоматического заполнения уровня в движке Encounter (en.cx) через Playwright. Читает JSON-конфиг (игра + уровни + коды) и вбивает их в LevelEditor через headful браузер.

## Стек
- TypeScript + ts-node
- Playwright (chromium, headless: false — визуально открывается окно браузера)
- commander (CLI)
- zod (валидация всех конфигов и codes.json)
- cli-table + colors + progress (вывод в терминал)

## Запуск
```bash
yarn dev go data/game.json        # ts-node, без сборки
# или
yarn build && yarn start go data/game.json
```
CLI команды:
- `go <путь_к_game>` — заполняет все уровни перечисленные в `game.json`/`game.yml` последовательно, используя один браузер и одну авторизацию.
- `game <путь> [-d domain] [-g gameId] [-b]` — создаёт новый `game.yml`/`game.json` (формат по расширению). По умолчанию `domain=tech.en.cx`, `gameId=0`, `defaultFormat=yml`, `levels=[]`. `--bare` — без комментариев.
- `level <путь_к_game> <dir> [level] [-f json|yml] [-b/--bare]` — создаёт папку `<dir>/` с `conf.*`, `codes.*`, пустым `task.html` и добавляет запись в `levels`. `dir` — имя папки (любая строка без слэшей, напр. `11`, `встреча`, `ulitsa-lenina`). `level` — номер уровня в en.cx; **если `dir` числовой, он используется и как level, аргумент можно опустить**; иначе обязателен. Примеры: `yarn dev new data/game.yml 11`, `yarn dev new data/game.yml встреча 11`. Формат: `--format` > `game.defaultFormat` > `yml`. По умолчанию yml-шаблон содержит секционные комментарии и закомментированные опциональные поля (`autopassPenalty`, `sectorsToClose`, `hints`, `penaltyHints`); `--bare` оставляет только секционные заголовки (`# файлы`, `# настройки уровня`) и минимум полей.

## Форматы конфигов: JSON и YAML
Все конфиги (`game`, `conf`, `codes`) поддерживают оба формата — парсер выбирает по расширению файла: `.yml`/`.yaml` → YAML (через пакет `yaml`), иначе JSON. В `game.json`/`game.yml` пути в `levels` могут ссылаться на `1/conf.yml`, `1/conf.json` — как удобно. То же для `codes` в conf (`codes: "codes.yml"` или `codes.json`). Утилита — [`parseConfigFile()` в src/utils.ts](zapolnyaka-en/src/utils.ts).

## Структура data/
```
data/
├── game.json            # глобальный конфиг игры
├── 1/                   # папка уровня N (по номеру уровня в en.cx)
│   ├── conf.json        # конфиг уровня
│   ├── task.html        # тело задания
│   └── codes.json       # коды (секторы/бонусы/штрафы)
├── 2/
│   ├── conf.json
│   ├── task.html
│   └── codes.json
└── ...
```

## Конфиг game.json (глобальный)
```json
{
  "domain": "tech.en.cx",
  "gameId": 81980,
  "levels": [
    "1/conf.json",
    "2/conf.json"
  ]
}
```
Пути в `levels` — относительно директории самого `game.*`. Обязательны: `domain`, `gameId`, `levels`. Опционально: `defaultFormat` (`json` | `yml`), `assetsDir` (папка с ассетами для команды `assets`, по умолчанию `assets/`).

**Логин/пароль — только в `.env`** (в корне репозитория `zapolnyaka-en/`): `EN_LOGIN=...` и `EN_PASSWORD=...`. Файл уже в `.gitignore`, пример — в `.env.example`. Если переменные не заданы — `go` прерывается с ошибкой.

## Загрузка ассетов (команда `assets`, только Go-версия)

Команда `assets <game>` заливает файлы из папки `assetsDir` (по умолчанию `assets/` рядом с game-файлом) в раздел en.cx «Файлы для игры». Используется для общего дизайна игры (`design.css`, `fog.js`, картинки), который подключается в `task.html` разметкой (`@import` / `<script src>`).

**Эндпоинт (разведан на живой форме):** `POST https://{domain}/Administration/Games/FileUploader.aspx?gid={gameId}`, `enctype=multipart/form-data`. Форма простая, без `__VIEWSTATE`:
- `inputFile1` — поле файла (один файл за запрос);
- `ctl03` — image-сабмит, поэтому шлём координаты клика `ctl03.x=1` и `ctl03.y=1`;
- игра определяется **только** query-параметром `gid` (скрытых полей с id нет).

Лимит — **48 МБ** на файл. После загрузки файл раздаётся с `https://d1.endata.cx/data/games/{gameId}/{файл}` (мгновенно, без кэша — для разработки) и `https://cdn.endata.cx/data/games/{gameId}/{файл}` (кэш 24 ч — прод). Реализация: `encx.AdminUploadGameFile` (multipart), `zapolnyaka.UploadAssets` (цикл + переавторизация при ErrSessionExpired), команда `cmd.RunAssets`. Команда **не** трогает тела уровней — версию `?v=N` для сброса кэша поднимаешь вручную.

## Конфиг уровня (data/N/conf.json)
```json
{
  "level": 3,
  "codes": "codes.json",
  "body": "task.html",
  "clean": true,
  "autopass": 1800,
  "autopassPenalty": 300,
  "hints": [
    { "time": 600, "text": "Подсказка 1" },
    { "time": 1200, "text": "Подсказка 2" }
  ],
  "penaltyHints": [
    { "time": 0, "text": "Спойлер", "penalty": 300, "comment": "Откроется со штрафом 5 мин" }
  ],
  "sectorsToClose": 7
}
```
Схемы в [src/config.ts](zapolnyaka-en/src/config.ts) (`Config.Game`, `Config.Level`).

Поля уровня: обязательны `level`, `codes`. Опционально: `body`, `clean`, `autopass`, `autopassPenalty`, `hints`, `penaltyHints`, `sectorsToClose`, `name`, `comment`. `codes` — относительный путь к JSON-файлу с кодами (от директории conf.json уровня). `body` — относительный путь к HTML/текстовому файлу с телом задания. `clean: true` — перед заливкой удалит все существующие задания, бонусы, сектора, подсказки и сбросит автопереход. `sectorsToClose: N` — «Условие прохождения»: закрыть N секторов для завершения уровня (по умолчанию «все секторы»). Выставляется **после** создания секторов. `name` / `comment` — «Название уровня» и «Комментарий к уровню» в админке. Форма: `NameCommentEdit.aspx?gid={gid}&level={level}`, поля `txtLevelName` (input) и `txtLevelComment` (textarea), кнопка `input[name="btnUpdate"]`.

**Все временные поля — в секундах** (150 = 2м30с, 1800 = 30м). `autopass` — длительность автоперехода (сек). `autopassPenalty` — штраф при AP (сек). `hints` — массив обычных подсказок: `time` (секунды до показа), `text`. `penaltyHints` — массив штрафных подсказок: `time` (секунды до доступности), `text`, опционально `penalty` (штраф в секундах), `comment` (текст предупреждения).

## Формат codes.json
Массив объектов. Каждый объект — один сектор/бонус/штраф/комбинация. Порядок выполнения = порядок в массиве.

```json
[
  {
    "type": "секторбонус",
    "sectorName": "сектор 1",
    "bonusName": "ингредиент 1",
    "answers": ["хлеб"],
    "time": 0,
    "help": "<script>(window.__q=window.__q||[]).push('ХЛЕБ');if(window.__drain)window.__drain();</script>"
  },
  {
    "type": "секторбонус",
    "sectorName": "сектор 2",
    "bonusName": "ингредиент 2",
    "answers": ["собака", "собаке"],
    "time": 60
  },
  {
    "type": "штраф",
    "bonusName": "ловушка",
    "answers": ["кошка", "кот"],
    "time": 60
  },
  {
    "type": "сектор",
    "sectorName": "голосование",
    "answers": ["1","2","3","4","5","6","7","8","9","10"]
  }
]
```

### Поля
| поле | обязательно | для каких типов | смысл |
|---|---|---|---|
| `type` | да | все | `сектор` \| `бонус` \| `штраф` \| `секторбонус` \| `секторштраф` |
| `sectorName` | опционально | типы с сектором (`сектор`/`секторбонус`/`секторштраф`) | имя сектора в админке. **Видно игроку сразу** — в списке секторов на странице уровня (не дожидаясь ввода кода). Если не задан — en.cx сгенерирует дефолтное имя. |
| `bonusName` | опционально | бонусные типы (`бонус`/`штраф`/`секторбонус`/`секторштраф`) | имя бонуса в админке. **Видно игроку сразу** — в списке бонусов на странице уровня. Если не задан — en.cx сгенерирует дефолтное. |
| `answers` | да | все | массив кодов (минимум 1) |
| `time` | да для всего кроме `сектор` | бонусные типы | секунды (длительность бонуса/штрафа). У чистого `сектор` разрешено, но игнорируется. |
| `help` | опционально | бонусные типы | идёт в `txtHelp` en.cx — **единственное поле, которое показывается игроку только после ввода верного кода**. Может содержать текст, HTML, `<script>`, картинку. У чистого `сектор` разрешено, но игнорируется. |

**⚠ Что видит игрок ДО ввода кода:** `sectorName` (в списке секторов), `bonusName` (в списке бонусов), длительность/знак бонуса. **После ввода** — только `help`. Никогда не клади ответ или спойлер в `sectorName`/`bonusName`.

Схема в [src/Entities/CodesParser.ts](zapolnyaka-en/src/Entities/CodesParser.ts) (`CodeSchema`, `CodesSchema`). Валидация через zod, ошибки прерывают запуск.

### Типы — что делает каждый
- **`сектор`** — только сектор. Создаёт сектор с ответами, ничего больше.
- **`бонус`** — только бонус (-время). Создаёт бонус с указанными кодами и положительным временем.
- **`штраф`** — только штрафной бонус (+время). То же что `бонус`, но с флагом `negative`.
- **`секторбонус`** — комбо: создаётся И сектор И бонус с одним набором кодов (имена независимые — `sectorName`/`bonusName`). Вводя код, игрок одновременно закрывает сектор и срабатывает бонус (выполняется `help`).
- **`секторштраф`** — комбо: сектор + штраф. Закрытие сектора одновременно добавляет штрафное время. Полезно для троллинга — игрок нашёл правильный код, но это была ловушка.

### Лимиты en.cx
- 10 ответов на сектор за один запрос — заполняка добивает чанками.
- 100 ответов на бонус за один запрос — заполняка тоже добивает чанками через «Редактировать».

## Логика работы ([src/Entities/Zapolnyaka.ts](zapolnyaka-en/src/Entities/Zapolnyaka.ts))
1. Логин на `https://{domain}/Login.aspx`.
2. Переход на `Administration/Games/LevelEditor.aspx?gid={gameId}&level={level}&swanswers=1`.
3. **Если `clean: true`**: `cleanLevel(page)` — удаляет все задания, бонусы, сектора, подсказки и сбрасывает автопереход.
4. **Если задан `autopass`**: `setAutopass(page, autopass, autopassPenalty?)`.
5. **Если задан `body`**: `setTaskBody(page, body)` — заполняет `textarea[name="inputTask"]`, клик `input[name="btnAdd"]`.
6. **Если задан `hints`**: для каждой — `addHint(page, time, text)`.
7. **Если задан `penaltyHints`**: для каждой — `addPenaltyHint(page, time, text, penalty?, comment?)`.
8. Для каждой записи `codes.json`:
   - Если тип содержит «сектор» (`сектор`/`секторбонус`/`секторштраф`) → `addAnswersToSector(name, answers)` — нажимает «Добавить сектор», вбивает имя и до 10 ответов; добивает остатки чанками по 10.
   - Если тип содержит «бонус»/«штраф» (всё кроме `сектор`) → `addBonus(...)` — открывает попап «Добавить», вбивает имя, время (часы/минуты/секунды), отмечает чекбокс уровня, ставит `#negative` если `штраф`/`секторштраф`, в `txtHelp` пишет `help` (или первый ответ как fallback). Лимит 100 ответов на бонус — добивает через «Редактировать».
9. Между действиями `timeout(2000)` мс.
10. Прогресс в терминале (`progress` bar) + цветная таблица всех кодов перед началом.

## Важные нюансы / подводные камни
- **Браузер не headless** — окно открывается, можно наблюдать/вмешаться.
- **Лимиты en.cx**: 10 ответов на сектор, 100 ответов на бонус — обходится чанками.
- **Парсер codes.json — zod-валидация**, ошибки прерывают запуск с понятным сообщением. Запрещённые комбинации (`time` у чистого сектора, `help` у чистого сектора) ловятся.
- **Лог-таблица** в терминале показывает все распарсенные коды ДО начала работы с браузером.
- **Селекторы привязаны к русской локали** en.cx («Добавить сектор», «Сохранить», «Вход», «показать», «Добавить ответы», «Редактировать», «Обновить»).
- **`addNewBonus` четыре раза кликает на `'30'`** — это дни/часы какого-то date picker'а, хардкод под текущий UI движка.
- **`chunkArray` и `timeout`** — в [src/utils.ts](zapolnyaka-en/src/utils.ts).
- **Креды в `.env`**: `EN_LOGIN` / `EN_PASSWORD`, грузятся через `dotenv.config()` в `src/index.ts`. В `game.*` их класть не надо.
- **Порог сектора** («Условие прохождения», например 7/8) — поле `sectorsToClose` в conf.json. Форма на `LevelEditor.aspx?gid={gid}&level={level}&swanswers=1`: radio `rbSectorCompleteType` (`1`=все, `2`=N), input `txtRequiredSectorsCount`, кнопка `input[name="pnlSettings_SectorsCompletionSettings_ctl00_btnSave"]`, скрытое поле `action=upsecsett`. Ставится **после** создания секторов, иначе нечего закрывать.

## Заливка тела задания (body)
Форма `TaskEdit.aspx`:
- `textarea[name="inputTask"]` — тело (HTML/BB-теги en.cx)
- `select[name="forMemberID"]` — кому адресовано (`всех` по умолчанию)
- `input[name="chkReplaceNlToBr"]` — автоперенос строк (`\n` → `<br>`), по умолчанию включён
- `input[name="btnAdd"]` — image-кнопка «Добавить»

После успешного добавления URL меняется на `?action=OpenerReload&tid={newTaskId}`. Скрипт добавляет **новое** задание; для замены используй `clean: true`.

## Автопереход (autopass)
Форма на `LevelEditor.aspx?gid={gid}&level={level}&swautopass=1`:
- `txtApHours`, `txtApMinutes`, `txtApSeconds` — длительность AP
- `txtApPenaltyHours`, `txtApPenaltyMinutes`, `txtApPenaltySeconds` — штрафное время
- `chkTimeoutPenalty` — чекбокс, **обязательно отметить** перед заполнением штрафных полей, иначе сервер игнорирует значения
- Save: `input[type="image"][alt="Сохранить"]` внутри формы с `txtApHours`

## Подсказки (hints / penaltyHints)
Формы:
- Обычная: `PromptEdit.aspx?gid={gid}&level={level}` — `NewPromptTimeout{Days,Hours,Minutes,Seconds}`, `NewPrompt` (textarea), `btnAdd`.
- Штрафная: `PromptEdit.aspx?gid={gid}&level={level}&penalty=1` — те же поля плюс `txtPenaltyComment`, `chkRequestPenaltyConfirm`, `PenaltyPrompt{Hours,Minutes,Seconds}`.
- Поле `ForMemberID` остаётся `0` (для всех).
- После создания редирект на `?prid={newId}&action=OpenerReload`.
- **`setFieldsViaDom`** заполняет все поля одним `page.evaluate` чтобы не триггерить asp.net AutoPostBack (иначе создаются дубли).
- **`submitForm`** делает DOM `.click()` через evaluate + `waitForNavigation` отдельно, чтобы избежать playwright auto-retry двойных submit'ов.

## Очистка уровня (clean режим)
При `clean: true` метод `cleanLevel(page)` перед заливкой удаляет всё содержимое уровня. URL-эндпоинты (работают по прямой навигации, без UI confirm-диалогов):

| Сущность | Перечисление на LevelEditor | URL удаления |
|---|---|---|
| Задание | `a[href*="TaskEdit.aspx"]`, извлечь `tid=\d+` | `TaskEdit.aspx?gid={gid}&level={level}&action=TaskDelete&tid={tid}` |
| Бонус | `a[href*="BonusEdit.aspx"]`, извлечь `bonus=\d+` (исключая `action=add`) | `BonusEdit.aspx?gid={gid}&level={level}&bonus={bid}&action=delete` |
| Сектор | `input[id^="hdnSectorNames_"]`, value формата `{id} : '{name}'` (нужен `swanswers=1` в URL LevelEditor) | `LevelEditor.aspx?gid={gid}&level={level}&swanswers=1&delsector={sid}` |
| Подсказка (обычная и штрафная) | `a[href*="PromptEdit.aspx"]`, извлечь `prid=\d+` + флаг `penalty=1` | `PromptEdit.aspx?gid={gid}&level={level}&prid={prid}[&penalty=1]&action=PromptDelete` |
| Автопереход | — (не удаление, а сброс) | `setAutopass(page, 0)` — выставляет 0:00:00 |

Порядок: задания → бонусы → сектора → подсказки → сброс AP. Между удалениями `timeout(500)` мс. Удаление через `fetchUrl()` (page.evaluate + fetch credentials:include) — избегает `ERR_ABORTED` от слишком быстрых page.goto. Штрафные подсказки **обязательно** удаляются с `&penalty=1` в URL, иначе сервер возвращает «add» форму.

## Когда читать этот файл
Каждый раз, когда пользователь говорит про «заполняку», «zapolnyaka», просит сгенерировать `codes.json` для неё, залить коды в движок en.cx, изменить формат кодов, починить селекторы Playwright или поменять конфиг.
