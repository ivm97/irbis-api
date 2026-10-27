# irbis-api

REST-шлюз к серверу **IRBIS64** на Go (Echo).  
Клиентский протокол — [GoIrbis](https://github.com/ivm97/GoIrbis).

Конфиг — только **`.env`** (или переменные окружения). YAML для настроек не используется.

Спека: [`api/openapi.yaml`](api/openapi.yaml) · UI: `/swagger`.

## Быстрый старт

```bash
cp .env.example .env
# поправьте IRBIS_ADDR / AUTH_USERS

go run ./cmd/server
```

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/swagger
```

Путь к env-файлу: `ENV_FILE` (по умолчанию `.env` в рабочей директории).  
Уже заданные переменные окружения **не** перезаписываются из файла.

## .env

```env
HTTP_ADDR=:8080
IRBIS_ADDR=127.0.0.1:6666
IRBIS_WORKSTATION=C
IRBIS_USE_REQUEST_CREDENTIALS=true

AUTH_ALLOW_PLAINTEXT=true
AUTH_USERS=librarian|secret|IBIS=read,write&RDR=read||reader|reader|IBIS=read
```

### Пользователи

Формат: `name|password_or_bcrypt|DB=perms&DB2=perms`, пользователи через `||`.

| Поле | Пример |
|------|--------|
| password | `secret` (если `AUTH_ALLOW_PLAINTEXT=true`) |
| bcrypt | `$2a$10$...` (префикс `$2`) |
| права | `IBIS=read,write` |

Или файл построчно (`AUTH_USERS_FILE=users.txt`):

```
librarian|secret|IBIS=read,write&RDR=read
reader|reader|IBIS=read
```

Хеш: `go run ./cmd/hashpwd 'my-password'`.

### Авторизация с фронта

```js
await fetch('http://localhost:8080/v1/login', {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: 'librarian', password: 'secret', workstation: 'C' }),
});
// дальше все запросы с credentials: 'include' — куки уходят сами
```

В `.env` API: `CORS_ALLOW_ORIGIN=http://localhost:5173`.  
Заголовки `X-API-User` / `X-API-Password` тоже работают (curl).

### АРМ

Дефолт: `IRBIS_WORKSTATION=C`. Иначе кука/логин/`X-Irbis-Workstation` (`A|C|M|R|B|K`).

## Эндпоинты

| Метод | Путь | Право |
|-------|------|-------|
| GET | `/health` | — |
| POST | `/v1/login` | — |
| POST | `/v1/logout` | — |
| GET | `/v1/me` | auth |
| GET | `/v1/config` | auth |
| GET/POST | `/v1/databases/{db}/records/search` | read |
| GET | `/v1/databases/{db}/records/count` | read |
| GET/PUT/DELETE | `/v1/databases/{db}/records/{mfn}` | read / write |
| POST | `/v1/databases/{db}/records` | write |
| PUT | `/v1/databases/{db}/records/batch` | write |
| POST | `/v1/databases/{db}/records/gbl` | write |
| GET | `/v1/databases/{db}/terms` | read |
| GET | `/v1/print/templates` | auth |
| POST | `/v1/databases/{db}/print` | read |

Запись — массив `fields` с числовым `tag` (удобно для повторов MARC/IRBIS).

Печать: шаблоны RTF из `TEMPLATES_DIR` (по умолчанию `templates/`), плейсхолдеры `#key#`; в `dopshab` блок `#D2#`…`#2D#` размножается по экземплярам (910).

## OpenAPI

```bash
go generate ./api/
```

## Тесты

```bash
go test ./...
```
