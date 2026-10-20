# Changelog

Все заметные изменения backend описываются в этом файле.
Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).

## [Unreleased]

### Added

- B01: каркас Go-сервиса `graphwar-server` — конфигурация из окружения (`GRAPHWAR_ADDR`, `GRAPHWAR_LOG_LEVEL`), JSON-логи через `slog`, health-эндпоинт `GET /healthz`, корректное завершение по SIGINT/SIGTERM.
- Тесты конфигурации и HTTP-обработчиков (`go test ./...`), команды Makefile: `run`, `test`, `vet`, `fmt`.
