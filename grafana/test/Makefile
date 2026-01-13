.PHONY: test clean

test:
	@echo "Running grafana composable tests..."
	tilt ci

clean:
	@echo "Cleaning up test resources..."
	tilt down || true
