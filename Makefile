.PHONY: help install uninstall build build-api build-worker build-migrate \
        run-api run-worker test test-race bench lint fmt vet tidy \
        migrate-up migrate-down migrate-status seed-disposable \
        smoke clean

# ────────────────────────────────────────────────────────────────────────
# Variáveis
# ────────────────────────────────────────────────────────────────────────
BIN_DIR        ?= ./bin
INSTALL_BIN    ?= /opt/mailclear/bin
ETC_DIR        ?= /etc/mailclear
ENV_FILE       ?= $(ETC_DIR)/.env
GO             ?= go
GOFLAGS        ?= -trimpath -ldflags="-s -w"

# Detecta versão e commit (se git disponível)
VERSION        := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS_VER    := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

# ────────────────────────────────────────────────────────────────────────
# Help
# ────────────────────────────────────────────────────────────────────────
help: ## Mostra esta ajuda
	@echo "MailClear — Targets disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ────────────────────────────────────────────────────────────────────────
# Setup / Install
# ────────────────────────────────────────────────────────────────────────
install: ## Roda install.sh (precisa sudo)
	sudo bash install.sh

uninstall: ## Desfaz a instalação
	sudo bash uninstall.sh

# ────────────────────────────────────────────────────────────────────────
# Build
# ────────────────────────────────────────────────────────────────────────
build: build-api build-worker build-migrate build-seed ## Compila todos os binários

build-api: ## Compila mailclear-api
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags="-s -w $(LDFLAGS_VER)" -o $(BIN_DIR)/mailclear-api ./cmd/server

build-worker: ## Compila mailclear-worker
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags="-s -w $(LDFLAGS_VER)" -o $(BIN_DIR)/mailclear-worker ./cmd/worker

build-migrate: ## Compila mailclear-migrate
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags="-s -w $(LDFLAGS_VER)" -o $(BIN_DIR)/mailclear-migrate ./cmd/migrate

build-seed: ## Compila mailclear-seed-disposable
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags="-s -w $(LDFLAGS_VER)" -o $(BIN_DIR)/mailclear-seed-disposable ./cmd/seed-disposable

# ────────────────────────────────────────────────────────────────────────
# Run (foreground, dev)
# ────────────────────────────────────────────────────────────────────────
run-api: build-api ## Roda API em foreground (dev)
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-api

run-worker: build-worker ## Roda worker em foreground (dev)
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-worker

# ────────────────────────────────────────────────────────────────────────
# Qualidade
# ────────────────────────────────────────────────────────────────────────
test: ## Roda testes
	$(GO) test -count=1 ./...

test-race: ## Roda testes com -race
	$(GO) test -race -count=1 ./...

bench: ## Roda benchmarks
	$(GO) test -bench=. -benchmem ./internal/validation/... ./internal/dns/...

lint: ## Roda golangci-lint (se instalado)
	@which golangci-lint > /dev/null || { echo "golangci-lint não instalado"; exit 1; }
	golangci-lint run

fmt: ## Formata o código
	$(GO) fmt ./...

vet: ## Roda go vet
	$(GO) vet ./...

tidy: ## go mod tidy
	$(GO) mod tidy

# ────────────────────────────────────────────────────────────────────────
# Banco
# ────────────────────────────────────────────────────────────────────────
migrate-up: build-migrate ## Aplica migrations
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-migrate up

migrate-down: build-migrate ## Reverte 1 migration
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-migrate down 1

migrate-status: build-migrate ## Mostra status das migrations
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-migrate version

# ────────────────────────────────────────────────────────────────────────
# Operações
# ────────────────────────────────────────────────────────────────────────
seed-disposable: build-seed ## Atualiza lista de disposable do GitHub
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; $(BIN_DIR)/mailclear-seed-disposable

smoke: ## Smoke test (curl /readyz)
	@bash scripts/bench.sh

# ────────────────────────────────────────────────────────────────────────
# Limpeza
# ────────────────────────────────────────────────────────────────────────
clean: ## Remove binários e arquivos gerados
	rm -rf $(BIN_DIR) coverage.out coverage.html
