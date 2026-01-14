.PHONY: test test-integration test-all clean

test:
	@echo "Running grafana composable unit tests..."
	tilt ci

test-integration:
	@echo "Running grafana composable integration tests..."
	tilt ci -f Tiltfile.integration

test-all: test test-integration
	@echo ""
	@echo "========================================"
	@echo "✅ All tests passed!"
	@echo "========================================"

clean:
	@echo "Cleaning up test resources..."
	tilt down || true
	rm -rf .compose-stage || true
