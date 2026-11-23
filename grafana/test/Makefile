.PHONY: test test-integration test-integration-users test-integration-ini test-all clean

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

test-integration-ini:
	@echo "Running generated ini integration tests..."
	tilt -f Tiltfile.integration-ini down 2>/dev/null || true
	tilt ci -f Tiltfile.integration-ini

test-all: test test-integration test-integration-users test-integration-ini
	@echo ""
	@echo "========================================"
	@echo "All tests passed!"
	@echo "========================================"

clean:
	@echo "Cleaning up test resources..."
	tilt down || true
	rm -rf .cc || true
