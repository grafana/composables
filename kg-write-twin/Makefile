BINARY := bin/kg-write-twin
IMAGE  := kg-write-twin:dev
UPSTREAM := repos/grafana/asserts-adi/contents/inference-engine/api-server/openapi/kg-write.yaml

# `test` is the composable contract used by the top-level Makefile: it runs the
# compose_composer integration test (builds the image from source and brings the
# service up through the pipeline). Requires docker + tilt, not a host Go toolchain.
.PHONY: test
test: integration-test

.PHONY: integration-test
integration-test:
		@(cd test; tilt -f Tiltfile.integration ci; tilt -f Tiltfile.integration down)

# ----------------------------------------------------------------------------
# Go dev targets (require a local Go toolchain; not run by the composable suite)
# ----------------------------------------------------------------------------
.PHONY: build go-test run docker record check-spec-drift tidy

build:
	go build -o $(BINARY) ./cmd/kg-write-twin

go-test:
	go test ./...

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
