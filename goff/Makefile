.PHONY: test
test: integration-test register-flags-test

.PHONY: integration-test
integration-test:
		@(cd test; rc=0; tilt -f Tiltfile.integration ci || rc=$$?; tilt -f Tiltfile.integration down; exit $$rc)

.PHONY: register-flags-test
register-flags-test:
		@(cd test; rc=0; tilt -f Tiltfile.integration-register-flags ci || rc=$$?; tilt -f Tiltfile.integration-register-flags down; exit $$rc)
