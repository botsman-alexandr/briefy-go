# BrieflyGO

Хук `UserPromptSubmit`: inject CAVEMAN-контекста в каждую сессию + счетчик ходов на сессию.

## Как работает

- Каждый промпт -> хук получает `session_id` из stdin, увеличивает `turnCount`.
- Turn 1: инжект полный core из `BRIFLY.md` (секции `CORE` + `EXAMPLES`).
- Каждый 10-й ход: повторный инжект core (анти-drift).
- Остальные ходы: короткий `brief` из секции `CONFIG`.
- Состояние: `briefly_logs/db.json` (trim до `maxSessions` по `lastSeen`).

## Файлы

| Файл | Назначение |
|---|---|
| `main.go` | Хук. Fail-open: любая ошибка -> тихий exit 0 |
| `BRIFLY.md` | Конфиг: `CONFIG` (maxSessions, brief), `CORE` (правила), `EXAMPLES` |

## Установка

1. Развернуть бинарник и конфиг в hooks dir (пусть совпадает с `command` в `settings.json`):
   ```
   <hooks-dir>/briefly-go/
   ├── briefly-go.exe   (Linux/macOS: briefly-go)
   └── BRIFLY.md
   ```
   Сборка:
   ```
   go build -o <hooks-dir>/briefly-go/briefly-go.exe .
   ```

2. Регистрация в `settings.json`:
   ```json
   {
     "hooksConfig": { "enabled": true },
     "hooks": {
       "UserPromptSubmit": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "<absolute-path>/<hooks-dir>/briefly-go/briefly-go.exe",
               "description": "Счетчик запросов и reinforcement BrieflyGO",
               "timeout": 1000
             }
           ]
         }
       ]
     }
   }
   ```
   Unix: `"command": "$HOME/<hooks-dir>/briefly-go/briefly-go"`.

3. Перезапустить сессию. Проверка: первый ответ в caveman-стиле без промпта.

## Windows: AppLocker блокирует .exe

Если несигнированные exe не запускаются («Программа заблокирована групповой политикой», exit 255) — сборка и запуск через `go run`. Временный бинарник `go run` в `%TEMP%` проходит политику.

Нюансы:

- `go run .` не работает — cwd хука неизвестен. Только абсолютный путь к `main.go`.
- Холодный старт `go run` компилирует: `timeout` 1000 -> 5000.
- `main.go` знает, что при `go run` exe лежит в `%TEMP%`: если рядом нет `BRIFLY.md`, конфиг ищется по fallback-пути из `resolveBaseDir()` (там, где лежат `BRIFLY.md` и `briefly_logs`).

Вариант A — напрямую:

```json
{
  "type": "command",
  "command": "go run C:/Users/<user>/path/to/briefy-go/main.go",
  "description": "Счетчик запросов и reinforcement BrieflyGO",
  "timeout": 5000
}
```

Вариант B — если `go` не резолвится из PATH, обертка-батник
`<hooks-dir>/briefly-go/briefly.cmd`:

```bat
@go run C:\Users\<user>\path\to\briefy-go\main.go
```

```json
{
  "type": "command",
  "command": "C:/Users/<user>/<hooks-dir>/briefly-go/briefly.cmd",
  "description": "Счетчик запросов и reinforcement BrieflyGO",
  "timeout": 5000
}
```

## Диагностика

| Симптом | Причина |
|---|---|
| `briefly_logs/` не появляется рядом с бинарником | Хук не вызывается: путь в `settings.json` неверен или exe блокируется политикой |
| Exit 255 + «заблокирована групповой политикой» | AppLocker/SRP: переход на `go run` (раздел выше) |
| Контекст только `brief`, core не инжектится | Хук не нашел `BRIFLY.md`: имя файла должно быть ровно `BRIFLY.md` (без E) |
| db.json растет | Trim работает по `lastSeen`; `maxSessions` в секции `CONFIG` |
