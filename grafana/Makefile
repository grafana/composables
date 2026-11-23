.PHONY: test
test: integration-test integration-test-users integration-test-ini

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)

.PHONY: integration-test-users
integration-test-users:
		@(cd test; tilt -f Tiltfile.integration-users down 2>/dev/null || true; tilt -f Tiltfile.integration-users ci; tilt -f Tiltfile.integration-users down)

.PHONY: integration-test-ini
integration-test-ini:
		@(cd test; tilt -f Tiltfile.integration-ini down 2>/dev/null || true; rc=0; tilt -f Tiltfile.integration-ini ci || rc=$$?; tilt -f Tiltfile.integration-ini down; exit $$rc)
