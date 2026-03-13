COMPOSE_FILE=infra/docker-compose.yml

ifeq ($(OS),Windows_NT)
INTEGRATION_CMD=powershell -ExecutionPolicy Bypass -File .\scripts\integration.ps1
else
INTEGRATION_CMD=./scripts/integration.sh
endif

.PHONY: up down test lint integration verify

up:
	docker compose -f $(COMPOSE_FILE) up -d --build

down:
	docker compose -f $(COMPOSE_FILE) down

test:
	go test ./...

lint:
	golangci-lint run ./...

integration:
	$(INTEGRATION_CMD)

verify:
	$(MAKE) test
	$(MAKE) integration
