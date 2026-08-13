BINARY := bin/kg-write-twin
IMAGE  := kg-write-twin:dev
UPSTREAM := repos/grafana/asserts-adi/contents/inference-engine/api-server/openapi/kg-write.yaml

# `test` is the composable contract used by the top-level Makefile. Unlike other
# composables, this one is a behavioral twin, so `test` gates API fidelity
# (Go unit + hermetic conformance tests) in addition to the compose_composer
# integration test. Requires a Go toolchain + docker + tilt (CI installs Go).
.PHONY: test
test: unit-test integration-test

.PHONY: unit-test
unit-test:
	go test ./...

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; status=$$?; tilt -f Tiltfile.integration down; exit $$status)

# ----------------------------------------------------------------------------
# Go dev targets (require a local Go toolchain)
# ----------------------------------------------------------------------------
.PHONY: build run docker record check-spec-drift tidy

build:
	go build -o $(BINARY) ./cmd/kg-write-twin

run:
	go run ./cmd/kg-write-twin

docker:
	docker build -t $(IMAGE) .

# Re-record goldens from a live instance: make record KG_TWIN_REAL_URL=http://localhost:8030
record:
	@test -n "$(KG_TWIN_REAL_URL)" || (echo "set KG_TWIN_REAL_URL"; exit 1)
	KG_TWIN_REAL_URL=$(KG_TWIN_REAL_URL) go run ./cmd/kg-twin-record

# Fail if the vendored OpenAPI spec drifts from upstream.
check-spec-drift:
	@gh api $(UPSTREAM) --jq '.content' | base64 -d > /tmp/kg-write-upstream.yaml
	@diff -u openapi/kg-write.yaml /tmp/kg-write-upstream.yaml && echo "openapi in sync" \
		|| (echo "!! openapi drift: upstream changed — re-probe the twin and refresh goldens"; exit 1)

tidy:
	go mod tidy
