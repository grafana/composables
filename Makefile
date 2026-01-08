.PHONY: test test-all list-components

# Find all directories containing a Tiltfile
COMPONENTS := $(dir $(wildcard */Tiltfile))

# Test all components
test-all:
	@echo "Testing all composables components..."
	@for dir in $(COMPONENTS); do \
		echo ""; \
		echo "=========================================="; \
		echo "Testing: $$dir"; \
		echo "=========================================="; \
		cd $$dir && tilt ci && tilt down; \
		if [ $$? -eq 0 ]; then \
			echo "✅ $$dir PASSED"; \
		else \
			echo "❌ $$dir FAILED"; \
			exit 1; \
		fi; \
		cd ..; \
	done
	@echo ""
	@echo "=========================================="
	@echo "✅ All components tested successfully!"
	@echo "=========================================="

# Test a specific component
# Usage: make test COMPONENT=nats
test:
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT not specified"; \
		echo "Usage: make test COMPONENT=nats"; \
		exit 1; \
	fi
	@if [ ! -d "$(COMPONENT)" ]; then \
		echo "Error: Component '$(COMPONENT)' not found"; \
		exit 1; \
	fi
	@if [ ! -f "$(COMPONENT)/Tiltfile" ]; then \
		echo "Error: No Tiltfile found in '$(COMPONENT)'"; \
		exit 1; \
	fi
	@echo "Testing component: $(COMPONENT)"
	@cd $(COMPONENT) && tilt ci && tilt down

# List all components
list-components:
	@echo "Available components:"
	@for dir in $(COMPONENTS); do \
		echo "  - $${dir%/}"; \
	done
