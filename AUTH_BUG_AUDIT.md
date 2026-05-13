# Аудит: причина потери авторизации

## Корневая причина

### Проблема 1: `omitempty` на Login/Password (ИСПРАВЛЕНО)

```go
// БЫЛО (баг):
Login    string `json:"login,omitempty"`
Password string `json:"password,omitempty"`

// СТАЛО (fix):
Login    string `json:"login"`
Password string `json:"password"`
```

С `omitempty` пустая строка `""` **не записывается** в JSON.
При чтении файла без поля `login` → `Login = ""`.
При мёрже в SaveHistory: `existing.Login = ""` → мёрж не помогает → цикл.

### Проблема 2: SaveHistory без мёрджа (ИСПРАВЛЕНО ранее)

Старый код просто перезаписывал файл целиком:
```go
// БЫЛО:
data, _ := json.MarshalIndent(h, "", "  ")
os.WriteFile(historyFile, data, 0o644)
```

Любой вызов `SaveHistory(History{LastGame: "..."})` с пустым Login стирал credentials.

Сейчас SaveHistory читает существующий файл и мёрджит:
```go
// ЕСТЬ:
existing := LoadHistory()
if h.Login == ""    { h.Login = existing.Login }
if h.Password == "" { h.Password = existing.Password }
```

## Все вызовы SaveHistory

| Место | Что сохраняет | Риск потери Login/Password |
|-------|--------------|---------------------------|
| `main.go promptAndRunGo()` | hist.LastGame | НЕТ — hist загружен через LoadHistory() |
| `main.go promptAndRunCode()` | hist.LastGame, hist.LastLevel | НЕТ |
| `main.go promptAndRunLevel()` | hist.LastGame | НЕТ |
| `main.go promptAndRunAuth()` | hist.Login, hist.Password | НЕТ — явно записывает credentials |
| `main.go promptAndRunSelfTest()` | hist.SelfTestDevLevel | НЕТ — hist загружен через LoadHistory() |
| `main.go runCLI() / selftest` | hist.SelfTestDevLevel | НЕТ |
| `main.go runCLI() / go` | hist.LastGame | НЕТ |
| `main.go runCLI() / auth` | hist.Login, hist.Password | НЕТ — явно записывает |

## Все файлы, имеющие отношение к auth

| Файл | Роль |
|------|------|
| `go_app/cmd/history.go` | History struct, LoadHistory, SaveHistory |
| `go_app/cmd/go_cmd.go` | читает Login/Password, НЕ сохраняет |
| `go_app/cmd/check_cmd.go` | читает Login/Password, НЕ сохраняет |
| `go_app/cmd/selftest_cmd.go` | читает Login/Password, НЕ сохраняет |
| `go_app/main.go` | всe вызовы SaveHistory |
| `go_app/internal/zapolnyaka/auth.go` | Auth(), gotoSafe() — работа с браузером |
| `dist/.zapolnyaka.json` | хранилище (CWD-относительный путь!) |

## Важно: путь к файлу зависит от CWD

```
const historyFile = ".zapolnyaka.json"
```

Файл создаётся **в текущей директории запуска**:
- Запуск из `dist/` → `dist/.zapolnyaka.json`
- Запуск из `go_app/` → `go_app/.zapolnyaka.json`

**Всегда запускай из `dist/`.**

## Что сделать прямо сейчас

```powershell
cd D:\MyWork\zapolnyka-en-\dist
.\zapolnyaka.exe auth МОЙ_ЛОГИН МОЙ_ПАРОЛЬ
```

После этого файл будет содержать:
```json
{
  "login": "МОЙ_ЛОГИН",
  "password": "МОЙ_ПАРОЛЬ",
  "lastGame": "...",
  "lastLevel": ""
}
```

С убранным `omitempty` пустая строка всегда записывается явно — credentials видны в файле даже если пустые.
