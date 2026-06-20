BINARY := bin/docs-sign
WEB_DIR := web
EMBED_DIR := internal/web/dist
GO_FILES := $(shell find . -name '*.go' -not -path './web/*' 2>/dev/null)

# Version is derived from git tags: a clean tagged commit yields e.g. "v1.2.3", commits
# past a tag yield "v1.2.3-4-gabc1234", and a repo with no tags falls back to the short
# SHA. Override on the command line with `make build VERSION=v1.2.3` if needed.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
VERSION_PKG := docs-sign/internal/version
GO_LDFLAGS  := -s -w -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT)

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
	CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BINARY) ./cmd/docs-sign
	@echo "built $(BINARY) ($(VERSION))"

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

.PHONY: docker
docker: ## Build the container image locally (single-arch, tagged docs-sign:dev)
	docker build --tag docs-sign:dev .

.PHONY: test
test: ## Run Go tests
	go test ./...

.PHONY: clean
clean: ## Remove build artifacts (keeps ./data and the embed placeholder)
	rm -rf bin
	find $(EMBED_DIR) -mindepth 1 ! -name placeholder.html -delete
