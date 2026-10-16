# Jana Address

Разбор адресов Jana Post: Go API + React UI + PostgreSQL. Хакатон Terricon Valley, кейс 1.

Сейчас это каркас: главная страница показывает состояние API и базы. Разбор адресов,
справочник и пакетный метод для жюри появятся следующими шагами.

## Запуск

Нужны Docker Engine ≥ 24 и Docker Compose ≥ 2.20.

```bash
git clone --recurse-submodules https://github.com/jana-address/jana-address-api.git
cd jana-address-api
docker compose up
```

- Интерфейс: http://localhost:3000
- API: http://localhost:8080/api/v1/health

Фронтенд лежит в отдельном репозитории `jana-address-web` и подключён submodule в
папку `web/`. Поэтому клонировать нужно с `--recurse-submodules`, а кнопка GitHub
«Download ZIP» не подойдёт: в архив submodule не попадает.

`.env` не обязателен, у всех переменных есть значения по умолчанию. Порты можно
поменять: `WEB_PORT=3001 API_PORT=8081 docker compose up` (см. `.env.example`).

## Без интернета

Интернет нужен только для первой сборки образов:

```bash
docker compose build
```

После этого `docker compose up` работает офлайн. Шрифты подключены локально, внешних
запросов нет ни в UI, ни в API.

## Сервисы

| Сервис | Образ | Порт | Что делает |
|---|---|---|---|
| `web` | nginx-unprivileged 1.30.6 (alpine-slim), uid 101 | 3000 → 8080 | отдаёт React-сборку, проксирует `/api` в `api` |
| `api` | Go 1.26.9 → distroless static nonroot, uid 65532 | 8080 | HTTP API |
| `db` | PostgreSQL 18.6 (alpine) без `gosu`, uid 70 | наружу не публикуется | данные |

Сервисы стартуют по цепочке healthcheck: `db` → `api` → `web`. База сидит во
внутренней сети без выхода наружу. Все контейнеры работают не от root, с
`cap_drop: ALL` и `no-new-privileges`, а `api` и `web` ещё и с файловой системой
только на чтение.

## API

| Метод | Путь | Ответ |
|---|---|---|
| GET | `/livez` | `ok`, если процесс жив |
| GET | `/readyz` | 200, если база отвечает, иначе 503 |
| GET | `/api/v1/health` | `{"status":"ok","version":"0.1.0","db":"ok"}` |

## Тесты и CI

GitHub Actions (`.github/workflows/ci.yml`) запускается на каждый push в `main` и на
pull request:

- **test:**
  - `go mod verify`, `gofmt`, `go vet`;
  - `go test -race` с покрытием против PostgreSQL 18.6 (service container);
  - `govulncheck`.
- **build:**
  - `docker compose config`;
  - сборка образов `api` и `db`;
  - smoke-тест: `docker compose up --wait db api`, затем `/api/v1/health` должен вернуть `"db":"ok"`.

Интеграционный тест базы запускается, только если задан `TEST_DATABASE_URL`:

```bash
TEST_DATABASE_URL=postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable go test ./...
```

Actions закреплены по SHA коммита, а не по тегу.

## Разработка

```bash
# API локально (нужен Go 1.26.9; более старый Go скачает его сам по директиве toolchain)
go test ./...
DATABASE_URL=postgres://jana:jana_local_only@localhost:5432/jana?sslmode=disable go run ./cmd/server

# тесты в контейнере
docker build --target test .

# фронтенд: см. web/README.md
```

После изменений во фронтенде обновите указатель submodule:

```bash
git -C web pull origin main && git add web && git commit -m "feat(web): bump submodule"
```

## Версии

Берём стабильные версии без известных уязвимостей, но не самые свежие. Если
исправление уязвимости есть только в новой версии, берём её. Базовые образы
закреплены по multi-arch digest.

| Компонент | Версия | Новейшая на 17.10.2026 |
|---|---|---|
| Go | 1.26.9 | 1.27.x |
| pgx | v5.10.0 | v5.11.0 |
| golang.org/x/text | v0.41.0 (минимальная с исправлениями GO-2026-5970 и GO-2026-6629) | v0.43.0 |
| PostgreSQL | 18.6 | 19 (RC) |
| React | 19.2.9 | 19.3.x |
| Vite | 8.2.4 | 8.3.x |
| TypeScript | 6.0.3 | 7.0 |
| Node (только сборка) | 24.22.0 LTS | 26.x |
| nginx | 1.30.6 (stable) | 1.31 (mainline) |

Проверка: `govulncheck` и `npm audit` чистые; trivy 0.69.3 не находит HIGH/CRITICAL
ни в одном образе.

## Данные организаторов

`train.json`, `sample_input.json` и `sample_output.json` нельзя выносить за пределы
хакатона. Они в `.gitignore` и `.dockerignore`: в git и в образы не попадают.
Храните их вне репозитория.

## Если что-то не так

- **`unable to prepare context: path "…/web" not found` или пустая папка `web/`.**
  Submodule не скачан, выполните `git submodule update --init`.
- **Порт занят.** Запустите `WEB_PORT=3001 API_PORT=8081 docker compose up`.
- **Начать с чистой базы.** `docker compose down -v` удалит том с данными.
