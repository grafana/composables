.PHONY: test
test: integration-test integration-test-users

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)

.PHONY: integration-test-users
integration-test-users:
		@(cd test; tilt -f Tiltfile.integration-users down 2>/dev/null || true; tilt -f Tiltfile.integration-users ci; tilt -f Tiltfile.integration-users down)
