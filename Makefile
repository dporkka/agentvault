.PHONY: build test test-ci bench smoke-test lint fmt tidy dev-core clean install help desktop desktop-dev ci contract-check contract-list-snake release release-cli release-cli-linux release-cli-darwin release-cli-windows release-extension release-desktop-linux release-desktop-linux-tar release-mobile

VAULT := ./test-vault
CORE := ./core
DESKTOP := ./apps/desktop-wails
# Ubuntu 24.04+/26.04 ship webkit2gtk 4.1 (not 4.0), so the desktop app needs this build tag.
WAILS_TAGS := webkit2_41

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

ci: lint test-ci contract-check ## Run the same checks as CI locally

build: ## Build the agentvault CLI binary
	cd $(CORE) && go build -o ../bin/agentvault ./cmd/agentvault

test: ## Run all Go tests (verbose)
	cd $(CORE) && go test ./... -v

test-ci: ## Run Go tests with race detection and cache disabled (CI mode)
	cd $(CORE) && go test -race -count=1 ./...

bench: ## Run Go benchmarks for core operations
	cd $(CORE) && go test -bench=. -benchmem -run=^$$ ./internal/indexer ./internal/search ./internal/importers ./internal/vectors

smoke-test: ## Run smoke tests on packaged CLI artifacts
	./scripts/smoke-test.sh

lint: ## Run read-only Go checks (vet + gofmt)
	cd $(CORE) && go vet ./...
	cd $(DESKTOP) && go vet ./...
	@UNFMT_CORE=$$(cd $(CORE) && gofmt -l .); \
	UNFMT_DESKTOP=$$(cd $(DESKTOP) && gofmt -l .); \
	if [ -n "$$UNFMT_CORE" ] || [ -n "$$UNFMT_DESKTOP" ]; then \
		echo "Go files not formatted:"; \
		printf '%s\n' "$$UNFMT_CORE" "$$UNFMT_DESKTOP"; \
		exit 1; \
	fi
	@echo "Go lint passed."

fmt: ## Format all Go files
	cd $(CORE) && gofmt -w .
	cd $(DESKTOP) && gofmt -w .

tidy: ## Tidy and verify Go modules
	cd $(CORE) && go mod tidy && go mod verify
	cd $(DESKTOP) && go mod tidy && go mod verify

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
	@cd apps/web-local && npx tsc --noEmit
	@cd apps/browser-extension && npx tsc --noEmit
	@cd apps/mobile-expo && npx tsc --noEmit
	@cd apps/desktop-wails/frontend && npx tsc --noEmit
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

check-secrets: ## Check whether GitHub Actions publishing secrets are configured
	@scripts/check-publish-secrets.sh

# Release scaffolding
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
DIST_DIR := $(CURDIR)/dist

release: release-cli release-extension release-desktop-linux release-mobile ## Build all release artifacts

release-cli: release-cli-linux release-cli-darwin release-cli-windows ## Build CLI archives for all platforms

release-cli-linux: ## Build Linux CLI archives
	@mkdir -p $(DIST_DIR)/cli
	cd $(CORE) && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/cli/agentvault-linux-amd64 ./cmd/agentvault
	cd $(CORE) && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/cli/agentvault-linux-arm64 ./cmd/agentvault
	cp LICENSE $(DIST_DIR)/cli/LICENSE
	cd $(DIST_DIR)/cli && tar -czf agentvault-$(VERSION)-linux-amd64.tar.gz agentvault-linux-amd64 LICENSE
	cd $(DIST_DIR)/cli && tar -czf agentvault-$(VERSION)-linux-arm64.tar.gz agentvault-linux-arm64 LICENSE

release-cli-darwin: ## Build macOS CLI archives
	@mkdir -p $(DIST_DIR)/cli
	cd $(CORE) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/cli/agentvault-darwin-amd64 ./cmd/agentvault
	cd $(CORE) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/cli/agentvault-darwin-arm64 ./cmd/agentvault
	cp LICENSE $(DIST_DIR)/cli/LICENSE
	cd $(DIST_DIR)/cli && tar -czf agentvault-$(VERSION)-darwin-amd64.tar.gz agentvault-darwin-amd64 LICENSE
	cd $(DIST_DIR)/cli && tar -czf agentvault-$(VERSION)-darwin-arm64.tar.gz agentvault-darwin-arm64 LICENSE

release-cli-windows: ## Build Windows CLI archives
	@mkdir -p $(DIST_DIR)/cli
	cd $(CORE) && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/cli/agentvault-windows-amd64.exe ./cmd/agentvault
	cp LICENSE $(DIST_DIR)/cli/LICENSE
	cd $(DIST_DIR)/cli && zip agentvault-$(VERSION)-windows-amd64.zip agentvault-windows-amd64.exe LICENSE

release-extension: ## Build and package the browser extension
	@mkdir -p $(DIST_DIR)/extension
	cd apps/browser-extension && npm run build
	cd apps/browser-extension/dist && zip -r $(DIST_DIR)/extension/agentvault-extension-$(VERSION).zip .

release-desktop-linux: ## Build the Linux desktop binary (requires libgtk-3-dev, libwebkit2gtk-4.1-dev)
	@mkdir -p $(DIST_DIR)/desktop
	cd $(DESKTOP)/frontend && npm ci && npm run build
	cd $(DESKTOP) && go build -tags $(WAILS_TAGS) -o $(DIST_DIR)/desktop/agentvault-desktop-linux-amd64 .
	cp LICENSE $(DIST_DIR)/desktop/LICENSE

release-desktop-linux-tar: release-desktop-linux ## Package the Linux desktop binary as a tar.gz
	cd $(DIST_DIR)/desktop && tar -czf agentvault-desktop-$(VERSION)-linux-amd64.tar.gz agentvault-desktop-linux-amd64 LICENSE

release-mobile: ## Export mobile bundles (requires Expo/EAS setup for store builds)
	@mkdir -p $(DIST_DIR)/mobile
	cd apps/mobile-expo && npm ci
	cd apps/mobile-expo && npx expo export --platform ios --output-dir $(DIST_DIR)/mobile/ios
	cd apps/mobile-expo && npx expo export --platform android --output-dir $(DIST_DIR)/mobile/android
	cd $(DIST_DIR)/mobile && zip -r agentvault-mobile-$(VERSION)-ios.zip ios
	cd $(DIST_DIR)/mobile && zip -r agentvault-mobile-$(VERSION)-android.zip android
