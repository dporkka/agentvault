.PHONY: build test lint dev-core clean install help desktop desktop-dev contract-check contract-list-snake

VAULT := ./test-vault
CORE := ./core
DESKTOP := ./apps/desktop-wails
# Ubuntu 24.04+/26.04 ship webkit2gtk 4.1 (not 4.0), so the desktop app needs this build tag.
WAILS_TAGS := webkit2_41

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the agentvault CLI binary
	cd $(CORE) && go build -o ../bin/agentvault ./cmd/agentvault

test: ## Run all Go tests
	cd $(CORE) && go test ./... -v

lint: ## Run go vet and fmt
	cd $(CORE) && go vet ./...
	cd $(CORE) && gofmt -w .

dev-core: ## Run CLI commands against test vault
	@echo "Example: go run ./core/cmd/agentvault init $(VAULT)"

desktop: ## Build the Wails desktop app (requires libgtk-3-dev, libwebkit2gtk-4.1-dev)
	cd $(DESKTOP) && wails build -tags $(WAILS_TAGS)

desktop-dev: ## Run the Wails desktop app in live-dev mode
	cd $(DESKTOP) && wails dev -tags $(WAILS_TAGS)

clean: ## Remove build artifacts and test vault
	rm -rf bin/
	rm -rf $(VAULT)

init-test: build ## Initialize a test vault
	$(CORE)/../bin/agentvault init $(VAULT)

index-test: build ## Index the test vault
	$(CORE)/../bin/agentvault index --vault $(VAULT)

search-test: build ## Search the test vault
	$(CORE)/../bin/agentvault search "test" --vault $(VAULT)

contract-check: ## Verify @agentvault/contract is the only source of API types in clients
	@echo "Checking @agentvault/contract usage..."
	@cd packages/contract && npx --yes -p typescript@5.4.5 tsc --noEmit
	@cd apps/web-local && npx --yes tsc --noEmit
	@cd apps/browser-extension && npx --yes tsc --noEmit
	@cd apps/mobile-expo && npx --yes tsc --noEmit
	@cd apps/desktop-wails/frontend && npx --yes tsc --noEmit
	@echo "Checking for snake_case fields in client code (server emits camelCase)..."
	@SNAKE_RE=$$(scripts/contract-snake-list.sh core/internal/contract/contract.go | paste -sd'|' -); \
	if [ -n "$$SNAKE_RE" ]; then \
	  HITS=$$(grep -RInE "$$SNAKE_RE" apps/web-local/src apps/browser-extension/src apps/mobile-expo/src apps/desktop-wails/frontend/src \
	    --include='*.ts' --include='*.tsx' | head -20); \
	  if [ -n "$$HITS" ]; then \
	    printf '%s\n' "$$HITS"; \
	    echo "Found snake_case keys; server emits camelCase."; \
	    exit 1; \
	  fi; \
	fi
	@echo "Checking for hard-coded API base URLs outside of @agentvault/contract..."
	@! grep -RIn 'http://127.0.0.1:47321' apps/web-local/src apps/browser-extension/src apps/mobile-expo/src apps/desktop-wails/frontend/src \
	  --include='*.ts' --include='*.tsx' | grep -v 'contract/src' || (echo "Found hard-coded base URL; use @agentvault/contract client." && exit 1)
	@echo "Contract check passed."

contract-list-snake: ## Print the snake_case JSON field list derived from Go struct tags
	@scripts/contract-snake-list.sh core/internal/contract/contract.go
