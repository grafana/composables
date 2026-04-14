.PHONY: test
test: integration-test register-flags-test

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)

.PHONY: register-flags-test
register-flags-test:
		@(cd test; tilt -f Tiltfile.integration-register-flags ci; tilt -f Tiltfile.integration-register-flags down)
