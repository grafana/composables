.PHONY: test test-integration test-integration-users test-all clean

test:
	@echo "Running grafana composable unit tests..."
	tilt ci

test-integration:
	@echo "Running grafana composable integration tests..."
	tilt ci -f Tiltfile.integration

test-integration-users:
	@echo "Running user provisioning integration tests..."
	tilt -f Tiltfile.integration-users down 2>/dev/null || true
	tilt ci -f Tiltfile.integration-users

test-all: test test-integration test-integration-users
	@echo ""
	@echo "========================================"
	@echo "All tests passed!"
	@echo "========================================"

clean:
	@echo "Cleaning up test resources..."
	tilt down || true
	rm -rf .cc || true
