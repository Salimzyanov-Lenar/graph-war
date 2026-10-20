# Graphwar backend

Go-сервис онлайн-игры Graphwar. План работ — [tasks.md](./tasks.md), правила совместной разработки — [../docs/plan.md](../docs/plan.md), термины — [../CONTEXT.md](../CONTEXT.md).

## Запуск

```sh
go run ./cmd/graphwar-server
```

Сервер слушает `:8080`. Проверка здоровья:

```sh
curl -s http://localhost:8080/healthz
# {"status":"ok"}
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRAPHWAR_ADDR` | `:8080` | Адрес прослушивания `host:port` |
| `GRAPHWAR_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `GRAPHWAR_DB_DSN` | пусто | DSN PostgreSQL; пусто — сервер работает без БД |
| `GRAPHWAR_COOKIE_SECURE` | `false` | `true` за HTTPS (Secure-атрибут cookie сессии) |

Невалидная конфигурация прерывает запуск с понятной ошибкой в stderr.

## База данных

```sh
make up               # PostgreSQL в Docker (только 127.0.0.1:5432)
make migrate          # применить миграции (goose)
make migrate-status   # статус миграций
make down             # остановить
```

Схема — в `migrations/*.sql`, запросы — в `queries/*.sql`; Go-код генерируется из SQL командой:

```sh
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Интеграционные тесты идут против настоящего PostgreSQL (без SQLite-замен):

```sh
make test-integration
```

## Проверки здоровья

- `GET /healthz` — liveness: процесс жив (всегда 200, пока слушает).
- `GET /readyz` — readiness: без `GRAPHWAR_DB_DSN` → 200 `{"database":"disabled"}`; с недоступной БД → 503 `{"database":"error"}`; иначе 200 `{"database":"ok"}`.

## Разработка

```sh
make fmt    # gofmt
make vet    # go vet ./...
make test   # go test ./... (unit)
make run    # запуск сервера
```
