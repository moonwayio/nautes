GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)
TARGET_MAX_CHAR_NUM=2

.PHONY: help
## Show this help
help:
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk '\
    		/^[a-zA-Z0-9._-]+:/ { \
    			if (match(lastLine, /^## (.*)/)) { \
    				helpCommand = substr($$1, 0, index($$1, ":") - 1); \
    				helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
    				printf "  ${YELLOW}%-$(TARGET_MAX_CHAR_NUM)s${RESET} ${GREEN}%s${RESET}\n", helpCommand, helpMessage; \
    			} \
    		} \
    		{ lastLine = $$0 }' $(MAKEFILE_LIST)
