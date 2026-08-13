# kg-write-twin

An in-memory behavioral clone ("digital twin") of the Asserts **Knowledge Graph Write API**
(`grafana/asserts-adi:inference-engine/api-server/openapi/kg-write.yaml`), for local
development and integration testing. Behavior was captured by probing a live dev instance.

Design: `docs/superpowers/specs/2026-07-02-kg-write-twin-design.md`.

## Run

    make run                      # :8030, base path /api-server
    make build && ./bin/kg-write-twin --addr :8030 --base-path /api-server
    docker build -t kg-write-twin:dev . && docker run -p 8030:8030 kg-write-twin:dev

Flags/env: `--addr`/`PORT` (default `:8030`), `--base-path`/`BASE_PATH` (default `/api-server`),
`--seed-file`/`SEED_FILE` (optional JSON fixtures keyed by tenant).

## As a cc composable

This directory is a self-contained [compose_composer](https://github.com/grafana/tilt-extensions/tree/main/compose_composer)
composable (`Tiltfile` + `compose.yaml`). An orchestrator in another tree can embed it by
passing its path as a CLI plugin:

    tilt up -f <orchestrator>/Tiltfile -- /path/to/kg-write-twin

The published/listen port is `${KG_WRITE_TWIN_PORT:-8030}`, so an orchestrator can place it on
a non-conflicting port. Gamma wires it in via `make cc/up` on port `8031` (override with
`KG_WRITE_TWIN_DIR` / `KG_WRITE_TWIN_PORT`).

## Endpoints

Spec endpoints (base path prefixed):

- `POST   /apis/kg.grafana.com/v1alpha1/namespaces/{ns}/entities`
- `DELETE /apis/kg.grafana.com/v1alpha1/namespaces/{ns}/entities/{type}/{name}?domain=&scope[k]=v`
- `POST   /apis/kg.grafana.com/v1alpha1/namespaces/{ns}/relationships`
- `DELETE /apis/kg.grafana.com/v1alpha1/namespaces/{ns}/relationships/{type}?from.*&to.*`

Test-only control surface (JSON):

- `POST /_control/reset?tenant=` — clear state (all tenants if `tenant` omitted)
- `GET  /_control/state?tenant=&kind=&domain=&type=&name=&origin=&includeExpired=` — extract stored graph
- `POST /_control/seed?tenant=` — bulk insert with explicit `origin` (create non-api objects for 409/403)
- `GET  /_control/healthz`

## Continuous validation

    make record KG_TWIN_REAL_URL=http://localhost:8030   # refresh goldens from a live server
    go test ./internal/conformance/...                   # hermetic twin-vs-golden
    KG_TWIN_REAL_URL=http://localhost:8030 go test ./internal/conformance/...  # + live differential
    make check-spec-drift                                # alert if upstream OpenAPI changed

## Divergence log (real API behavior vs OpenAPI spec)

Intentional fidelity choices matching the **real** server, where it differs from `kg-write.yaml`:

| Behavior | OpenAPI says | Real server (twin matches) |
|----------|--------------|----------------------------|
| Missing `X-Scope-OrgID` | 400 | **424 FAILED_DEPENDENCY** "No tenant selected for request" |
| Non-numeric tenant | (unspecified) | **500 INTERNAL_SERVER_ERROR** "Failed initializing tenantId=…" |
| Relationship upsert | 200 | **always 200** (never 201, even on create) |
| Response `properties` | echoes input | enriched with `_domain`, `_origin`, `_expires_at`/`_expired` |
| `ttlSeconds` negative | (int) | any negative → `_expires_at = Int64 max` (never expire) |
| `ttlSeconds == 0` | (int) | sets `_expired` (immediately expired), no `_expires_at` |
| TTL enforcement | (implied) | **metadata only** — expired objects remain writable/deletable |
| Relationship refs | scope optional | response always includes `scope` (`{}` when empty) |
| Scope/property values | strings | scalars coerced to strings (`5` → `"5"`) |
| Unknown Content-Type | (unspecified) | 415 UNSUPPORTED_MEDIA_TYPE |
| Wrong method / unknown path | (unspecified) | 405 / 404 "No static resource <path>." |

Best-effort (not byte-identical to the real server):
- 409/403 non-api-origin **messages** are from the OpenAPI descriptions (unreachable via the write API; only via `/_control/seed`).
- Multi-error `subErrors` ordering is deterministic in the twin; the conformance harness sorts by `field` before comparing.
- YAML error bodies do not reproduce Java's `!<ApiValidationError>` type tag.
