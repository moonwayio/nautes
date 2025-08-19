ROOT_DIR := $(shell git rev-parse --show-toplevel)
include $(ROOT_DIR)/scripts/make/help.mk
include $(ROOT_DIR)/scripts/make/mockery.mk
include $(ROOT_DIR)/scripts/make/pre-commit.mk
include $(ROOT_DIR)/scripts/make/vuln.mk

.PHONY: tools
## Install development tools
tools:
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "Development tools installed!"

.PHONY: generate
## Generate code
generate:
	@$(MAKE) mockery.generate || true
	@go generate ./...

.PHONY: lint
## Run linters
lint:
	@golangci-lint run ./... --fix

.PHONY: modcheck
# Check if go.mod and go.sum are tidy
modcheck: tidy
	@git diff --exit-code go.mod go.sum

.PHONY: test
## Run tests
test:
	@go test ./... -covermode=count -coverprofile=coverage.txt -timeout 30s -v

.PHONY: test-ci
## Run tests for CI
test-ci:
	@go test ./... -covermode=atomic -coverprofile=coverage.txt -timeout 30s -race -v -count=1

.PHONY: clean
## Clean up
clean:
	@git clean -Xdf

.PHONY: tidy
## Tidy up go modules
tidy:
	@go mod tidy
