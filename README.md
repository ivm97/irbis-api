# irbis-api

REST-шлюз к серверу **IRBIS64** на Go (Echo).  
Клиентский протокол — [GoIrbis](https://github.com/ivm97/GoIrbis).

Конфиг — только **`.env`** (или переменные окружения). YAML для настроек не используется.

Спека: [`api/openapi.yaml`](api/openapi.yaml) · UI: `/swagger`.

## Быстрый старт

```bash
cp .env.example .env
# поправьте IRBIS_ADDR и IRBIS_SERVICE_USERNAME/PASSWORD

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
IRBIS_SERVICE_USERNAME=admin
IRBIS_SERVICE_PASSWORD=secret
```

### Пользователи и права (IRBIS ACL)

По умолчанию API читает пользователей с сервера (`client_m` через admin-учётку из env):

1. пароль — из списка пользователей IRBIS;
2. на первом запросе с данным АРМ — ini `IRBIS{ARM}_{user}.INI` (корень, path `0`) → `DBNNAMECAT`;
3. меню баз `db_….mnu` из DATAI (path `1`); `-ИМЯ` = только чтение; нет меню → `IRBIS_DATABASES_MENU` (`dbnam2.mnu`);
4. кэш `(user, ARM)` на `IRBIS_ACL_CACHE_TTL` (по умолчанию 5m).

Опциональный локальный fallback: `AUTH_USERS` / `AUTH_USERS_FILE` (если заданы — **только файл**, без ограничений IRBIS по АРМ/mnu).

Формат: `name|password_or_bcrypt|DB=perms&DB2=perms`, через `||` (или построчно в файле).  
Пустой ACL / `*` / два поля `name|pass` → `unrestricted` (любая БД, любой АРМ).  
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
