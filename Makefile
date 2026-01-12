# Find all directories containing both a Tiltfile and a Makefile
SUBDIRS := $(patsubst %/,%,$(dir $(wildcard */Makefile)))

# ANSI color codes
GREEN := \033[0;32m
RED := \033[0;31m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m

.PHONY: test list-components

# Test all composables - runs all tests and shows summary at end
test:
	@printf "Running tests for $(words $(SUBDIRS)) composables...\n"
	@printf "\n"
	@failed=""; \
	for dir in $(SUBDIRS); do \
		printf "$(BLUE)Testing$(NC) %-30s ... " "$$dir"; \
		if $(MAKE) -C $$dir test > /tmp/test-$$dir.log 2>&1; then \
			printf "$(GREEN)✓ PASS$(NC)\n"; \
			rm -f /tmp/test-$$dir.log; \
		else \
			printf "$(RED)✗ FAIL$(NC)\n"; \
			failed="$$failed $$dir"; \
		fi; \
	done; \
	printf "\n"; \
	if [ -n "$$failed" ]; then \
		printf "$(RED)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)\n"; \
		printf "$(RED)Failed tests:$$failed$(NC)\n"; \
		printf "$(RED)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)\n"; \
		for dir in $$failed; do \
			printf "\n"; \
			printf "$(RED)───── Output from $$dir ─────$(NC)\n"; \
			cat /tmp/test-$$dir.log 2>/dev/null || printf "Log missing\n"; \
			rm -f /tmp/test-$$dir.log; \
		done; \
		printf "\n"; \
		printf "$(RED)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)\n"; \
		exit 1; \
	else \
		printf "$(GREEN)✓ All $(words $(SUBDIRS)) tests passed!$(NC)\n"; \
	fi

# List all components
list-components:
	@echo "Available components:"
	@$(foreach dir,$(SUBDIRS),echo "  - $(dir)";)
