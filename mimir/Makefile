.PHONY: test unit-test integration-test

test: unit-test

unit-test:
	@(cd test; tilt ci)

integration-test:
	@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)
