.PHONY: test
test: unit-test integration-test

.PHONY: unit-test
unit-test:
		@(cd test; tilt ci)

.PHONY: integration-test
integration-test:
		@(cd test; rc=0; tilt -f Tiltfile.integration ci || rc=$$?; tilt -f Tiltfile.integration down; exit $$rc)
