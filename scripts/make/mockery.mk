ROOT_DIR := $(shell git rev-parse --show-toplevel)
include $(ROOT_DIR)/scripts/make/pkg.mk

.PHONY: mockery.generate
## Generate mocks code.
mockery.generate: _mockery.tools
	@echo "Generating mocks code..."
	@mockery

.PHONY: _mockery.tools
_mockery.tools:
	@if ! command -v mockery &> /dev/null; then \
		echo "Installing mockery..."; \
       	PKG_MANAGER=$$($(MAKE) _pkgtype); \
        if [ "$$PKG_MANAGER" = "brew" ]; then \
          	brew install mockery; \
        else \
        	go install github.com/vektra/mockery/v3@v3.2.5; \
        fi \
	fi
