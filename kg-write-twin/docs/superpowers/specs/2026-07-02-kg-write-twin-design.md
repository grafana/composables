# Digital Twin: Knowledge Graph Write API — Design

**Date:** 2026-07-02
**Status:** Approved design (pending spec review)
**Author:** brainstorming session

## Purpose

A standalone, in-memory **behavioral clone** (a "Digital Twin", per the
[StrongDM DTU technique](https://factory.strongdm.ai/techniques/dtu)) of the Asserts
**Knowledge Graph Write API** defined in
`grafana/asserts-adi:inference-engine/api-server/openapi/kg-write.yaml`.

It exists to make local development and integration testing of Gamma's graph-write path
fast, deterministic, and free of the real Java `api-server` (which pulls in Loki, GCom, and
other external dependencies). The twin replicates the API's **observable boundary behavior**
— status codes, response shapes, validation errors, TTL metadata, content negotiation — not
the server's internal logic.

Fidelity target (chosen during brainstorming): **full behavioral fidelity**, validated
against a live dev instance (see "Captured Behavior" below).

## Non-Goals

- No graph query/read API (the real API-server has separate read endpoints; out of scope).
- No persistence — state is in-memory and resets on restart.
- No real tracing/metrics backends (Server-Timing/traceparent headers are cosmetic).
- No authn/authz beyond the tenant header the real API uses (`X-Scope-OrgID`).

## Packaging & Layout

Standalone Go module so it can be extracted and **published as its own `cc` composable**
(consumed via `cc.use('kg-write-twin')`, exactly like `k3s-apiserver`, `mimir`, `grafana`).

- **Module:** `github.com/grafana/kg-write-twin` (own `go.mod`, zero dependency on the gamma module).
- **Directory:** `kg-write-twin/` at repo root.
- **Deps:** stdlib only, plus `gopkg.in/yaml.v3` for content negotiation.

```
kg-write-twin/
  go.mod / go.sum
  cmd/kg-write-twin/main.go     # flags/env -> build store + server -> ListenAndServe
  internal/model/               # request/response/error DTOs mirroring the OpenAPI schemas
  internal/store/               # in-memory tenant-scoped store; entity/relationship maps; origin + expiry metadata
  internal/api/
    router.go                   # ServeMux wiring, base path, security-header middleware
    entities.go                 # upsert + delete entity handlers
    relationships.go            # upsert + delete relationship handlers
    tenant.go                   # X-Scope-OrgID + namespace resolution (424/400/403/500)
    codec.go                    # content negotiation (json / x-yaml / x-yml), request decode + response encode
    errors.go                   # ApiError construction, requestId formatting, status -> body
    control.go                  # /_control test-only endpoints
  internal/conformance/
    scenarios.go                # single declarative catalog of request scenarios (source of truth)
    conformance_test.go         # differential test: twin vs golden (and vs live when configured)
    normalize.go                # strips/canonicalizes volatile fields before comparison
  cmd/kg-twin-record/main.go    # record tool: replay scenarios against a live instance -> goldens
  testdata/golden/              # normalized recorded responses (committed)
  openapi/kg-write.yaml         # vendored copy of the source spec (reference + drift check)
  Dockerfile                    # multi-stage static build
  Tiltfile                      # cc composable: cc_export / cc_setup
  compose.yaml                  # service definition (port 8030)
  Makefile                      # build / test / docker / record / check-spec-drift
  README.md
```

### Component boundaries

- **`store`** — pure data layer. Knows nothing about HTTP. Keyed by tenant. Exposes
  `UpsertEntity`, `DeleteEntity`, `UpsertRelationship`, `DeleteRelationship`, plus control ops
  (`Reset`, `Query`, `Seed`). Returns typed results (created vs updated, not-found, origin-conflict)
  that handlers map to status codes. Control ops: `Reset(tenant?)`, `Query(filter)` (the
  filterable extraction backing `GET /_control/state`), and `Seed`. Independently unit-testable.
- **`api`** — HTTP layer. Routing, tenant/namespace resolution, validation, content negotiation,
  error mapping. Depends on `store` and `model`.
- **`model`** — plain DTOs + validation rules, shared by both.

This keeps each unit understandable in isolation: the store can be tested without a server, and
handlers can be tested with `httptest` against an in-memory store.

## Routing

Go 1.22+ stdlib `net/http.ServeMux` method+wildcard patterns (no web framework). Base path is
configurable, default `/api-server` (matches the spec's server URL). Routes:

| Method | Pattern | Handler |
|--------|---------|---------|
| POST   | `{base}/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities` | upsert entity |
| DELETE | `{base}/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities/{type}/{name}` | delete entity |
| POST   | `{base}/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships` | upsert relationship |
| DELETE | `{base}/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships/{type}` | delete relationship |
| POST   | `{base}/_control/reset` | reset state |
| GET    | `{base}/_control/state` | extract/read-back stored graph (filterable) |
| POST   | `{base}/_control/seed` | seed entities/relationships (with explicit origin/expiry) |
| GET    | `{base}/_control/healthz` | liveness |

Unknown path → 404 `"No static resource <path>."`; known path + wrong method → 405
`"Request method '<M>' is not supported"`.

## Data Model

- **Tenant** = value of `X-Scope-OrgID`. Store is partitioned per tenant.
- **Entity identity** = `(domain, type, name, canonical(scope))`. `scope` is a
  `map[string]string`; canonical key sorts entries so key order is irrelevant. Scope values
  coerce to strings (the real API accepts a JSON number and stores `"5"`).
- **Relationship identity** = `(domain, type, fromRef, toRef)` where each ref is an entity identity.
- **origin** — every object stores an origin string; spec writes always set `origin="api"`.
  Non-api origins exist only via `/_control/seed` (used to exercise 409/403-not-api-origin).
- **Expiry metadata** — from `ttlSeconds` at write time:
  - `ttl > 0` → `_expires_at = now_ms + ttl*1000`
  - `ttl < 0` (any negative, incl. `-1`) → `_expires_at = 9223372036854775807` (Int64 max)
  - `ttl == 0` → `_expired = now_ms` (and **no** `_expires_at`)
  - **Expiry is metadata only.** The write path does **not** evict or hide expired objects —
    a relationship to an expired entity still succeeds, and deleting an expired entity still
    returns 204. (Confirmed against the live server; matches the design.)

### Response `properties` enrichment

Response `properties` = user properties **merged with computed keys**, in this order:
`_domain` (= request domain), then the user's properties (in input order), then `_origin`,
then `_expires_at` **or** `_expired`. Reproducing key order requires an **ordered properties
map** (custom JSON/YAML marshaling), since Go maps are unordered — this is best-effort byte
fidelity and is worth doing because integration tests may snapshot bodies.

## Endpoint Semantics (captured from the live dev instance)

All 4xx/5xx bodies use the `ApiError` schema:
`{status, requestId, timestamp, message, subErrors?}`. Observed responses did **not** include
`debugMessage`, `trace_id`, or `span_id`; `subErrors` is present for validation (422) and for
the not-found "static resource" 404, and **absent** for parse (400), tenant (424), namespace
(400/403), 415, 405, and 500.

`requestId` is a random 64-bit value formatted as **space-padded 16-char hex**
(`fmt.Sprintf("%16x", rnd)`), reproducing the observed leading-space quirk. `timestamp` is
epoch milliseconds.

### Tenant / namespace resolution (all endpoints, checked in this order)

1. Missing `X-Scope-OrgID` → **424 FAILED_DEPENDENCY** `"No tenant selected for request"`.
2. Namespace not matching `^stacks-.+$` → **400 BAD_REQUEST** `"namespace must be of the form 'stacks-<stackId>'"`.
3. Namespace stackId ≠ tenant → **403 FORBIDDEN** `"namespace does not match the request tenant"`.
4. Tenant not parseable as an integer → **500 INTERNAL_SERVER_ERROR** `"Failed initializing tenantId=<v>, cannot continue"` (a real quirk, reproduced for fidelity).

### POST entities — upsert

- **201** on create, **200** on update (same identity re-written; recreate after delete → 201).
- Response `EntityWriteResponseDto`: `{domain, type, name, scope?, properties}`. `scope` omitted
  when empty. `properties` enriched as above.
- **409 CONFLICT** if a matching entity exists with `origin != "api"` (message per OpenAPI:
  *"Target already exists with a non-API origin"* — unreachable via this API alone; only via seed).
- **422** validation (see Validation Rules).

### DELETE entities/{type}/{name}?domain=&scope[k]=v

- **204** (no body) on success; **404** `"entity not found"` when absent.
- `domain` query param required (blank → 422 `"must not be blank"`; `kg`/bad slug → 422 pattern msg).
- Optional `scope` as deepObject query (`scope[env]=dev`) must match stored scope exactly.
- Invalid `{type}` path var → **422** `message:"Invalid request"`, `field:"deleteEntity.type"`.
- **403** if target exists but `origin != "api"` (per OpenAPI: *"target is not API-origin"* — via seed only).

### POST relationships — upsert

- **Always 200** (no 201, even on first create).
- `from` and `to` entities must already exist (expiry ignored). `from` is checked first:
  missing `from` → **404** `"from entity not found"`; else missing `to` → **404** `"to entity not found"`.
- Response `RelationshipWriteResponseDto`: `{domain, type, from, to, properties}`. `from`/`to`
  **always** include `scope` (defaults to `{}`). `properties` enriched as above.
- **409 CONFLICT** if a matching edge exists with `origin != "api"` (per OpenAPI: *"Edge already
  exists with a non-API origin"* — via seed only). **422** validation.

### DELETE relationships/{type}?from.*&to.*

- **204** on success; **404** `"relationship not found"` when absent.
- Required query params: `from.domain`, `from.type`, `from.name`, `to.domain`, `to.type`,
  `to.name`; optional `from.scope[k]`, `to.scope[k]` deepObject maps.
- Invalid `{type}` path var → **422** `message:"Invalid request"`, `field:"deleteRelationship.type"`.
- **403** if edge exists but `origin != "api"` (per OpenAPI: *"edge is not API-origin"* — via seed only).

### Validation Rules (422 UNPROCESSABLE_ENTITY)

Body-validation message: `"Invalid request: ServletWebRequest: uri=<full-uri>;client=127.0.0.1"`.
`subErrors[]` items: `{"@type":"ApiValidationError", "field", "rejectedValue"?, "message"}`
(`rejectedValue` omitted when the value is null/missing).

| Field | Rule | Message |
|-------|------|---------|
| `domain` | `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$` and `!= "kg"` | `domain must be a lowercase k8s-style slug and not the reserved 'kg'` |
| `type` | `^[A-Za-z][A-Za-z0-9_]*$` | `type must be a valid identifier` |
| `name` | not blank | `must not be blank` |
| `ttlSeconds` | not null | `must not be null` |
| `from` / `to` (relationship) | not null | `must not be null` |
| any required string missing/blank | not blank | `must not be blank` |

Multiple violations are all returned. Spring's ordering is reflection-dependent; the twin will
emit a **deterministic** order (field declaration order) — accepted as best-effort, since exact
ordering is not behaviorally significant.

Parse errors: malformed JSON → **400 BAD_REQUEST** `"JSON parse error: <detail>"`, **no**
`subErrors` key.

### Content negotiation

- Request `Content-Type`: `application/json` (default), `application/x-yaml`, `application/x-yml`.
  Unknown type → **415 UNSUPPORTED_MEDIA_TYPE** `"Content-Type '<ct>' is not supported"`.
- Response format from `Accept`: `application/x-yaml`/`x-yml` → YAML body (Content-Type
  `application/x-yaml`); otherwise JSON. Applies to success **and** error bodies. YAML rendering
  of `subErrors` type tags is best-effort (`yaml.v3` won't emit Java's `!<ApiValidationError>` tag).

### Response headers

Reproduce the static Spring security headers for realism (`X-Content-Type-Options: nosniff`,
`X-XSS-Protection: 0`, `X-Frame-Options: DENY`, `Cache-Control: no-cache, no-store, max-age=0,
must-revalidate`, `Pragma: no-cache`, `Expires: 0`) via middleware. Tracing headers
(`Server-Timing`, traceparent) are omitted.

## Control (test-only) endpoints — `/_control`

Clearly separated from the spec surface, for deterministic integration tests:

- **`POST /_control/reset`** — clear all state, or a single tenant via `?tenant=<id>`.
- **`GET /_control/state`** — **extract/read back the stored graph as JSON** (the read-back the
  write API otherwise lacks), for integration-test assertions. Returns a
  `{entities: [...], relationships: [...]}` object where each item includes its full stored form
  **plus internal metadata** (`origin`, `_expires_at`/`_expired`, `createdAt`). Filterable via
  query params, all optional and AND-combined:
  - `tenant=<id>` — restrict to one tenant (default: all tenants).
  - `kind=entity|relationship` — return only one collection.
  - `domain=<slug>` — match `domain`.
  - `type=<ident>` — match entity/relationship `type`.
  - `name=<str>` — match entity `name` (ignored for relationships).
  - `origin=<str>` — match `origin` (e.g. `api` vs a seeded non-api origin).
  - `includeExpired=true|false` — default `true` (expired objects are returned, matching the
    write path's metadata-only expiry); set `false` to exclude objects whose expiry has passed.

  JSON only (no content negotiation on the control surface) to keep test assertions simple.
- **`POST /_control/seed`** — bulk-insert entities/relationships with **explicit `origin`** and
  optional explicit expiry. Primary use: create non-api-origin objects to exercise 409 (upsert)
  and 403-not-api-origin (delete), and to preload fixtures.
- **`GET /_control/healthz`** — liveness (200).

## Configuration

Flags (with env fallbacks):

- `--addr` / `PORT` — listen address, default `:8030`.
- `--base-path` — default `/api-server`.
- `--seed-file` — optional JSON file preloaded via the seed path at boot.

## Testing Strategy

TDD, standard-library assertions only (house style — no testify).

- **`store` unit tests** — scope canonicalization, create-vs-update result, expiry metadata
  computation (`_expires_at`/`_expired`/Int64-max), origin-conflict detection, seed/reset, and
  `Query` filtering (each filter field + `includeExpired`).
- **`api` handler tests** (`httptest`) — one table-driven suite per endpoint asserting status
  code + body for every captured case: 200/201/204, 400, 403, 404, 409 (via seed), 415, 422,
  424, 500, plus content-negotiation (JSON/YAML request+response) and path-var validation.
- **Golden behavior** — the "Captured Behavior" cases in this doc are the source of truth; each
  becomes a test case. These cases live as a single scenario catalog reused by the conformance
  harness (see "Continuous Validation").

## Continuous Validation Against the Real API (drift detection)

The twin must stay behaviorally faithful as the real `api-server` evolves. Per the DTU technique,
validation against the live service is an **ongoing** provision, not a one-time capture. Four
mechanisms, sharing one scenario catalog:

### 1. Single scenario catalog

`internal/conformance/scenarios.go` holds one declarative list of request scenarios — each with a
name, optional seed/setup steps, method, path, headers, content-type, and body. This is the single
source of truth consumed by (a) the twin's own handler tests, (b) the differential conformance
test, and (c) the record tool. Adding a newly discovered behavior means adding one scenario.

### 2. Differential conformance test (twin vs golden, and vs live)

`conformance_test.go` replays every scenario against an **in-process twin** and asserts the
response (status + normalized body + relevant headers) matches the committed golden. When
`KG_TWIN_REAL_URL` is set, it *also* replays against the live instance and diffs live-vs-twin,
failing on any behavioral difference. Without that env var the test is fully hermetic (golden
comparison only), so default `make test` and CI need no live server.

### 3. Normalization

`normalize.go` canonicalizes volatile fields before comparison, so only meaningful differences
surface: `requestId` (random), `timestamp` (wall clock), absolute `_expires_at`/`_expired` epoch
values (compare kind/relative, not exact), tracing headers (`Server-Timing`), and the
`uri=...;client=...` host portion of validation messages. The function is documented so it is
explicit what is ignored and why.

### 4. Record mode + goldens

`cmd/kg-twin-record` (via `make record KG_TWIN_REAL_URL=...`) replays the scenario catalog against
a live instance and writes normalized responses to `testdata/golden/`. This makes re-validation
after an intended upstream change a single command; the golden diff in code review shows exactly
what changed in the real API.

### 5. OpenAPI upstream drift check

`make check-spec-drift` fetches the upstream `kg-write.yaml`
(`gh api repos/grafana/asserts-adi/contents/inference-engine/api-server/openapi/kg-write.yaml`)
and diffs it against the vendored `openapi/kg-write.yaml`, exiting non-zero (with a diff) when they
differ — a cheap CI/periodic signal that the contract changed and the twin should be re-probed.
Stretch: validate twin JSON responses against the vendored OpenAPI schemas.

### Divergence log

The observed-vs-spec differences captured in this design (424 tenant, always-200 relationships,
`properties` enrichment, TTL-as-metadata, etc.) are recorded in the twin's README as a
**divergence log**, so future maintainers can tell intentional fidelity choices from real-API
changes that the drift checks surface.

### CI wiring

Default CI: hermetic tests + golden comparison + `check-spec-drift`. Optional scheduled/nightly
job (or the `cc` integration env) runs the differential suite with `KG_TWIN_REAL_URL` pointed at a
real `api-server` when one is available.

## cc Composable Packaging

- **`Dockerfile`** — multi-stage: build static binary, copy into a minimal/distroless image,
  expose the port, `ENTRYPOINT` the binary.
- **`compose.yaml`** — a single `kg-write-twin` service on port 8030.
- **`Tiltfile`** — exposes `cc_export(cc)` returning `cc.create('kg-write-twin', compose.yaml, ...)`
  and a `cc_setup` that builds the binary/image with file-watching for hot reload. Once published
  (to `grafana/composeables` or its own repo), gamma's `cc/Tiltfile` adds `cc.use('kg-write-twin')`.

## Open Questions / Best-Effort Fidelity Notes

- 409 / 403-not-api-origin **messages** are taken from the OpenAPI descriptions, not observed
  (unreachable via the write API); may differ slightly from the real server's wording.
- Multi-error `subErrors` **ordering** is deterministic in the twin but may not match Spring's.
- YAML `subErrors` **type tag** (`!<ApiValidationError>`) is not reproduced by `yaml.v3`.
- Property **key ordering** is reproduced best-effort via an ordered map.
