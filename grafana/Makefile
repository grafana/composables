.PHONY: test
test: integration-test integration-test-users integration-test-ini

.PHONY: integration-test
integration-test:
		@(cd test; rc=0; tilt -f Tiltfile.integration ci || rc=$$?; tilt -f Tiltfile.integration down; exit $$rc)

.PHONY: integration-test-users
integration-test-users:
		@(cd test; tilt -f Tiltfile.integration-users down 2>/dev/null || true; rc=0; tilt -f Tiltfile.integration-users ci || rc=$$?; tilt -f Tiltfile.integration-users down; exit $$rc)

.PHONY: integration-test-ini
integration-test-ini:
		@(cd test; tilt -f Tiltfile.integration-ini down 2>/dev/null || true; rc=0; tilt -f Tiltfile.integration-ini ci || rc=$$?; tilt -f Tiltfile.integration-ini down; exit $$rc)
