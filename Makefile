BINARY := bin/docs-sign
WEB_DIR := web
EMBED_DIR := internal/web/dist
GO_FILES := $(shell find . -name '*.go' -not -path './web/*' 2>/dev/null)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## Install frontend deps and tidy go modules
	cd $(WEB_DIR) && npm install
	go mod tidy

.PHONY: web
web: ## Build the frontend into the embed dir
	find $(EMBED_DIR) -mindepth 1 ! -name placeholder.html -delete
	cd $(WEB_DIR) && npm run build

.PHONY: build
build: web ## Build the single self-contained binary (frontend embedded)
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/docs-sign
	@echo "built $(BINARY)"

.PHONY: run
run: build ## Build and run with a local ./data dir
	./$(BINARY) --data ./data --addr 127.0.0.1:8080

.PHONY: dev-server
dev-server: ## Run the Go API server (expects frontend on the Vite dev server)
	go run ./cmd/docs-sign --data ./data --addr 127.0.0.1:8080 --dev

.PHONY: dev-web
dev-web: ## Run the Vite dev server with API proxy
	cd $(WEB_DIR) && npm run dev

.PHONY: licenses
licenses: ## Regenerate THIRD_PARTY_LICENSES from the module cache and node_modules
	./scripts/gen-third-party-licenses.sh

.PHONY: test
test: ## Run Go tests
	go test ./...

.PHONY: clean
clean: ## Remove build artifacts (keeps ./data and the embed placeholder)
	rm -rf bin
	find $(EMBED_DIR) -mindepth 1 ! -name placeholder.html -delete
