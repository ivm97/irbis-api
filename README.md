# irbis-api

REST-шлюз к серверу **IRBIS64** на Go (Echo).  
Клиентский протокол — модуль [GoIrbis](https://github.com/ivm97/GoIrbis).

Авторизация API — заголовки + локальный файл пользователей.  
АРМ (workstation) по умолчанию — Каталогизатор (`C`); на запрос можно передать `X-Irbis-Workstation`.

Спека: [`api/openapi.yaml`](api/openapi.yaml) · Swagger UI: `/swagger` · OpenAPI: `/openapi.yaml`.

## Быстрый старт

```bash
cp configs/config.example.yaml configs/config.yaml
cp configs/users.example.yaml configs/users.yaml
cp .env.example .env   # опционально: IRBIS_WORKSTATION=C и др.

go run ./cmd/server -config configs/config.yaml
```

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/swagger
```

## Авторизация и АРМ

| Header | Описание |
|--------|----------|
| `X-API-User` | логин API |
| `X-API-Password` | пароль API |
| `X-Irbis-Workstation` | опционально: `A\|C\|M\|R\|B\|K`. Иначе дефолт из конфига / `IRBIS_WORKSTATION` (`C`) |

Права в `users.yaml`: `read` / `write` (write включает read) на базу.

## Эндпоинты

| Метод | Путь | Право | Описание |
|-------|------|-------|----------|
| GET | `/health` | — | liveness |
| GET | `/v1/me` | auth | профиль API-юзера и права на базы |
| GET | `/v1/config` | auth | АРМы, search modes, метаданные IRBIS (версия, список БД) |
| GET/POST | `/v1/databases/{db}/records/search` | read | поиск (сырое expression и/или mode+value), `limit`/`offset`, `total` |
| GET | `/v1/databases/{db}/records/count` | read | число найденных |
| GET | `/v1/databases/{db}/records/{mfn}` | read | запись (массив `fields` с `tag`) |
| POST | `/v1/databases/{db}/records` | write | создать |
| PUT | `/v1/databases/{db}/records/{mfn}` | write | заменить |
| DELETE | `/v1/databases/{db}/records/{mfn}` | write | логическое удаление |
| PUT | `/v1/databases/{db}/records/batch` | write | пакетная запись |
| POST | `/v1/databases/{db}/records/gbl` | write | глобальная корректировка (GBL) |
| GET | `/v1/databases/{db}/terms` | read | словарь |

### Поиск

- `expression` — сырое выражение IRBIS (оставляем).
- `mode` + `value` — безопасные префиксы (`author`→`A=`, `title`→`T=`, …, `all`→`*`).
- `offset` / `limit` → IRBIS `FirstRecord = offset+1`, `NumberOfRecords = limit`.
- В ответе: `count` (страница), `total` (всего на сервере), `hits`.

### Формат записи

```json
{
  "database": "IBIS",
  "mfn": 12,
  "fields": [
    { "tag": 200, "subfields": [{ "code": "a", "value": "Заглавие" }] },
    { "tag": 700, "subfields": [{ "code": "a", "value": "Иванов" }] }
  ]
}
```

### GBL

```bash
curl -s -X POST \
  -H 'X-API-User: librarian' -H 'X-API-Password: secret' \
  -H 'Content-Type: application/json' \
  -d '{"mode":"author","value":"Test","min_mfn":1,"max_mfn":100,"actualize":true,"statements":[{"command":"ADD","parameter1":"907","parameter2":"*","format1":"'\''x'\''"}]}' \
  http://localhost:8080/v1/databases/IBIS/records/gbl
```

Либо `"filename": "task.gbl"` (серверный файл).

## Конфиг и env

YAML + overlay из окружения / `.env` (процессный env имеет приоритет над `.env`):

- `IRBIS_HOST`, `IRBIS_PORT`, `IRBIS_WORKSTATION`, `IRBIS_IO_TIMEOUT`, …
- см. [`.env.example`](.env.example)

## Генерация из OpenAPI

```bash
go generate ./api/
```

Ручки — Echo (`echo.Context`), код в [`api/gen`](api/gen), адаптер в `internal/adapter/httpapi/oapi_server.go`.

## Тесты

```bash
go test ./...
```

Юнит/HTTP-тесты с моком каталога (живой IRBIS не нужен).
