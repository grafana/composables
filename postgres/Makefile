.PHONY: test
test: integration-test

.PHONY: integration
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)
		@(cd test; tilt -f Tiltfile.integration-require-database ci; tilt -f Tiltfile.integration-require-database down)
