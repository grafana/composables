.PHONY: test
test: integration-test
	@(cd test; tilt ci)

.PHONY: integration
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)
