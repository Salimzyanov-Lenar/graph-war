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

Невалидная конфигурация прерывает запуск с понятной ошибкой в stderr.

## Разработка

```sh
make fmt    # gofmt
make vet    # go vet ./...
make test   # go test ./...
make run    # запуск сервера
```
