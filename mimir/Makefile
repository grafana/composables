.PHONY: test unit-test integration-test

test: unit-test

unit-test:
	@(cd test; tilt ci)

integration-test:
	@(cd test; rc=0; tilt -f Tiltfile.integration ci || rc=$$?; tilt -f Tiltfile.integration down; exit $$rc)
