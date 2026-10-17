# Jana Address на Go

Один Go-сервер: отдаёт интерфейс «Разбор адресов» (RU / KK / EN) и API разбора.

    go run .                      # http://localhost:8080
    PORT=3000 go run .            # другой порт

API:

    GET  /api/v1/health
    POST /api/v1/parse         {"raw": "Кулманова 133 flat 191"}
    POST /api/v1/parse/batch   [{"id":"1","raw":"..."}]  или  {"records":[...]}   (до 10 000 записей)

Для проверки загрузки файла используйте `sample_input.json`. Адрес читается из поля `raw`, `address` или `text`.

Интерфейс вшит в бинарник (`web/index.html`, go:embed). Разбор — правила в `parser.go`, без внешних зависимостей.
