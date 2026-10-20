.PHONY: run test test-integration vet fmt up down migrate migrate-status

DSN ?= postgres://graphwar:graphwar@127.0.0.1:5432/graphwar

run:
	go run ./cmd/graphwar-server

test:
	go test ./...

test-integration:
	docker compose up -d postgres
	@until docker compose exec -T postgres pg_isready -U graphwar >/dev/null 2>&1; do sleep 0.5; done
	GRAPHWAR_TEST_DSN=$(DSN) go test ./internal/storage/ -v

vet:
	go vet ./...

fmt:
	gofmt -w .

up:
	docker compose up -d postgres

down:
	docker compose down

migrate:
	GRAPHWAR_DB_DSN=$(DSN) go run ./cmd/migrate

migrate-status:
	GRAPHWAR_DB_DSN=$(DSN) go run ./cmd/migrate -status
