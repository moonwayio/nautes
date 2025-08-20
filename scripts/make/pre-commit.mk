ROOT_DIR := $(shell git rev-parse --show-toplevel)
include $(ROOT_DIR)/scripts/make/pkg.mk

.PHONY: pre-commit.install
## Install pre-commit hooks
pre-commit.install: _pre_commit.tools
	@pre-commit install

.PHONY: _pre_commit.tools
_pre_commit.tools:
	@if ! command -v pre-commit &> /dev/null; then \
		echo "Installing pre-commit..."; \
       	PKG_MANAGER=$$($(MAKE) _pkgtype); \
        if [ "$$PKG_MANAGER" = "brew" ]; then \
          	brew install pre-commit; \
        else \
        	echo "Don't know how to install dependencies for $$PKG_MANAGER"; \
        	exit 1; \
        fi \
	fi
