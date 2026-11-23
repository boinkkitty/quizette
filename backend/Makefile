LINT := go tool -modfile=tools/golangci.mod golangci-lint

# Local settings live in ENV_FILE (copy .env.example). Override with `make dev ENV_FILE=.env.other`.
ENV_FILE ?= .env.local
LOAD_ENV = test -f $(ENV_FILE) || { echo "$(ENV_FILE) not found: cp .env.example $(ENV_FILE)" >&2; exit 1; }; set -a; . ./$(ENV_FILE); set +a

.PHONY: run dev gen test test-integration test-race lint fmt bench fixture

run: ## Run the API with the process environment only (as in production)
	go run ./cmd/api

dev: ## Run the API with settings from $(ENV_FILE)
	@$(LOAD_ENV); go run ./cmd/api

gen: ## Regenerate sqlc code
	go tool sqlc generate

test:
	go test ./...

test-race:
	go test -race ./...

test-integration: ## Needs Docker (testcontainers) or TEST_DATABASE_URL
	go test -race -tags integration ./...

lint:
	$(LINT) run ./...

fmt:
	$(LINT) fmt ./...

fixture:
	go run ./cmd/genfixture

bench: fixture ## Compare pdf-native vs text-extract (needs GEMINI_API_KEY in $(ENV_FILE))
	@$(LOAD_ENV); go run ./cmd/genbench -dir testdata/pdfs -runs 3 -count 10
