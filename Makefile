.PHONY: build test lint dev-core clean install help desktop desktop-dev

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
