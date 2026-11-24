.PHONY: test
test: integration-test

.PHONY: integration
integration-test:
		(cd test; rc=0; tilt -f Tiltfile.integration ci || rc=$$?; tilt -f Tiltfile.integration down; exit $$rc)
