ROOT_DIR := $(shell git rev-parse --show-toplevel)
include $(ROOT_DIR)/scripts/make/pkg.mk

.PHONY: vuln.check
## Check for vulnerabilities
vuln.check: _vuln.tools
	@echo "Checking for vulnerabilities..."
	@govulncheck -test ./...

.PHONY: _vuln.tools
_vuln.tools:
	@if ! command -v govulncheck &> /dev/null; then \
		echo "Installing govulncheck..."; \
        go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
