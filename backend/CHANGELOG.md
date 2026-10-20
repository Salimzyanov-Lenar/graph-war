# Changelog

Все заметные изменения backend описываются в этом файле.
Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).

## [Unreleased]

### Added

- B02: парсер формул `internal/expr` — лексер и рекурсивный спуск по грамматике контракта (приоритеты, правоассоциативный `^`, унарный минус `-x^2 = -(x^2)`), AST с лимитами (100 символов / 64 узла / глубина 16), позиции ошибок, запрет неявного умножения; тесты: таблица валидных/невалидных формул, лимиты, детерминированный фузз без паник.
- B15: PostgreSQL-слой — Docker Compose (`postgres:17-alpine`, только localhost), миграции goose (`matches`, `match_participants`), типизированные запросы через sqlc v1.31.1 + pgx v5.11.0, команда `cmd/migrate`, разделение `GET /healthz` (liveness) и `GET /readyz` (readiness: ping БД). Интеграционный тест хранилища против настоящего PostgreSQL (`make test-integration`).
- B01: каркас Go-сервиса `graphwar-server` — конфигурация из окружения (`GRAPHWAR_ADDR`, `GRAPHWAR_LOG_LEVEL`), JSON-логи через `slog`, health-эндпоинт `GET /healthz`, корректное завершение по SIGINT/SIGTERM.
- Тесты конфигурации и HTTP-обработчиков (`go test ./...`), команды Makefile: `run`, `test`, `vet`, `fmt`.
