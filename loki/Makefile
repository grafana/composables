.PHONY: test
test: unit-test integration-test

.PHONY: unit-test
unit-test:
		@(cd test; tilt ci)

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)
