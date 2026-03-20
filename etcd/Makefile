.PHONY: test
test: integration-test

.PHONY: integration
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)
