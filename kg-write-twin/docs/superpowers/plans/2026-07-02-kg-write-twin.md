# KG Write API Digital Twin — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone, in-memory Go service that behaviorally clones the Asserts Knowledge Graph Write API (4 endpoints), for local dev and integration testing, packaged as a `cc` composable, with continuous drift-validation against the real API.

**Architecture:** Three layers — `internal/model` (DTOs + validation), `internal/store` (in-memory, tenant-scoped state with origin + TTL metadata), `internal/api` (stdlib `net/http` handlers, tenant/namespace resolution, content negotiation, error mapping, `/_control` test surface). A conformance harness (`internal/conformance` + `cmd/kg-twin-record`) replays a shared scenario catalog against committed goldens (hermetic) and, when `KG_TWIN_REAL_URL` is set, against a live instance.

**Tech Stack:** Go 1.25 (stdlib `net/http.ServeMux` method+wildcard routing), `gopkg.in/yaml.v3` for content negotiation. Standard-library test assertions only (no testify). `gh` CLI for OpenAPI drift check.

**Reference:** Design spec at `docs/superpowers/specs/2026-07-02-kg-write-twin-design.md` — the "Endpoint Semantics (captured from the live dev instance)" section is the behavioral source of truth. All work happens under the new `kg-write-twin/` directory (its own Go module).

---

## File Structure

```
kg-write-twin/
  go.mod                              # module github.com/grafana/kg-write-twin
  go.sum
  .gitignore
  README.md                          # usage + divergence log
  Makefile                           # build/test/run/docker/record/check-spec-drift
  openapi/kg-write.yaml              # vendored copy of the upstream spec
  cmd/kg-write-twin/main.go          # server entrypoint (flags/env -> store + server)
  cmd/kg-twin-record/main.go         # record scenarios against a live instance -> goldens
  internal/model/
    strmap.go                        # StringMap: JSON decode with scalar->string coercion
    strmap_test.go
    dto.go                           # request/response DTOs + FieldError
    validate.go                      # domain/type/name/ttl validation rules
    validate_test.go
  internal/store/
    store.go                         # Store, tenant partitioning, keys, expiry helper
    entity.go                        # UpsertEntity / DeleteEntity
    relationship.go                  # UpsertRelationship / DeleteRelationship
    control.go                       # Reset / Seed / Query (filter)
    store_test.go
    entity_test.go
    relationship_test.go
    control_test.go
  internal/api/
    server.go                        # router (ServeMux), middleware, 404/405 handlers
    tenant.go                        # X-Scope-OrgID + namespace resolution
    codec.go                         # content negotiation: decode/encode json+yaml
    errors.go                        # apiError, requestId, error body
    respond.go                       # spec response bodies + properties enrichment
    entities.go                      # upsert/delete entity handlers
    relationships.go                 # upsert/delete relationship handlers
    control.go                       # /_control handlers
    *_test.go                        # httptest suites (one per handler file)
  internal/conformance/
    scenarios.go                     # single declarative scenario catalog
    normalize.go                     # strip/canonicalize volatile fields
    normalize_test.go
    conformance_test.go              # twin-vs-golden (+ twin-vs-live when configured)
    testdata/golden/                 # committed normalized responses
  Dockerfile
  compose.yaml
  Tiltfile
```

---

## Task 1: Module scaffold

**Files:**
- Create: `kg-write-twin/go.mod`
- Create: `kg-write-twin/.gitignore`
- Create: `kg-write-twin/openapi/kg-write.yaml` (vendored copy)
- Create: `kg-write-twin/README.md` (skeleton)

- [ ] **Step 1: Create the module directory and go.mod**

Run:
```bash
cd kg-write-twin && go mod init github.com/grafana/kg-write-twin
```
Then edit `kg-write-twin/go.mod` to pin the Go version and add the yaml dep:
```
module github.com/grafana/kg-write-twin

go 1.25

require gopkg.in/yaml.v3 v3.0.1
```

- [ ] **Step 2: Add the yaml dependency**

Run:
```bash
cd kg-write-twin && go get gopkg.in/yaml.v3@v3.0.1 && go mod tidy
```
Expected: `go.sum` created, `gopkg.in/yaml.v3` present.

- [ ] **Step 3: Vendor the upstream OpenAPI spec**

Run (from repo root):
```bash
gh api repos/grafana/asserts-adi/contents/inference-engine/api-server/openapi/kg-write.yaml --jq '.content' | base64 -d > kg-write-twin/openapi/kg-write.yaml
head -3 kg-write-twin/openapi/kg-write.yaml
```
Expected: file begins with `openapi: 3.1.0`.

- [ ] **Step 4: Create .gitignore**

`kg-write-twin/.gitignore`:
```
/bin/
*.out
```

- [ ] **Step 5: Create README skeleton**

`kg-write-twin/README.md`:
```markdown
# kg-write-twin

An in-memory behavioral clone ("digital twin") of the Asserts Knowledge Graph Write API,
for local development and integration testing. See
`../docs/superpowers/specs/2026-07-02-kg-write-twin-design.md` for the design.

## Quick start

    make run          # starts on :8030 with base path /api-server

## Divergence log

(populated in the final task)
```

- [ ] **Step 6: Commit**

```bash
git add kg-write-twin/go.mod kg-write-twin/go.sum kg-write-twin/.gitignore kg-write-twin/openapi/kg-write.yaml kg-write-twin/README.md
git commit -m "feat(kg-write-twin): scaffold standalone module + vendored openapi"
```

---

## Task 2: StringMap (scalar-coercing map)

The real API stores scope/properties as `Map<String,String>` and coerces JSON scalars to strings (a JSON number `5` becomes `"5"`). This type reproduces that on decode.

**Files:**
- Create: `kg-write-twin/internal/model/strmap.go`
- Test: `kg-write-twin/internal/model/strmap_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/model/strmap_test.go`:
```go
package model

import (
	"encoding/json"
	"testing"
)

func TestStringMapCoercesScalars(t *testing.T) {
	var m StringMap
	if err := json.Unmarshal([]byte(`{"a":"x","n":5,"b":true}`), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]string{"a": "x", "n": "5", "b": "true"}
	if len(m) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(m), len(want), m)
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("m[%q]=%q want %q", k, m[k], v)
		}
	}
}

func TestStringMapNull(t *testing.T) {
	var m StringMap
	if err := json.Unmarshal([]byte(`null`), &m); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if m != nil {
		t.Errorf("want nil map, got %v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/model/ -run TestStringMap -v`
Expected: FAIL (StringMap undefined / build error).

- [ ] **Step 3: Write the implementation**

`kg-write-twin/internal/model/strmap.go`:
```go
package model

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// StringMap is a map[string]string that, when decoded from JSON, coerces scalar
// values (numbers, booleans) to their string form — matching the real API's
// Jackson Map<String,String> binding.
type StringMap map[string]string

func (m *StringMap) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*m = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(StringMap, len(raw))
	for k, v := range raw {
		out[k] = coerceScalar(v)
	}
	*m = out
	return nil
}

func coerceScalar(raw json.RawMessage) string {
	s := string(bytes.TrimSpace(raw))
	if len(s) == 0 {
		return ""
	}
	if s[0] == '"' {
		if unq, err := strconv.Unquote(s); err == nil {
			return unq
		}
	}
	return s // numbers, booleans, null -> literal text
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd kg-write-twin && go test ./internal/model/ -run TestStringMap -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/model/strmap.go kg-write-twin/internal/model/strmap_test.go
git commit -m "feat(kg-write-twin): StringMap with scalar->string coercion"
```

---

## Task 3: DTOs and validation

**Files:**
- Create: `kg-write-twin/internal/model/dto.go`
- Create: `kg-write-twin/internal/model/validate.go`
- Test: `kg-write-twin/internal/model/validate_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/model/validate_test.go`:
```go
package model

import (
	"encoding/json"
	"testing"
)

func ptr(i int64) *int64 { return &i }

func TestValidateEntity(t *testing.T) {
	cases := []struct {
		name string
		req  EntityWriteRequest
		want []FieldError // field+message only checked
	}{
		{"ok", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "a", TTLSeconds: ptr(10)}, nil},
		{"domain kg", EntityWriteRequest{Domain: "kg", Type: "Team", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "domain", Message: domainMsg}}},
		{"domain upper", EntityWriteRequest{Domain: "Bad", Type: "Team", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "domain", Message: domainMsg}}},
		{"type bad", EntityWriteRequest{Domain: "irm", Type: "1bad", Name: "a", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "type", Message: typeMsg}}},
		{"name blank", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "", TTLSeconds: ptr(10)},
			[]FieldError{{Field: "name", Message: blankMsg}}},
		{"ttl nil", EntityWriteRequest{Domain: "irm", Type: "Team", Name: "a"},
			[]FieldError{{Field: "ttlSeconds", Message: nullMsg}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidateEntity(c.req)
			if len(got) != len(c.want) {
				t.Fatalf("got %d errors %v, want %d", len(got), got, len(c.want))
			}
			for i := range c.want {
				if got[i].Field != c.want[i].Field || got[i].Message != c.want[i].Message {
					t.Errorf("err[%d]=%+v want field=%s msg=%s", i, got[i], c.want[i].Field, c.want[i].Message)
				}
			}
		})
	}
}

func TestFieldErrorMarshal(t *testing.T) {
	withVal, _ := json.Marshal(FieldError{Field: "domain", RejectedValue: "kg", HasRejected: true, Message: domainMsg})
	if got := string(withVal); got != `{"@type":"ApiValidationError","field":"domain","rejectedValue":"kg","message":"`+domainMsg+`"}` {
		t.Errorf("with value: %s", got)
	}
	noVal, _ := json.Marshal(FieldError{Field: "name", Message: blankMsg})
	if got := string(noVal); got != `{"@type":"ApiValidationError","field":"name","message":"`+blankMsg+`"}` {
		t.Errorf("no value: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/model/ -run 'TestValidate|TestFieldError' -v`
Expected: FAIL (undefined types).

- [ ] **Step 3: Write dto.go**

`kg-write-twin/internal/model/dto.go`:
```go
package model

import "encoding/json"

// EntityRef identifies an entity endpoint of a relationship.
type EntityRef struct {
	Domain string    `json:"domain"`
	Type   string    `json:"type"`
	Name   string    `json:"name"`
	Scope  StringMap `json:"scope"`
}

// EntityWriteRequest is the POST /entities body.
type EntityWriteRequest struct {
	Domain     string    `json:"domain"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Scope      StringMap `json:"scope"`
	Properties StringMap `json:"properties"`
	TTLSeconds *int64    `json:"ttlSeconds"`
}

// RelationshipWriteRequest is the POST /relationships body.
type RelationshipWriteRequest struct {
	Domain     string     `json:"domain"`
	Type       string     `json:"type"`
	From       *EntityRef `json:"from"`
	To         *EntityRef `json:"to"`
	Properties StringMap  `json:"properties"`
	TTLSeconds *int64     `json:"ttlSeconds"`
}

// FieldError is one ApiValidationError sub-error.
type FieldError struct {
	Field         string
	RejectedValue any
	HasRejected   bool
	Message       string
}

func (e FieldError) MarshalJSON() ([]byte, error) {
	// A struct preserves key order (@type, field, rejectedValue, message) to
	// match the real API; encoding/json would sort a map's keys.
	type ordered struct {
		Type          string `json:"@type"`
		Field         string `json:"field"`
		RejectedValue any    `json:"rejectedValue,omitempty"`
		Message       string `json:"message"`
	}
	o := ordered{Type: "ApiValidationError", Field: e.Field, Message: e.Message}
	if e.HasRejected {
		o.RejectedValue = e.RejectedValue
	}
	return json.Marshal(o)
}
```

> Note: `rejectedValue,omitempty` drops it when nil. When `HasRejected` is true the value is non-nil, so it is emitted; when false it is omitted. This matches the observed behavior (omitted for missing fields).

- [ ] **Step 4: Write validate.go**

`kg-write-twin/internal/model/validate.go`:
```go
package model

import "regexp"

const (
	domainMsg = "domain must be a lowercase k8s-style slug and not the reserved 'kg'"
	typeMsg   = "type must be a valid identifier"
	blankMsg  = "must not be blank"
	nullMsg   = "must not be null"
)

var (
	domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	typeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// ValidateDomain returns a FieldError for a domain value, or nil if valid.
// Blank -> "must not be blank"; otherwise must match the slug pattern and not be "kg".
func ValidateDomain(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	if v == "kg" || !domainRe.MatchString(v) {
		return &FieldError{Field: field, RejectedValue: v, HasRejected: true, Message: domainMsg}
	}
	return nil
}

// ValidateType returns a FieldError for a type value, or nil if valid.
func ValidateType(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	if !typeRe.MatchString(v) {
		return &FieldError{Field: field, RejectedValue: v, HasRejected: true, Message: typeMsg}
	}
	return nil
}

func validateBlank(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	return nil
}

// ValidateEntity validates an entity upsert body. Errors are returned in a
// deterministic declaration order (domain, type, name, ttlSeconds).
func ValidateEntity(r EntityWriteRequest) []FieldError {
	var errs []FieldError
	if e := ValidateDomain("domain", r.Domain); e != nil {
		errs = append(errs, *e)
	}
	if e := ValidateType("type", r.Type); e != nil {
		errs = append(errs, *e)
	}
	if e := validateBlank("name", r.Name); e != nil {
		errs = append(errs, *e)
	}
	if r.TTLSeconds == nil {
		errs = append(errs, FieldError{Field: "ttlSeconds", Message: nullMsg})
	}
	return errs
}

// ValidateRelationship validates a relationship upsert body.
// Order: domain, type, from, to, ttlSeconds.
func ValidateRelationship(r RelationshipWriteRequest) []FieldError {
	var errs []FieldError
	if e := ValidateDomain("domain", r.Domain); e != nil {
		errs = append(errs, *e)
	}
	if e := ValidateType("type", r.Type); e != nil {
		errs = append(errs, *e)
	}
	if r.From == nil {
		errs = append(errs, FieldError{Field: "from", Message: nullMsg})
	}
	if r.To == nil {
		errs = append(errs, FieldError{Field: "to", Message: nullMsg})
	}
	if r.TTLSeconds == nil {
		errs = append(errs, FieldError{Field: "ttlSeconds", Message: nullMsg})
	}
	return errs
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/model/ -v`
Expected: PASS (all model tests).

- [ ] **Step 6: Commit**

```bash
git add kg-write-twin/internal/model/dto.go kg-write-twin/internal/model/validate.go kg-write-twin/internal/model/validate_test.go
git commit -m "feat(kg-write-twin): request DTOs + validation rules"
```

---

## Task 4: Store foundation (keys + expiry)

**Files:**
- Create: `kg-write-twin/internal/store/store.go`
- Test: `kg-write-twin/internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/store/store_test.go`:
```go
package store

import (
	"math"
	"testing"
)

func TestComputeExpiry(t *testing.T) {
	const now = int64(1000)
	cases := []struct {
		ttl            int64
		wantExpiresAt  int64
		wantExpired    int64
		wantZero       bool
	}{
		{10, now + 10000, 0, false},
		{-1, math.MaxInt64, 0, false},
		{-5, math.MaxInt64, 0, false},
		{0, 0, now, true},
	}
	for _, c := range cases {
		ea, ex, zero := computeExpiry(now, c.ttl)
		if ea != c.wantExpiresAt || ex != c.wantExpired || zero != c.wantZero {
			t.Errorf("ttl=%d -> (%d,%d,%v) want (%d,%d,%v)", c.ttl, ea, ex, zero, c.wantExpiresAt, c.wantExpired, c.wantZero)
		}
	}
}

func TestEntityKeyScopeOrderIndependent(t *testing.T) {
	a := entityKey("irm", "Team", "x", map[string]string{"a": "1", "b": "2"})
	b := entityKey("irm", "Team", "x", map[string]string{"b": "2", "a": "1"})
	if a != b {
		t.Errorf("scope key order should not matter: %q vs %q", a, b)
	}
	c := entityKey("irm", "Team", "x", map[string]string{"a": "1"})
	if a == c {
		t.Errorf("different scope should produce different key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/store/ -run 'TestComputeExpiry|TestEntityKey' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write store.go**

`kg-write-twin/internal/store/store.go`:
```go
package store

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// OriginAPI is the origin assigned to all writes made through the spec API.
const OriginAPI = "api"

// Ref is an entity endpoint of a relationship.
type Ref struct {
	Domain string            `json:"domain"`
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Scope  map[string]string `json:"scope"`
}

// Entity is a stored entity (also the /_control/state representation).
type Entity struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin"`
	ExpiresAt  int64             `json:"expiresAt,omitempty"`
	Expired    int64             `json:"expired,omitempty"`
	TTLZero    bool              `json:"ttlZero"`
	CreatedAt  int64             `json:"createdAt"`
}

// Relationship is a stored relationship (also the /_control/state representation).
type Relationship struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	From       Ref               `json:"from"`
	To         Ref               `json:"to"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin"`
	ExpiresAt  int64             `json:"expiresAt,omitempty"`
	Expired    int64             `json:"expired,omitempty"`
	TTLZero    bool              `json:"ttlZero"`
	CreatedAt  int64             `json:"createdAt"`
}

// EntityInput is the write payload for an entity (origin + ttl explicit).
type EntityInput struct {
	Domain, Type, Name string
	Scope, Properties  map[string]string
	Origin             string
	TTLSeconds         int64
}

// RelationshipInput is the write payload for a relationship.
type RelationshipInput struct {
	Domain, Type string
	From, To     Ref
	Properties   map[string]string
	Origin       string
	TTLSeconds   int64
}

type tenantData struct {
	entities      map[string]*Entity
	relationships map[string]*Relationship
}

// Store is an in-memory, tenant-partitioned graph store.
type Store struct {
	mu      sync.RWMutex
	now     func() int64 // epoch millis
	tenants map[string]*tenantData
}

// New creates a Store. now must return epoch milliseconds.
func New(now func() int64) *Store {
	return &Store{now: now, tenants: map[string]*tenantData{}}
}

func (s *Store) tenant(id string) *tenantData {
	td := s.tenants[id]
	if td == nil {
		td = &tenantData{entities: map[string]*Entity{}, relationships: map[string]*Relationship{}}
		s.tenants[id] = td
	}
	return td
}

func computeExpiry(nowMS, ttl int64) (expiresAt, expired int64, zero bool) {
	switch {
	case ttl == 0:
		return 0, nowMS, true
	case ttl < 0:
		return math.MaxInt64, 0, false
	default:
		return nowMS + ttl*1000, 0, false
	}
}

func scopeKey(scope map[string]string) string {
	if len(scope) == 0 {
		return ""
	}
	keys := make([]string, 0, len(scope))
	for k := range scope {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s;", k, scope[k])
	}
	return b.String()
}

func entityKey(domain, typ, name string, scope map[string]string) string {
	return domain + "\x00" + typ + "\x00" + name + "\x00" + scopeKey(scope)
}

func relKey(typ string, from, to Ref) string {
	return typ + "\x00" +
		entityKey(from.Domain, from.Type, from.Name, from.Scope) + "\x00->\x00" +
		entityKey(to.Domain, to.Type, to.Name, to.Scope)
}

func (e *Entity) isExpired(nowMS int64) bool {
	if e.TTLZero {
		return true
	}
	return e.ExpiresAt != math.MaxInt64 && nowMS >= e.ExpiresAt
}

func (r *Relationship) isExpired(nowMS int64) bool {
	if r.TTLZero {
		return true
	}
	return r.ExpiresAt != math.MaxInt64 && nowMS >= r.ExpiresAt
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/store/ -run 'TestComputeExpiry|TestEntityKey' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/store/store.go kg-write-twin/internal/store/store_test.go
git commit -m "feat(kg-write-twin): store foundation — types, keys, expiry"
```

---

## Task 5: Store entity upsert/delete

**Files:**
- Create: `kg-write-twin/internal/store/entity.go`
- Test: `kg-write-twin/internal/store/entity_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/store/entity_test.go`:
```go
package store

import "testing"

func fixedNow() int64 { return 1000 }

func TestUpsertEntityCreateThenUpdate(t *testing.T) {
	s := New(fixedNow)
	in := EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 10}
	if _, r := s.UpsertEntity("13", in); r != Created {
		t.Fatalf("first upsert = %v, want Created", r)
	}
	if _, r := s.UpsertEntity("13", in); r != Updated {
		t.Fatalf("second upsert = %v, want Updated", r)
	}
}

func TestUpsertEntityOriginConflict(t *testing.T) {
	s := New(fixedNow)
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: "inference", TTLSeconds: 10})
	if _, r := s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 10}); r != OriginConflict {
		t.Fatalf("upsert over non-api = %v, want OriginConflict", r)
	}
}

func TestDeleteEntity(t *testing.T) {
	s := New(fixedNow)
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 10})
	if r := s.DeleteEntity("13", "irm", "Team", "a", nil); r != Deleted {
		t.Fatalf("delete = %v, want Deleted", r)
	}
	if r := s.DeleteEntity("13", "irm", "Team", "a", nil); r != NotFound {
		t.Fatalf("re-delete = %v, want NotFound", r)
	}
	// recreate after delete -> Created again
	if _, r := s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 10}); r != Created {
		t.Fatalf("recreate = %v, want Created", r)
	}
}

func TestDeleteEntityOriginConflict(t *testing.T) {
	s := New(fixedNow)
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: "inference", TTLSeconds: 10})
	if r := s.DeleteEntity("13", "irm", "Team", "a", nil); r != DeleteOriginConflict {
		t.Fatalf("delete non-api = %v, want DeleteOriginConflict", r)
	}
}

func TestExpiryMetadataIgnoredForExistence(t *testing.T) {
	now := int64(1000)
	s := New(func() int64 { return now })
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 1})
	now = 1_000_000 // far past expiry
	if r := s.DeleteEntity("13", "irm", "Team", "a", nil); r != Deleted {
		t.Fatalf("expired entity should still be deletable, got %v", r)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/store/ -run 'Entity' -v`
Expected: FAIL (undefined UpsertEntity/Created/etc).

- [ ] **Step 3: Write entity.go**

`kg-write-twin/internal/store/entity.go`:
```go
package store

// UpsertResult is the outcome of an entity/edge upsert.
type UpsertResult int

const (
	Created UpsertResult = iota
	Updated
	OriginConflict
)

// DeleteResult is the outcome of a delete.
type DeleteResult int

const (
	Deleted DeleteResult = iota
	NotFound
	DeleteOriginConflict
)

// UpsertEntity creates or updates an entity. Existing objects with a non-API
// origin cause OriginConflict. Expiry is metadata only and never blocks writes.
func (s *Store) UpsertEntity(tenant string, in EntityInput) (Entity, UpsertResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := entityKey(in.Domain, in.Type, in.Name, in.Scope)
	existing, ok := td.entities[key]
	if ok && existing.Origin != OriginAPI {
		return *existing, OriginConflict
	}
	now := s.now()
	ea, ex, zero := computeExpiry(now, in.TTLSeconds)
	e := &Entity{
		Domain: in.Domain, Type: in.Type, Name: in.Name,
		Scope: in.Scope, Properties: in.Properties,
		Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
	}
	td.entities[key] = e
	if ok {
		return *e, Updated
	}
	return *e, Created
}

// DeleteEntity removes an entity by identity. Non-API-origin targets cause
// DeleteOriginConflict; absent targets cause NotFound.
func (s *Store) DeleteEntity(tenant, domain, typ, name string, scope map[string]string) DeleteResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := entityKey(domain, typ, name, scope)
	existing, ok := td.entities[key]
	if !ok {
		return NotFound
	}
	if existing.Origin != OriginAPI {
		return DeleteOriginConflict
	}
	delete(td.entities, key)
	return Deleted
}

// entityExists reports whether an entity identity is present (expiry ignored).
// Caller must hold the lock.
func (s *Store) entityExists(td *tenantData, ref Ref) bool {
	_, ok := td.entities[entityKey(ref.Domain, ref.Type, ref.Name, ref.Scope)]
	return ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/store/ -run 'Entity|Expiry' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/store/entity.go kg-write-twin/internal/store/entity_test.go
git commit -m "feat(kg-write-twin): store entity upsert/delete with origin + result codes"
```

---

## Task 6: Store relationship upsert/delete

**Files:**
- Create: `kg-write-twin/internal/store/relationship.go`
- Test: `kg-write-twin/internal/store/relationship_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/store/relationship_test.go`:
```go
package store

import "testing"

func seedEntity(s *Store, name string) {
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Service", Name: name, Origin: OriginAPI, TTLSeconds: 3600})
}

func ref(name string) Ref { return Ref{Domain: "irm", Type: "Service", Name: name} }

func TestUpsertRelationship(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "a")
	seedEntity(s, "b")
	in := RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60}
	if _, r := s.UpsertRelationship("13", in); r != RelOK {
		t.Fatalf("create rel = %v, want RelOK", r)
	}
	if _, r := s.UpsertRelationship("13", in); r != RelOK {
		t.Fatalf("update rel = %v, want RelOK (always 200)", r)
	}
}

func TestUpsertRelationshipMissingEndpoints(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "b")
	// from missing (checked first)
	if _, r := s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("ghost"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60}); r != FromNotFound {
		t.Fatalf("missing from = %v, want FromNotFound", r)
	}
	seedEntity(s, "a")
	if _, r := s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("ghost"), Origin: OriginAPI, TTLSeconds: 60}); r != ToNotFound {
		t.Fatalf("missing to = %v, want ToNotFound", r)
	}
}

func TestDeleteRelationship(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "a")
	seedEntity(s, "b")
	s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60})
	if r := s.DeleteRelationship("13", "uses", ref("a"), ref("b")); r != Deleted {
		t.Fatalf("delete rel = %v, want Deleted", r)
	}
	if r := s.DeleteRelationship("13", "uses", ref("a"), ref("b")); r != NotFound {
		t.Fatalf("re-delete rel = %v, want NotFound", r)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/store/ -run Relationship -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write relationship.go**

`kg-write-twin/internal/store/relationship.go`:
```go
package store

// RelUpsertResult is the outcome of a relationship upsert.
type RelUpsertResult int

const (
	RelOK RelUpsertResult = iota // create or update — always 200
	FromNotFound
	ToNotFound
	RelOriginConflict
)

func normalizeRefScope(r Ref) Ref {
	if r.Scope == nil {
		r.Scope = map[string]string{}
	}
	return r
}

// UpsertRelationship creates or updates an edge. Both endpoints must already
// exist (expiry ignored); "from" is checked before "to". A matching non-API
// edge causes RelOriginConflict.
func (s *Store) UpsertRelationship(tenant string, in RelationshipInput) (Relationship, RelUpsertResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	if !s.entityExists(td, in.From) {
		return Relationship{}, FromNotFound
	}
	if !s.entityExists(td, in.To) {
		return Relationship{}, ToNotFound
	}
	from := normalizeRefScope(in.From)
	to := normalizeRefScope(in.To)
	key := relKey(in.Type, from, to)
	if existing, ok := td.relationships[key]; ok && existing.Origin != OriginAPI {
		return *existing, RelOriginConflict
	}
	now := s.now()
	ea, ex, zero := computeExpiry(now, in.TTLSeconds)
	rel := &Relationship{
		Domain: in.Domain, Type: in.Type, From: from, To: to,
		Properties: in.Properties, Origin: in.Origin,
		ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
	}
	td.relationships[key] = rel
	return *rel, RelOK
}

// DeleteRelationship removes an edge by (type, from, to). Non-API edges cause
// DeleteOriginConflict; absent edges cause NotFound.
func (s *Store) DeleteRelationship(tenant, typ string, from, to Ref) DeleteResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := relKey(typ, normalizeRefScope(from), normalizeRefScope(to))
	existing, ok := td.relationships[key]
	if !ok {
		return NotFound
	}
	if existing.Origin != OriginAPI {
		return DeleteOriginConflict
	}
	delete(td.relationships, key)
	return Deleted
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/store/ -run Relationship -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/store/relationship.go kg-write-twin/internal/store/relationship_test.go
git commit -m "feat(kg-write-twin): store relationship upsert/delete with endpoint checks"
```

---

## Task 7: Store control ops (Reset / Seed / Query)

**Files:**
- Create: `kg-write-twin/internal/store/control.go`
- Test: `kg-write-twin/internal/store/control_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/store/control_test.go`:
```go
package store

import "testing"

func TestReset(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "a")
	s.UpsertEntity("99", EntityInput{Domain: "irm", Type: "Team", Name: "z", Origin: OriginAPI, TTLSeconds: 10})
	s.Reset("13")
	if got := len(s.Query(Filter{Tenant: "13", IncludeExpired: true}).Entities); got != 0 {
		t.Fatalf("tenant 13 entities after reset = %d, want 0", got)
	}
	if got := len(s.Query(Filter{Tenant: "99", IncludeExpired: true}).Entities); got != 1 {
		t.Fatalf("tenant 99 entities after reset(13) = %d, want 1", got)
	}
	s.Reset("") // reset all
	if got := len(s.Query(Filter{IncludeExpired: true}).Entities); got != 0 {
		t.Fatalf("all entities after reset-all = %d, want 0", got)
	}
}

func TestQueryFilters(t *testing.T) {
	s := New(fixedNow)
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 10})
	s.UpsertEntity("13", EntityInput{Domain: "cmdb", Type: "Host", Name: "h", Origin: "inference", TTLSeconds: 10})

	if got := len(s.Query(Filter{Tenant: "13", Domain: "irm", IncludeExpired: true}).Entities); got != 1 {
		t.Errorf("domain filter = %d, want 1", got)
	}
	if got := len(s.Query(Filter{Tenant: "13", Origin: "inference", IncludeExpired: true}).Entities); got != 1 {
		t.Errorf("origin filter = %d, want 1", got)
	}
	if got := len(s.Query(Filter{Tenant: "13", Kind: "relationship", IncludeExpired: true}).Entities); got != 0 {
		t.Errorf("kind=relationship should return no entities, got %d", got)
	}
}

func TestQueryIncludeExpired(t *testing.T) {
	now := int64(1000)
	s := New(func() int64 { return now })
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Team", Name: "a", Origin: OriginAPI, TTLSeconds: 1})
	now = 1_000_000
	if got := len(s.Query(Filter{Tenant: "13", IncludeExpired: true}).Entities); got != 1 {
		t.Errorf("includeExpired=true = %d, want 1", got)
	}
	if got := len(s.Query(Filter{Tenant: "13", IncludeExpired: false}).Entities); got != 0 {
		t.Errorf("includeExpired=false = %d, want 0", got)
	}
}

func TestSeed(t *testing.T) {
	s := New(fixedNow)
	s.Seed("13",
		[]EntityInput{{Domain: "irm", Type: "Team", Name: "a", Origin: "inference", TTLSeconds: 10}},
		nil)
	res := s.Query(Filter{Tenant: "13", IncludeExpired: true})
	if len(res.Entities) != 1 || res.Entities[0].Origin != "inference" {
		t.Fatalf("seeded entity = %+v, want one with origin=inference", res.Entities)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/store/ -run 'Reset|Query|Seed' -v`
Expected: FAIL (undefined Filter/Query/Reset/Seed).

- [ ] **Step 3: Write control.go**

`kg-write-twin/internal/store/control.go`:
```go
package store

import "sort"

// Filter selects objects for Query. Empty string fields match anything.
type Filter struct {
	Tenant         string
	Kind           string // "", "entity", "relationship"
	Domain         string
	Type           string
	Name           string
	Origin         string
	IncludeExpired bool
}

// QueryResult is the /_control/state payload.
type QueryResult struct {
	Entities      []Entity       `json:"entities"`
	Relationships []Relationship `json:"relationships"`
}

// Reset clears one tenant, or all tenants when tenant == "".
func (s *Store) Reset(tenant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenant == "" {
		s.tenants = map[string]*tenantData{}
		return
	}
	delete(s.tenants, tenant)
}

// Seed inserts entities/relationships with explicit origin (used to create
// non-API-origin objects for 409/403 testing, and to preload fixtures).
func (s *Store) Seed(tenant string, ents []EntityInput, rels []RelationshipInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	now := s.now()
	for _, in := range ents {
		ea, ex, zero := computeExpiry(now, in.TTLSeconds)
		td.entities[entityKey(in.Domain, in.Type, in.Name, in.Scope)] = &Entity{
			Domain: in.Domain, Type: in.Type, Name: in.Name, Scope: in.Scope, Properties: in.Properties,
			Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
		}
	}
	for _, in := range rels {
		ea, ex, zero := computeExpiry(now, in.TTLSeconds)
		from, to := normalizeRefScope(in.From), normalizeRefScope(in.To)
		td.relationships[relKey(in.Type, from, to)] = &Relationship{
			Domain: in.Domain, Type: in.Type, From: from, To: to, Properties: in.Properties,
			Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
		}
	}
}

// Query returns matching objects, sorted for deterministic output.
func (s *Store) Query(f Filter) QueryResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	res := QueryResult{Entities: []Entity{}, Relationships: []Relationship{}}

	forTenant := func(id string, td *tenantData) {
		if f.Kind != "relationship" {
			for _, e := range td.entities {
				if !f.IncludeExpired && e.isExpired(now) {
					continue
				}
				if (f.Domain == "" || e.Domain == f.Domain) &&
					(f.Type == "" || e.Type == f.Type) &&
					(f.Name == "" || e.Name == f.Name) &&
					(f.Origin == "" || e.Origin == f.Origin) {
					res.Entities = append(res.Entities, *e)
				}
			}
		}
		if f.Kind != "entity" {
			for _, r := range td.relationships {
				if !f.IncludeExpired && r.isExpired(now) {
					continue
				}
				if (f.Domain == "" || r.Domain == f.Domain) &&
					(f.Type == "" || r.Type == f.Type) &&
					(f.Origin == "" || r.Origin == f.Origin) {
					res.Relationships = append(res.Relationships, *r)
				}
			}
		}
	}

	if f.Tenant != "" {
		if td := s.tenants[f.Tenant]; td != nil {
			forTenant(f.Tenant, td)
		}
	} else {
		for id, td := range s.tenants {
			forTenant(id, td)
		}
	}

	sort.Slice(res.Entities, func(i, j int) bool {
		return res.Entities[i].Type+res.Entities[i].Name < res.Entities[j].Type+res.Entities[j].Name
	})
	sort.Slice(res.Relationships, func(i, j int) bool {
		return res.Relationships[i].Type+res.Relationships[i].From.Name < res.Relationships[j].Type+res.Relationships[j].From.Name
	})
	return res
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/store/ -v`
Expected: PASS (all store tests).

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/store/control.go kg-write-twin/internal/store/control_test.go
git commit -m "feat(kg-write-twin): store control ops — reset, seed, filtered query"
```

---

## Task 8: API errors + request id

**Files:**
- Create: `kg-write-twin/internal/api/errors.go`
- Test: `kg-write-twin/internal/api/errors_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/api/errors_test.go`:
```go
package api

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/grafana/kg-write-twin/internal/model"
)

func TestRequestIDFormat(t *testing.T) {
	// 16 chars, hex with possible leading spaces (space-padded %16x).
	re := regexp.MustCompile(`^[ 0-9a-f]{16}$`)
	for i := 0; i < 50; i++ {
		id := newRequestID()
		if len(id) != 16 || !re.MatchString(id) {
			t.Fatalf("bad request id %q", id)
		}
	}
}

func TestErrorBodyShape(t *testing.T) {
	e := apiError{httpCode: 422, status: "UNPROCESSABLE_ENTITY", message: "Invalid request",
		subErrors: []model.FieldError{{Field: "domain", Message: "must not be blank"}}}
	b := e.body(func() int64 { return 1234 })
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "UNPROCESSABLE_ENTITY" || got["message"] != "Invalid request" {
		t.Errorf("body = %v", got)
	}
	if _, ok := got["subErrors"]; !ok {
		t.Errorf("subErrors missing: %v", got)
	}
	if _, ok := got["requestId"].(string); !ok {
		t.Errorf("requestId missing/not string: %v", got)
	}
	if got["timestamp"].(float64) != 1234 {
		t.Errorf("timestamp = %v, want 1234", got["timestamp"])
	}
}

func TestErrorBodyOmitsEmptySubErrors(t *testing.T) {
	e := apiError{httpCode: 404, status: "NOT_FOUND", message: "entity not found"}
	b := e.body(func() int64 { return 1 })
	var got map[string]any
	json.Unmarshal(b, &got)
	if _, ok := got["subErrors"]; ok {
		t.Errorf("subErrors should be omitted when empty: %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/api/ -run 'RequestID|ErrorBody' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write errors.go**

`kg-write-twin/internal/api/errors.go`:
```go
package api

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/grafana/kg-write-twin/internal/model"
)

// apiError is an internal representation mapped to an ApiError response body.
type apiError struct {
	httpCode  int
	status    string
	message   string
	subErrors []model.FieldError
}

// newRequestID mimics the real API's String.format("%16x", rnd): 16-wide,
// space-padded hex of a random 64-bit value.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%16x", binary.BigEndian.Uint64(b[:]))
}

// body renders the ApiError JSON with field order status, requestId, timestamp,
// message, subErrors (subErrors omitted when empty).
func (e apiError) body(now func() int64) []byte {
	type errorBody struct {
		Status    string             `json:"status"`
		RequestID string             `json:"requestId"`
		Timestamp int64              `json:"timestamp"`
		Message   string             `json:"message"`
		SubErrors []model.FieldError `json:"subErrors,omitempty"`
	}
	eb := errorBody{
		Status: e.status, RequestID: newRequestID(), Timestamp: now(),
		Message: e.message, SubErrors: e.subErrors,
	}
	out, _ := json.Marshal(eb)
	return out
}

// Constructors for the fixed error shapes captured from the real API.
func errTenantMissing() apiError {
	return apiError{424, "FAILED_DEPENDENCY", "No tenant selected for request", nil}
}
func errBadNamespace() apiError {
	return apiError{400, "BAD_REQUEST", "namespace must be of the form 'stacks-<stackId>'", nil}
}
func errNamespaceMismatch() apiError {
	return apiError{403, "FORBIDDEN", "namespace does not match the request tenant", nil}
}
func errTenantInit(v string) apiError {
	return apiError{500, "INTERNAL_SERVER_ERROR", fmt.Sprintf("Failed initializing tenantId=%s, cannot continue", v), nil}
}
func errUnsupportedMedia(ct string) apiError {
	return apiError{415, "UNSUPPORTED_MEDIA_TYPE", fmt.Sprintf("Content-Type '%s' is not supported", ct), nil}
}
func errParse(detail string) apiError {
	return apiError{400, "BAD_REQUEST", "JSON parse error: " + detail, nil}
}
func errValidation(message string, subErrors []model.FieldError) apiError {
	return apiError{422, "UNPROCESSABLE_ENTITY", message, subErrors}
}
func errNotFound(message string) apiError {
	return apiError{404, "NOT_FOUND", message, nil}
}
func errConflict(message string) apiError {
	return apiError{409, "CONFLICT", message, nil}
}
func errForbidden(message string) apiError {
	return apiError{403, "FORBIDDEN", message, nil}
}
func errMethodNotAllowed(method string) apiError {
	return apiError{405, "METHOD_NOT_ALLOWED", fmt.Sprintf("Request method '%s' is not supported", method), nil}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run 'RequestID|ErrorBody' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/api/errors.go kg-write-twin/internal/api/errors_test.go
git commit -m "feat(kg-write-twin): api error model + space-padded requestId"
```

---

## Task 9: Content negotiation codec

**Files:**
- Create: `kg-write-twin/internal/api/codec.go`
- Test: `kg-write-twin/internal/api/codec_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/api/codec_test.go`:
```go
package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/model"
)

func TestDecodeJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader(`{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}`))
	r.Header.Set("Content-Type", "application/json")
	var req model.EntityWriteRequest
	if e := decodeBody(r, &req); e != nil {
		t.Fatalf("decode json: %+v", e)
	}
	if req.Domain != "irm" || req.TTLSeconds == nil || *req.TTLSeconds != 10 {
		t.Errorf("decoded = %+v", req)
	}
}

func TestDecodeYAML(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader("domain: irm\ntype: Team\nname: a\nttlSeconds: 10\nscope:\n  env: 5\n"))
	r.Header.Set("Content-Type", "application/x-yaml")
	var req model.EntityWriteRequest
	if e := decodeBody(r, &req); e != nil {
		t.Fatalf("decode yaml: %+v", e)
	}
	if req.Domain != "irm" || req.Scope["env"] != "5" {
		t.Errorf("decoded = %+v", req)
	}
}

func TestDecodeUnsupportedMedia(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader("hi"))
	r.Header.Set("Content-Type", "text/plain")
	var req model.EntityWriteRequest
	e := decodeBody(r, &req)
	if e == nil || e.httpCode != 415 {
		t.Fatalf("want 415, got %+v", e)
	}
}

func TestDecodeMalformedJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	var req model.EntityWriteRequest
	e := decodeBody(r, &req)
	if e == nil || e.httpCode != 400 || !strings.HasPrefix(e.message, "JSON parse error:") {
		t.Fatalf("want 400 parse error, got %+v", e)
	}
}

func TestResponseFormat(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("Accept", "application/x-yaml")
	if responseFormat(r) != formatYAML {
		t.Errorf("want yaml")
	}
	r2 := httptest.NewRequest("POST", "/x", nil)
	if responseFormat(r2) != formatJSON {
		t.Errorf("want json default")
	}
}

func TestEncode(t *testing.T) {
	payload := map[string]any{"a": "b"}
	if got := string(encode(formatJSON, payload)); got != `{"a":"b"}` {
		t.Errorf("json encode = %s", got)
	}
	if got := string(encode(formatYAML, payload)); !strings.Contains(got, "a: b") {
		t.Errorf("yaml encode = %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/api/ -run 'Decode|ResponseFormat|Encode' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write codec.go**

`kg-write-twin/internal/api/codec.go`:
```go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

type format int

const (
	formatJSON format = iota
	formatYAML
)

func contentTypeBase(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

func isYAML(ct string) bool { return ct == "application/x-yaml" || ct == "application/x-yml" }

// decodeBody reads the request body per Content-Type (json default; x-yaml/x-yml
// converted to json) and unmarshals into dst. Returns an apiError on 415/400.
func decodeBody(r *http.Request, dst any) *apiError {
	ct := contentTypeBase(r.Header.Get("Content-Type"))
	data, _ := io.ReadAll(r.Body)
	switch {
	case ct == "" || ct == "application/json":
		if err := json.Unmarshal(data, dst); err != nil {
			e := errParse(err.Error())
			return &e
		}
	case isYAML(ct):
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			e := errParse(err.Error())
			return &e
		}
		jb, err := json.Marshal(generic)
		if err != nil {
			e := errParse(err.Error())
			return &e
		}
		if err := json.Unmarshal(jb, dst); err != nil {
			e := errParse(err.Error())
			return &e
		}
	default:
		e := errUnsupportedMedia(contentTypeBase(r.Header.Get("Content-Type")))
		return &e
	}
	return nil
}

func responseFormat(r *http.Request) format {
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/x-yaml") || strings.Contains(accept, "application/x-yml") {
		return formatYAML
	}
	return formatJSON
}

// encode marshals payload as JSON, or converts JSON->YAML for the yaml format so
// keys come from json struct tags.
func encode(f format, payload any) []byte {
	jb, _ := json.Marshal(payload)
	if f == formatJSON {
		return jb
	}
	var generic any
	_ = json.Unmarshal(jb, &generic)
	yb, _ := yaml.Marshal(generic)
	return yb
}

func contentType(f format) string {
	if f == formatYAML {
		return "application/x-yaml"
	}
	return "application/json"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run 'Decode|ResponseFormat|Encode' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/api/codec.go kg-write-twin/internal/api/codec_test.go
git commit -m "feat(kg-write-twin): content-negotiation codec (json/yaml, 415/400)"
```

---

## Task 10: Tenant + namespace resolution

**Files:**
- Create: `kg-write-twin/internal/api/tenant.go`
- Test: `kg-write-twin/internal/api/tenant_test.go`

- [ ] **Step 1: Write the failing test**

`kg-write-twin/internal/api/tenant_test.go`:
```go
package api

import (
	"net/http/httptest"
	"testing"
)

func TestResolveTenant(t *testing.T) {
	cases := []struct {
		name      string
		org       string // "" means header absent
		namespace string
		wantID    string
		wantCode  int // 0 = success
	}{
		{"ok", "13", "stacks-13", "13", 0},
		{"missing header", "", "stacks-13", "", 424},
		{"bad namespace", "13", "foo", "", 400},
		{"mismatch", "13", "stacks-99", "", 403},
		{"non-numeric tenant", "abc", "stacks-abc", "", 500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/x", nil)
			if c.org != "" {
				r.Header.Set("X-Scope-OrgID", c.org)
			}
			id, e := resolveTenant(r, c.namespace)
			if c.wantCode == 0 {
				if e != nil {
					t.Fatalf("want success, got %+v", e)
				}
				if id != c.wantID {
					t.Errorf("id = %q, want %q", id, c.wantID)
				}
				return
			}
			if e == nil || e.httpCode != c.wantCode {
				t.Fatalf("want code %d, got %+v", c.wantCode, e)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kg-write-twin && go test ./internal/api/ -run ResolveTenant -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write tenant.go**

`kg-write-twin/internal/api/tenant.go`:
```go
package api

import (
	"net/http"
	"strconv"
	"strings"
)

// resolveTenant applies the real API's check order:
//  1. missing X-Scope-OrgID           -> 424
//  2. namespace not "stacks-<id>"     -> 400
//  3. namespace stackId != tenant     -> 403
//  4. tenant not an integer           -> 500
// On success it returns the tenant id (the org header value).
func resolveTenant(r *http.Request, namespace string) (string, *apiError) {
	org := r.Header.Get("X-Scope-OrgID")
	if org == "" {
		e := errTenantMissing()
		return "", &e
	}
	stackID, ok := strings.CutPrefix(namespace, "stacks-")
	if !ok || stackID == "" {
		e := errBadNamespace()
		return "", &e
	}
	if stackID != org {
		e := errNamespaceMismatch()
		return "", &e
	}
	if _, err := strconv.Atoi(org); err != nil {
		e := errTenantInit(org)
		return "", &e
	}
	return org, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run ResolveTenant -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/api/tenant.go kg-write-twin/internal/api/tenant_test.go
git commit -m "feat(kg-write-twin): tenant + namespace resolution (424/400/403/500)"
```

---

## Task 11: Response builders + entity handlers + router

**Files:**
- Create: `kg-write-twin/internal/api/respond.go`
- Create: `kg-write-twin/internal/api/server.go`
- Create: `kg-write-twin/internal/api/entities.go`
- Test: `kg-write-twin/internal/api/entities_test.go`

- [ ] **Step 1: Write respond.go**

`kg-write-twin/internal/api/respond.go`:
```go
package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/store"
)

// enrichedProps builds the response "properties": user props merged with
// computed _domain, _origin, and _expires_at (or _expired when ttl==0).
func enrichedProps(domain, origin string, props map[string]string, expiresAt, expired int64, ttlZero bool) map[string]any {
	out := map[string]any{"_domain": domain}
	for k, v := range props {
		out[k] = v
	}
	out["_origin"] = origin
	if ttlZero {
		out["_expired"] = expired
	} else {
		out["_expires_at"] = expiresAt
	}
	return out
}

type entityResponse struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]any    `json:"properties"`
}

func entityBody(e store.Entity) entityResponse {
	return entityResponse{
		Domain: e.Domain, Type: e.Type, Name: e.Name, Scope: e.Scope,
		Properties: enrichedProps(e.Domain, e.Origin, e.Properties, e.ExpiresAt, e.Expired, e.TTLZero),
	}
}

type refResponse struct {
	Domain string            `json:"domain"`
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Scope  map[string]string `json:"scope"` // always present, {} when empty
}

func refBody(r store.Ref) refResponse {
	sc := r.Scope
	if sc == nil {
		sc = map[string]string{}
	}
	return refResponse{Domain: r.Domain, Type: r.Type, Name: r.Name, Scope: sc}
}

type relationshipResponse struct {
	Domain     string         `json:"domain"`
	Type       string         `json:"type"`
	From       refResponse    `json:"from"`
	To         refResponse    `json:"to"`
	Properties map[string]any `json:"properties"`
}

func relationshipBody(r store.Relationship) relationshipResponse {
	return relationshipResponse{
		Domain: r.Domain, Type: r.Type, From: refBody(r.From), To: refBody(r.To),
		Properties: enrichedProps(r.Domain, r.Origin, r.Properties, r.ExpiresAt, r.Expired, r.TTLZero),
	}
}
```

(`net/http` is used by the shared write helpers added in the next step.)

- [ ] **Step 2: Add the shared write helpers to respond.go**

Append to `kg-write-twin/internal/api/respond.go`:
```go
// writeError renders an apiError to the client (respecting Accept).
func writeError(w http.ResponseWriter, r *http.Request, now func() int64, e apiError) {
	f := responseFormat(r)
	w.Header().Set("Content-Type", contentType(f))
	w.WriteHeader(e.httpCode)
	// e.body() produces JSON; convert to yaml if needed.
	jb := e.body(now)
	if f == formatYAML {
		w.Write(encode(formatYAML, rawJSON(jb)))
		return
	}
	w.Write(jb)
}

// writeSuccess renders a success payload (respecting Accept).
func writeSuccess(w http.ResponseWriter, r *http.Request, status int, payload any) {
	f := responseFormat(r)
	w.Header().Set("Content-Type", contentType(f))
	w.WriteHeader(status)
	w.Write(encode(f, payload))
}

// writeJSON renders JSON only (used by the /_control surface).
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(encode(formatJSON, payload))
}
```

Add the `rawJSON` helper to `codec.go`:
```go
// rawJSON re-parses JSON bytes into a generic value so encode(formatYAML,...)
// can render them as YAML.
func rawJSON(b []byte) any {
	var v any
	_ = json.Unmarshal(b, &v)
	return v
}
```

- [ ] **Step 3: Write server.go (router + middleware + 404/405)**

`kg-write-twin/internal/api/server.go`:
```go
package api

import (
	"net/http"
	"strings"

	"github.com/grafana/kg-write-twin/internal/store"
)

type handlers struct {
	store    *store.Store
	now      func() int64
	basePath string
}

// NewServer builds the HTTP handler. basePath defaults to "/api-server".
func NewServer(st *store.Store, basePath string, now func() int64) http.Handler {
	h := &handlers{store: st, now: now, basePath: basePath}
	mux := http.NewServeMux()
	p := basePath

	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities", h.entities)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities/{type}/{name}", h.entityItem)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships", h.relationships)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships/{type}", h.relationshipItem)
	mux.HandleFunc(p+"/_control/reset", h.methodGate(http.MethodPost, h.reset))
	mux.HandleFunc(p+"/_control/state", h.methodGate(http.MethodGet, h.state))
	mux.HandleFunc(p+"/_control/seed", h.methodGate(http.MethodPost, h.seed))
	mux.HandleFunc(p+"/_control/healthz", h.methodGate(http.MethodGet, h.healthz))
	mux.HandleFunc("/", h.notFound)

	return securityHeaders(mux)
}

// methodGate returns a handler that enforces a single method, else 405.
func (h *handlers) methodGate(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeError(w, r, h.now, errMethodNotAllowed(r.Method))
			return
		}
		next(w, r)
	}
}

// entities routes POST; other methods -> 405.
func (h *handlers) entities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.upsertEntity(w, r)
}

func (h *handlers) entityItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.deleteEntity(w, r)
}

func (h *handlers) relationships(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.upsertRelationship(w, r)
}

func (h *handlers) relationshipItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.deleteRelationship(w, r)
}

// notFound reproduces the real "No static resource <path>." message.
func (h *handlers) notFound(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, h.basePath+"/")
	writeError(w, r, h.now, errNotFound("No static resource "+rel+"."))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("X-XSS-Protection", "0")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		hd.Set("Pragma", "no-cache")
		hd.Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// bodyValidationMessage reproduces the Spring ServletWebRequest message.
func bodyValidationMessage(r *http.Request) string {
	return "Invalid request: ServletWebRequest: uri=" + r.URL.Path + ";client=" + clientIP(r)
}

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return "127.0.0.1"
	}
	return host
}
```

- [ ] **Step 4: Write entities.go**

`kg-write-twin/internal/api/entities.go`:
```go
package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/model"
	"github.com/grafana/kg-write-twin/internal/store"
)

func (h *handlers) upsertEntity(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	var req model.EntityWriteRequest
	if derr := decodeBody(r, &req); derr != nil {
		writeError(w, r, h.now, *derr)
		return
	}
	if errs := model.ValidateEntity(req); len(errs) > 0 {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), errs))
		return
	}
	in := store.EntityInput{
		Domain: req.Domain, Type: req.Type, Name: req.Name,
		Scope: req.Scope, Properties: req.Properties,
		Origin: store.OriginAPI, TTLSeconds: *req.TTLSeconds,
	}
	e, res := h.store.UpsertEntity(tenant, in)
	switch res {
	case store.OriginConflict:
		writeError(w, r, h.now, errConflict("Target already exists with a non-API origin"))
	case store.Created:
		writeSuccess(w, r, http.StatusCreated, entityBody(e))
	default: // Updated
		writeSuccess(w, r, http.StatusOK, entityBody(e))
	}
}

func (h *handlers) deleteEntity(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	typ := r.PathValue("type")
	if te := model.ValidateType("deleteEntity.type", typ); te != nil {
		writeError(w, r, h.now, errValidation("Invalid request", []model.FieldError{*te}))
		return
	}
	domain := r.URL.Query().Get("domain")
	if de := model.ValidateDomain("domain", domain); de != nil {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), []model.FieldError{*de}))
		return
	}
	scope := parseScopeParams(r, "scope")
	switch h.store.DeleteEntity(tenant, domain, typ, r.PathValue("name"), scope) {
	case store.Deleted:
		w.WriteHeader(http.StatusNoContent)
	case store.DeleteOriginConflict:
		writeError(w, r, h.now, errForbidden("target is not API-origin"))
	default: // NotFound
		writeError(w, r, h.now, errNotFound("entity not found"))
	}
}
```

Add `parseScopeParams` to `codec.go`:
```go
// parseScopeParams extracts deepObject query params like scope[env]=dev into a map.
// prefix is e.g. "scope" or "from.scope".
func parseScopeParams(r *http.Request, prefix string) map[string]string {
	out := map[string]string{}
	for k, vs := range r.URL.Query() {
		if strings.HasPrefix(k, prefix+"[") && strings.HasSuffix(k, "]") && len(vs) > 0 {
			key := k[len(prefix)+1 : len(k)-1]
			out[key] = vs[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
```

- [ ] **Step 5: Write the httptest suite**

`kg-write-twin/internal/api/entities_test.go`:
```go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/store"
)

func newTestServer() *httptest.Server {
	st := store.New(func() int64 { return 1000 })
	return httptest.NewServer(NewServer(st, "/api-server", func() int64 { return 1000 }))
}

func do(t *testing.T, srv *httptest.Server, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

var jsonHdr = map[string]string{"Content-Type": "application/json", "X-Scope-OrgID": "13"}

const entPath = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities"

func TestEntityCreateThenUpdate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	body := `{"domain":"irm","type":"Team","name":"a","scope":{"env":"prod"},"properties":{"k":"v"},"ttlSeconds":10}`
	if code, resp := do(t, srv, "POST", entPath, body, jsonHdr); code != 201 {
		t.Fatalf("create = %d (%s)", code, resp)
	}
	code, resp := do(t, srv, "POST", entPath, body, jsonHdr)
	if code != 200 {
		t.Fatalf("update = %d (%s)", code, resp)
	}
	var got entityResponse
	json.Unmarshal([]byte(resp), &got)
	if got.Properties["_origin"] != "api" || got.Properties["_domain"] != "irm" {
		t.Errorf("enriched props missing: %v", got.Properties)
	}
	if _, ok := got.Properties["_expires_at"]; !ok {
		t.Errorf("_expires_at missing: %v", got.Properties)
	}
}

func TestEntityTTLZeroUsesExpired(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	_, resp := do(t, srv, "POST", entPath, `{"domain":"irm","type":"Team","name":"z","ttlSeconds":0}`, jsonHdr)
	var got entityResponse
	json.Unmarshal([]byte(resp), &got)
	if _, ok := got.Properties["_expired"]; !ok {
		t.Errorf("ttl=0 should set _expired: %v", got.Properties)
	}
	if _, ok := got.Properties["_expires_at"]; ok {
		t.Errorf("ttl=0 should NOT set _expires_at: %v", got.Properties)
	}
}

func TestEntityValidation422(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	code, resp := do(t, srv, "POST", entPath, `{"domain":"kg","type":"Team","name":"a","ttlSeconds":10}`, jsonHdr)
	if code != 422 {
		t.Fatalf("code = %d (%s)", code, resp)
	}
	if !strings.Contains(resp, "ApiValidationError") || !strings.Contains(resp, "reserved 'kg'") {
		t.Errorf("body = %s", resp)
	}
}

func TestEntityTenantErrors(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	// missing header -> 424
	if code, _ := do(t, srv, "POST", entPath, `{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}`,
		map[string]string{"Content-Type": "application/json"}); code != 424 {
		t.Errorf("missing tenant = %d, want 424", code)
	}
	// mismatch -> 403
	if code, _ := do(t, srv, "POST", "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-99/entities",
		`{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}`, jsonHdr); code != 403 {
		t.Errorf("mismatch = %d, want 403", code)
	}
}

func TestEntityConflictViaSeed(t *testing.T) {
	st := store.New(func() int64 { return 1000 })
	st.Seed("13", []store.EntityInput{{Domain: "irm", Type: "Team", Name: "a", Origin: "inference", TTLSeconds: 10}}, nil)
	srv := httptest.NewServer(NewServer(st, "/api-server", func() int64 { return 1000 }))
	defer srv.Close()
	code, _ := do(t, srv, "POST", entPath, `{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}`, jsonHdr)
	if code != 409 {
		t.Errorf("conflict = %d, want 409", code)
	}
}

func TestEntityDelete(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	do(t, srv, "POST", entPath, `{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}`, jsonHdr)
	del := "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities/Team/a?domain=irm"
	if code, _ := do(t, srv, "DELETE", del, "", map[string]string{"X-Scope-OrgID": "13"}); code != 204 {
		t.Errorf("delete = %d, want 204", code)
	}
	if code, _ := do(t, srv, "DELETE", del, "", map[string]string{"X-Scope-OrgID": "13"}); code != 404 {
		t.Errorf("re-delete = %d, want 404", code)
	}
}

func TestEntityDeleteBadTypePath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	del := "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities/1bad/a?domain=irm"
	code, resp := do(t, srv, "DELETE", del, "", map[string]string{"X-Scope-OrgID": "13"})
	if code != 422 || !strings.Contains(resp, "deleteEntity.type") {
		t.Errorf("bad type path = %d (%s)", code, resp)
	}
}

func TestUnsupportedMethodAndPath(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	if code, _ := do(t, srv, "GET", entPath, "", map[string]string{"X-Scope-OrgID": "13"}); code != 405 {
		t.Errorf("GET entities = %d, want 405", code)
	}
	if code, resp := do(t, srv, "GET", "/api-server/nope", "", nil); code != 404 || !strings.Contains(resp, "No static resource") {
		t.Errorf("unknown path = %d (%s)", code, resp)
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run 'Entity|Unsupported' -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add kg-write-twin/internal/api/respond.go kg-write-twin/internal/api/server.go kg-write-twin/internal/api/entities.go kg-write-twin/internal/api/codec.go kg-write-twin/internal/api/entities_test.go
git commit -m "feat(kg-write-twin): router, response builders, entity handlers"
```

---

## Task 12: Relationship handlers

**Files:**
- Create: `kg-write-twin/internal/api/relationships.go`
- Test: `kg-write-twin/internal/api/relationships_test.go`

- [ ] **Step 1: Write relationships.go**

`kg-write-twin/internal/api/relationships.go`:
```go
package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/model"
	"github.com/grafana/kg-write-twin/internal/store"
)

func toStoreRef(r *model.EntityRef) store.Ref {
	if r == nil {
		return store.Ref{}
	}
	return store.Ref{Domain: r.Domain, Type: r.Type, Name: r.Name, Scope: r.Scope}
}

func (h *handlers) upsertRelationship(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	var req model.RelationshipWriteRequest
	if derr := decodeBody(r, &req); derr != nil {
		writeError(w, r, h.now, *derr)
		return
	}
	if errs := model.ValidateRelationship(req); len(errs) > 0 {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), errs))
		return
	}
	in := store.RelationshipInput{
		Domain: req.Domain, Type: req.Type,
		From: toStoreRef(req.From), To: toStoreRef(req.To),
		Properties: req.Properties, Origin: store.OriginAPI, TTLSeconds: *req.TTLSeconds,
	}
	rel, res := h.store.UpsertRelationship(tenant, in)
	switch res {
	case store.FromNotFound:
		writeError(w, r, h.now, errNotFound("from entity not found"))
	case store.ToNotFound:
		writeError(w, r, h.now, errNotFound("to entity not found"))
	case store.RelOriginConflict:
		writeError(w, r, h.now, errConflict("Edge already exists with a non-API origin"))
	default: // RelOK — always 200
		writeSuccess(w, r, http.StatusOK, relationshipBody(rel))
	}
}

func (h *handlers) deleteRelationship(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	typ := r.PathValue("type")
	if te := model.ValidateType("deleteRelationship.type", typ); te != nil {
		writeError(w, r, h.now, errValidation("Invalid request", []model.FieldError{*te}))
		return
	}
	q := r.URL.Query()
	from := store.Ref{Domain: q.Get("from.domain"), Type: q.Get("from.type"), Name: q.Get("from.name"), Scope: parseScopeParams(r, "from.scope")}
	to := store.Ref{Domain: q.Get("to.domain"), Type: q.Get("to.type"), Name: q.Get("to.name"), Scope: parseScopeParams(r, "to.scope")}
	switch h.store.DeleteRelationship(tenant, typ, from, to) {
	case store.Deleted:
		w.WriteHeader(http.StatusNoContent)
	case store.DeleteOriginConflict:
		writeError(w, r, h.now, errForbidden("edge is not API-origin"))
	default: // NotFound
		writeError(w, r, h.now, errNotFound("relationship not found"))
	}
}
```

- [ ] **Step 2: Write the httptest suite**

`kg-write-twin/internal/api/relationships_test.go`:
```go
package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/store"
)

const relPath = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/relationships"

func seedTwoEntities(t *testing.T, srv *httptest.Server) {
	do(t, srv, "POST", entPath, `{"domain":"irm","type":"Team","name":"team-a","ttlSeconds":3600}`, jsonHdr)
	do(t, srv, "POST", entPath, `{"domain":"irm","type":"Service","name":"svc-a","ttlSeconds":3600}`, jsonHdr)
}

func TestRelationshipUpsertAlways200(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	seedTwoEntities(t, srv)
	body := `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"team-a"},"to":{"domain":"irm","type":"Service","name":"svc-a"},"ttlSeconds":3600}`
	if code, resp := do(t, srv, "POST", relPath, body, jsonHdr); code != 200 {
		t.Fatalf("create rel = %d (%s)", code, resp)
	}
	code, resp := do(t, srv, "POST", relPath, body, jsonHdr)
	if code != 200 {
		t.Fatalf("update rel = %d (%s)", code, resp)
	}
	if !strings.Contains(resp, `"scope":{}`) {
		t.Errorf("refs should include empty scope {}: %s", resp)
	}
}

func TestRelationshipMissingEndpoints(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	do(t, srv, "POST", entPath, `{"domain":"irm","type":"Service","name":"svc-a","ttlSeconds":3600}`, jsonHdr)
	body := `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"ghost"},"to":{"domain":"irm","type":"Service","name":"svc-a"},"ttlSeconds":3600}`
	if code, resp := do(t, srv, "POST", relPath, body, jsonHdr); code != 404 || !strings.Contains(resp, "from entity not found") {
		t.Errorf("missing from = %d (%s)", code, resp)
	}
}

func TestRelationshipDelete(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	seedTwoEntities(t, srv)
	do(t, srv, "POST", relPath, `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"team-a"},"to":{"domain":"irm","type":"Service","name":"svc-a"},"ttlSeconds":3600}`, jsonHdr)
	del := relPath + "/owns?from.domain=irm&from.type=Team&from.name=team-a&to.domain=irm&to.type=Service&to.name=svc-a"
	if code, _ := do(t, srv, "DELETE", del, "", map[string]string{"X-Scope-OrgID": "13"}); code != 204 {
		t.Errorf("delete rel = %d, want 204", code)
	}
	if code, resp := do(t, srv, "DELETE", del, "", map[string]string{"X-Scope-OrgID": "13"}); code != 404 || !strings.Contains(resp, "relationship not found") {
		t.Errorf("re-delete = %d (%s)", code, resp)
	}
}

func TestRelationshipConflictViaSeed(t *testing.T) {
	st := store.New(func() int64 { return 1000 })
	st.Seed("13",
		[]store.EntityInput{
			{Domain: "irm", Type: "Team", Name: "team-a", Origin: store.OriginAPI, TTLSeconds: 3600},
			{Domain: "irm", Type: "Service", Name: "svc-a", Origin: store.OriginAPI, TTLSeconds: 3600},
		},
		[]store.RelationshipInput{{Domain: "irm", Type: "owns",
			From: store.Ref{Domain: "irm", Type: "Team", Name: "team-a"},
			To:   store.Ref{Domain: "irm", Type: "Service", Name: "svc-a"},
			Origin: "inference", TTLSeconds: 3600}})
	srv := httptest.NewServer(NewServer(st, "/api-server", func() int64 { return 1000 }))
	defer srv.Close()
	body := `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"team-a"},"to":{"domain":"irm","type":"Service","name":"svc-a"},"ttlSeconds":3600}`
	if code, _ := do(t, srv, "POST", relPath, body, jsonHdr); code != 409 {
		t.Errorf("rel conflict = %d, want 409", code)
	}
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run Relationship -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add kg-write-twin/internal/api/relationships.go kg-write-twin/internal/api/relationships_test.go
git commit -m "feat(kg-write-twin): relationship handlers (200-always, 404 endpoints, 409 seed)"
```

---

## Task 13: Control (/_control) handlers

**Files:**
- Create: `kg-write-twin/internal/api/control.go`
- Test: `kg-write-twin/internal/api/control_test.go`

- [ ] **Step 1: Write control.go**

`kg-write-twin/internal/api/control.go`:
```go
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/grafana/kg-write-twin/internal/store"
)

// seedRequest is the /_control/seed body.
type seedRequest struct {
	Entities []struct {
		Domain     string            `json:"domain"`
		Type       string            `json:"type"`
		Name       string            `json:"name"`
		Scope      map[string]string `json:"scope"`
		Properties map[string]string `json:"properties"`
		Origin     string            `json:"origin"`
		TTLSeconds int64             `json:"ttlSeconds"`
	} `json:"entities"`
	Relationships []struct {
		Domain     string            `json:"domain"`
		Type       string            `json:"type"`
		From       store.Ref         `json:"from"`
		To         store.Ref         `json:"to"`
		Properties map[string]string `json:"properties"`
		Origin     string            `json:"origin"`
		TTLSeconds int64             `json:"ttlSeconds"`
	} `json:"relationships"`
}

func defaultOrigin(o string) string {
	if o == "" {
		return store.OriginAPI
	}
	return o
}

func (h *handlers) reset(w http.ResponseWriter, r *http.Request) {
	h.store.Reset(r.URL.Query().Get("tenant"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) state(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	includeExpired := q.Get("includeExpired") != "false" // default true
	res := h.store.Query(store.Filter{
		Tenant: q.Get("tenant"), Kind: q.Get("kind"), Domain: q.Get("domain"),
		Type: q.Get("type"), Name: q.Get("name"), Origin: q.Get("origin"),
		IncludeExpired: includeExpired,
	})
	writeJSON(w, http.StatusOK, res)
}

func (h *handlers) seed(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		tenant = r.Header.Get("X-Scope-OrgID")
	}
	data, _ := io.ReadAll(r.Body)
	var req seedRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ents := make([]store.EntityInput, 0, len(req.Entities))
	for _, e := range req.Entities {
		ents = append(ents, store.EntityInput{
			Domain: e.Domain, Type: e.Type, Name: e.Name, Scope: e.Scope, Properties: e.Properties,
			Origin: defaultOrigin(e.Origin), TTLSeconds: e.TTLSeconds,
		})
	}
	rels := make([]store.RelationshipInput, 0, len(req.Relationships))
	for _, rl := range req.Relationships {
		rels = append(rels, store.RelationshipInput{
			Domain: rl.Domain, Type: rl.Type, From: rl.From, To: rl.To, Properties: rl.Properties,
			Origin: defaultOrigin(rl.Origin), TTLSeconds: rl.TTLSeconds,
		})
	}
	h.store.Seed(tenant, ents, rels)
	writeJSON(w, http.StatusOK, map[string]any{"seededEntities": len(ents), "seededRelationships": len(rels)})
}

func (h *handlers) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
```

- [ ] **Step 2: Write the httptest suite**

`kg-write-twin/internal/api/control_test.go`:
```go
package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/store"
)

func TestControlSeedStateReset(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	// seed a non-api entity
	seedBody := `{"entities":[{"domain":"irm","type":"Team","name":"a","origin":"inference","ttlSeconds":10}]}`
	if code, resp := do(t, srv, "POST", "/api-server/_control/seed?tenant=13", seedBody, map[string]string{"Content-Type": "application/json"}); code != 200 {
		t.Fatalf("seed = %d (%s)", code, resp)
	}

	// state shows it
	code, resp := do(t, srv, "GET", "/api-server/_control/state?tenant=13", "", nil)
	if code != 200 {
		t.Fatalf("state = %d", code)
	}
	var qr store.QueryResult
	json.Unmarshal([]byte(resp), &qr)
	if len(qr.Entities) != 1 || qr.Entities[0].Origin != "inference" {
		t.Fatalf("state entities = %+v", qr.Entities)
	}

	// origin filter
	_, resp2 := do(t, srv, "GET", "/api-server/_control/state?tenant=13&origin=api", "", nil)
	var qr2 store.QueryResult
	json.Unmarshal([]byte(resp2), &qr2)
	if len(qr2.Entities) != 0 {
		t.Errorf("origin=api filter should exclude seeded inference entity: %+v", qr2.Entities)
	}

	// reset
	do(t, srv, "POST", "/api-server/_control/reset?tenant=13", "", nil)
	_, resp3 := do(t, srv, "GET", "/api-server/_control/state?tenant=13", "", nil)
	if !strings.Contains(resp3, `"entities":[]`) {
		t.Errorf("after reset = %s", resp3)
	}
}

func TestControlHealthz(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	if code, resp := do(t, srv, "GET", "/api-server/_control/healthz", "", nil); code != 200 || !strings.Contains(resp, "ok") {
		t.Errorf("healthz = %d (%s)", code, resp)
	}
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd kg-write-twin && go test ./internal/api/ -run Control -v`
Expected: PASS

- [ ] **Step 4: Run the whole suite**

Run: `cd kg-write-twin && go test ./... -v`
Expected: PASS (model, store, api).

- [ ] **Step 5: Commit**

```bash
git add kg-write-twin/internal/api/control.go kg-write-twin/internal/api/control_test.go
git commit -m "feat(kg-write-twin): /_control reset/state/seed/healthz handlers"
```

---

## Task 14: Server entrypoint (main)

**Files:**
- Create: `kg-write-twin/cmd/kg-write-twin/main.go`

- [ ] **Step 1: Write main.go**

`kg-write-twin/cmd/kg-write-twin/main.go`:
```go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grafana/kg-write-twin/internal/api"
	"github.com/grafana/kg-write-twin/internal/store"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func normalizeAddr(a string) string {
	if a == "" {
		return ":8030"
	}
	if !strings.Contains(a, ":") {
		return ":" + a // bare port
	}
	return a
}

func main() {
	addr := flag.String("addr", normalizeAddr(envOr("PORT", ":8030")), "listen address")
	basePath := flag.String("base-path", envOr("BASE_PATH", "/api-server"), "URL base path")
	seedFile := flag.String("seed-file", os.Getenv("SEED_FILE"), "optional JSON seed file")
	flag.Parse()

	now := func() int64 { return time.Now().UnixMilli() }
	st := store.New(now)

	if *seedFile != "" {
		if err := loadSeed(st, *seedFile); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seeded from %s", *seedFile)
	}

	srv := api.NewServer(st, *basePath, now)
	log.Printf("kg-write-twin listening on %s (base path %s)", *addr, *basePath)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		log.Fatal(err)
	}
}

// loadSeed reads a JSON file matching the /_control/seed body, keyed by tenant.
func loadSeed(st *store.Store, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var byTenant map[string]struct {
		Entities      []store.EntityInput       `json:"entities"`
		Relationships []store.RelationshipInput `json:"relationships"`
	}
	if err := json.Unmarshal(data, &byTenant); err != nil {
		return err
	}
	for tenant, s := range byTenant {
		for i := range s.Entities {
			if s.Entities[i].Origin == "" {
				s.Entities[i].Origin = store.OriginAPI
			}
		}
		for i := range s.Relationships {
			if s.Relationships[i].Origin == "" {
				s.Relationships[i].Origin = store.OriginAPI
			}
		}
		st.Seed(tenant, s.Entities, s.Relationships)
	}
	return nil
}
```

> Note: `store.EntityInput`/`RelationshipInput` have unexported... no — their fields are exported, but the struct tags are absent. Add JSON tags so the seed file decodes. See next step.

- [ ] **Step 2: Add JSON tags to store input types**

Edit `kg-write-twin/internal/store/store.go` — replace the `EntityInput` and `RelationshipInput` definitions with tagged versions:
```go
// EntityInput is the write payload for an entity (origin + ttl explicit).
type EntityInput struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin,omitempty"`
	TTLSeconds int64             `json:"ttlSeconds"`
}

// RelationshipInput is the write payload for a relationship.
type RelationshipInput struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	From       Ref               `json:"from"`
	To         Ref               `json:"to"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin,omitempty"`
	TTLSeconds int64             `json:"ttlSeconds"`
}
```

- [ ] **Step 3: Build and smoke-test**

Run:
```bash
cd kg-write-twin && go build ./... && go vet ./...
go run ./cmd/kg-write-twin --addr :18030 &
sleep 1
curl -sS -o /dev/null -w '%{http_code}\n' -X POST 'http://localhost:18030/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities' -H 'Content-Type: application/json' -H 'X-Scope-OrgID: 13' --data '{"domain":"irm","type":"Team","name":"a","ttlSeconds":10}'
curl -sS 'http://localhost:18030/api-server/_control/state?tenant=13'
kill %1
```
Expected: `201` then a JSON state dump containing the entity.

- [ ] **Step 4: Commit**

```bash
git add kg-write-twin/cmd/kg-write-twin/main.go kg-write-twin/internal/store/store.go
git commit -m "feat(kg-write-twin): server entrypoint with flags/env + seed-file"
```

---

## Task 15: Conformance harness (scenarios, normalize, goldens, record tool)

**Files:**
- Create: `kg-write-twin/internal/conformance/scenarios.go`
- Create: `kg-write-twin/internal/conformance/normalize.go`
- Create: `kg-write-twin/internal/conformance/normalize_test.go`
- Create: `kg-write-twin/internal/conformance/conformance_test.go`
- Create: `kg-write-twin/cmd/kg-twin-record/main.go`
- Create: `kg-write-twin/internal/conformance/testdata/golden/` (generated)

- [ ] **Step 1: Write scenarios.go**

`kg-write-twin/internal/conformance/scenarios.go`:
```go
package conformance

// Request is a single HTTP call (path is relative to the base URL, and must
// already include the base path, e.g. "/api-server/...").
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}

// Scenario is a named sequence: optional setup calls, then the asserted request.
type Scenario struct {
	Name    string
	Setup   []Request
	Request Request
}

var jsonHdr = map[string]string{"Content-Type": "application/json", "X-Scope-OrgID": "13", "Accept": "application/json"}
var tenantHdr = map[string]string{"X-Scope-OrgID": "13"}

const (
	entities = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/entities"
	rels     = "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-13/relationships"
)

// Scenarios is the single catalog reused by the twin tests, the conformance
// test, and the record tool. Each captured behavior in the design spec appears
// here. Every scenario resets its tenant first so runs are independent.
func Scenarios() []Scenario {
	reset := Request{Method: "POST", Path: "/api-server/_control/reset?tenant=13"}
	// NOTE: reset only affects the twin. Against a live server the reset path
	// 404s harmlessly (it is a twin-only endpoint); scenarios are written to be
	// order-independent by using unique names per scenario.
	return []Scenario{
		{Name: "entity_create", Setup: []Request{reset},
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"c1","scope":{"env":"prod"},"properties":{"k":"v"},"ttlSeconds":10}`}},
		{Name: "entity_update", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"u1","ttlSeconds":10}`}},
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"u1","ttlSeconds":10}`}},
		{Name: "entity_ttl_zero",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"z1","ttlSeconds":0}`}},
		{Name: "entity_domain_kg_422",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"kg","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_missing_ttl_422",
			Request: Request{Method: "POST", Path: entities, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x"}`}},
		{Name: "entity_missing_tenant_424",
			Request: Request{Method: "POST", Path: entities, Headers: map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_ns_mismatch_403",
			Request: Request{Method: "POST", Path: "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/stacks-99/entities", Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_bad_namespace_400",
			Request: Request{Method: "POST", Path: "/api-server/apis/kg.grafana.com/v1alpha1/namespaces/foo/entities", Headers: jsonHdr,
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_bad_content_type_415",
			Request: Request{Method: "POST", Path: entities, Headers: map[string]string{"Content-Type": "text/plain", "X-Scope-OrgID": "13", "Accept": "application/json"},
				Body: `{"domain":"irm","type":"Team","name":"x","ttlSeconds":10}`}},
		{Name: "entity_get_405",
			Request: Request{Method: "GET", Path: entities, Headers: tenantHdr}},
		{Name: "relationship_create_200", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"r-from","ttlSeconds":3600}`},
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Service","name":"r-to","ttlSeconds":3600}`}},
			Request: Request{Method: "POST", Path: rels, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"r-from"},"to":{"domain":"irm","type":"Service","name":"r-to"},"ttlSeconds":3600}`}},
		{Name: "relationship_missing_to_404", Setup: []Request{
			{Method: "POST", Path: entities, Headers: jsonHdr, Body: `{"domain":"irm","type":"Team","name":"r2-from","ttlSeconds":3600}`}},
			Request: Request{Method: "POST", Path: rels, Headers: jsonHdr,
				Body: `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"r2-from"},"to":{"domain":"irm","type":"Service","name":"ghost"},"ttlSeconds":3600}`}},
	}
}
```

- [ ] **Step 2: Write normalize.go**

`kg-write-twin/internal/conformance/normalize.go`:
```go
package conformance

import (
	"encoding/json"
	"sort"
	"strings"
)

// Response is a captured HTTP response.
type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Normalized is the comparable form of a response with volatile fields removed.
type Normalized struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// Normalize strips/canonicalizes volatile fields so only meaningful differences
// between the twin and the real API surface.
func Normalize(resp Response) Normalized {
	n := Normalized{Status: resp.Status}
	trimmed := strings.TrimSpace(resp.Body)
	if trimmed == "" {
		n.Body = json.RawMessage(`null`)
		return n
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		// Non-JSON body: keep as a JSON string.
		b, _ := json.Marshal(trimmed)
		n.Body = b
		return n
	}
	v = scrub(v)
	b, _ := json.Marshal(v) // encoding/json sorts map keys -> canonical
	n.Body = b
	return n
}

// scrub recursively removes/rewrites volatile fields.
func scrub(v any) any {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "requestId")
		delete(t, "timestamp")
		for k, val := range t {
			switch k {
			case "_expires_at", "_expired", "expiresAt", "expired", "createdAt":
				t[k] = 0 // absolute time -> sentinel
			case "message":
				if s, ok := val.(string); ok {
					t[k] = scrubMessage(s)
				}
			default:
				t[k] = scrub(val)
			}
		}
		if se, ok := t["subErrors"].([]any); ok {
			sort.Slice(se, func(i, j int) bool {
				return fieldOf(se[i]) < fieldOf(se[j])
			})
		}
		return t
	case []any:
		for i := range t {
			t[i] = scrub(t[i])
		}
		return t
	default:
		return v
	}
}

func fieldOf(v any) string {
	if m, ok := v.(map[string]any); ok {
		if f, ok := m["field"].(string); ok {
			return f
		}
	}
	return ""
}

func scrubMessage(s string) string {
	if strings.HasPrefix(s, "Invalid request: ServletWebRequest:") {
		return "Invalid request: ServletWebRequest"
	}
	if strings.HasPrefix(s, "JSON parse error:") {
		return "JSON parse error"
	}
	if strings.HasPrefix(s, "Failed initializing tenantId=") {
		return "Failed initializing tenantId"
	}
	return s
}
```

- [ ] **Step 3: Write normalize_test.go**

`kg-write-twin/internal/conformance/normalize_test.go`:
```go
package conformance

import (
	"strings"
	"testing"
)

func TestNormalizeStripsVolatile(t *testing.T) {
	a := Normalize(Response{Status: 422, Body: `{"status":"X","requestId":"  abc","timestamp":111,"message":"Invalid request: ServletWebRequest: uri=/a;client=127.0.0.1","subErrors":[{"field":"b"},{"field":"a"}]}`})
	b := Normalize(Response{Status: 422, Body: `{"status":"X","requestId":"def","timestamp":999,"message":"Invalid request: ServletWebRequest: uri=/other;client=10.0.0.1","subErrors":[{"field":"a"},{"field":"b"}]}`})
	if string(a.Body) != string(b.Body) {
		t.Errorf("normalized bodies differ:\n a=%s\n b=%s", a.Body, b.Body)
	}
}

func TestNormalizeExpiryScrubbed(t *testing.T) {
	a := Normalize(Response{Status: 201, Body: `{"properties":{"_expires_at":111}}`})
	if !strings.Contains(string(a.Body), `"_expires_at":0`) {
		t.Errorf("expiry not scrubbed: %s", a.Body)
	}
}

func TestNormalizeEmptyBody(t *testing.T) {
	if got := string(Normalize(Response{Status: 204, Body: ""}).Body); got != "null" {
		t.Errorf("empty body = %s, want null", got)
	}
}
```

- [ ] **Step 4: Run normalize tests**

Run: `cd kg-write-twin && go test ./internal/conformance/ -run Normalize -v`
Expected: PASS

- [ ] **Step 5: Write the record tool**

`kg-write-twin/cmd/kg-twin-record/main.go`:
```go
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grafana/kg-write-twin/internal/conformance"
)

// Records every scenario against KG_TWIN_REAL_URL and writes normalized goldens.
func main() {
	base := os.Getenv("KG_TWIN_REAL_URL")
	if base == "" {
		log.Fatal("set KG_TWIN_REAL_URL (e.g. http://localhost:8030)")
	}
	base = strings.TrimSuffix(base, "/")
	outDir := "internal/conformance/testdata/golden"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, sc := range conformance.Scenarios() {
		for _, s := range sc.Setup {
			mustCall(client, base, s)
		}
		resp := mustCall(client, base, sc.Request)
		n := conformance.Normalize(resp)
		b, _ := json.MarshalIndent(n, "", "  ")
		path := filepath.Join(outDir, sc.Name+".json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("recorded %s -> %s (status %d)", sc.Name, path, resp.Status)
	}
}

func mustCall(c *http.Client, base string, r conformance.Request) conformance.Response {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequest(r.Method, base+r.Path, body)
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", r.Method, r.Path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return conformance.Response{Status: resp.StatusCode, Body: string(b)}
}
```

- [ ] **Step 6: Write conformance_test.go (twin-vs-golden + optional live)**

`kg-write-twin/internal/conformance/conformance_test.go`:
```go
package conformance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/kg-write-twin/internal/api"
	"github.com/grafana/kg-write-twin/internal/store"
)

// startTwin returns a fresh in-process twin.
func startTwin() *httptest.Server {
	st := store.New(func() int64 { return 1000 })
	return httptest.NewServer(api.NewServer(st, "/api-server", func() int64 { return 1000 }))
}

func call(t *testing.T, base string, r Request) Response {
	t.Helper()
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequest(r.Method, base+r.Path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return Response{Status: resp.StatusCode, Body: string(b)}
}

func runScenario(t *testing.T, base string, sc Scenario) Normalized {
	for _, s := range sc.Setup {
		call(t, base, s)
	}
	return Normalize(call(t, base, sc.Request))
}

// TestTwinMatchesGoldens is hermetic: it compares the in-process twin against
// committed goldens. Populate goldens with `make record KG_TWIN_REAL_URL=...`.
func TestTwinMatchesGoldens(t *testing.T) {
	twin := startTwin()
	defer twin.Close()
	for _, sc := range Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			goldenPath := filepath.Join("testdata", "golden", sc.Name+".json")
			gb, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Skipf("no golden (%s); run `make record`", goldenPath)
			}
			var golden Normalized
			if err := json.Unmarshal(gb, &golden); err != nil {
				t.Fatalf("golden parse: %v", err)
			}
			got := runScenario(t, twin.URL, sc)
			assertEqual(t, sc.Name, golden, got)
		})
	}
}

// TestTwinMatchesLive runs only when KG_TWIN_REAL_URL is set: it diffs the twin
// against the live instance directly.
func TestTwinMatchesLive(t *testing.T) {
	real := os.Getenv("KG_TWIN_REAL_URL")
	if real == "" {
		t.Skip("set KG_TWIN_REAL_URL to run live differential conformance")
	}
	real = strings.TrimSuffix(real, "/")
	twin := startTwin()
	defer twin.Close()
	for _, sc := range Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			want := runScenario(t, real, sc)
			got := runScenario(t, twin.URL, sc)
			assertEqual(t, sc.Name, want, got)
		})
	}
}

func assertEqual(t *testing.T, name string, want, got Normalized) {
	t.Helper()
	if want.Status != got.Status {
		t.Errorf("%s: status got %d want %d", name, got.Status, want.Status)
	}
	if string(want.Body) != string(got.Body) {
		t.Errorf("%s: body mismatch\n want: %s\n got:  %s", name, want.Body, got.Body)
	}
}
```

- [ ] **Step 7: Generate goldens from the live instance and run conformance**

Run (requires the real api-server on :8030):
```bash
cd kg-write-twin && KG_TWIN_REAL_URL=http://localhost:8030 go run ./cmd/kg-twin-record
go test ./internal/conformance/ -run TestTwinMatchesGoldens -v
```
Expected: goldens written; hermetic test PASS. If any scenario fails, the diff shows a real twin↔API divergence — fix the twin (or record a corrected golden if the real API is the source of truth) before continuing.

- [ ] **Step 8: Run the full differential suite against live (optional but recommended)**

Run:
```bash
cd kg-write-twin && KG_TWIN_REAL_URL=http://localhost:8030 go test ./internal/conformance/ -v
```
Expected: both `TestTwinMatchesGoldens` and `TestTwinMatchesLive` PASS.

- [ ] **Step 9: Commit**

```bash
git add kg-write-twin/internal/conformance/ kg-write-twin/cmd/kg-twin-record/
git commit -m "feat(kg-write-twin): conformance harness — scenarios, normalize, goldens, record tool"
```

---

## Task 16: Packaging — Makefile, Dockerfile, compose, Tiltfile

**Files:**
- Create: `kg-write-twin/Makefile`
- Create: `kg-write-twin/Dockerfile`
- Create: `kg-write-twin/compose.yaml`
- Create: `kg-write-twin/Tiltfile`

- [ ] **Step 1: Write the Makefile**

`kg-write-twin/Makefile`:
```makefile
BINARY := bin/kg-write-twin
IMAGE  := kg-write-twin:dev
UPSTREAM := repos/grafana/asserts-adi/contents/inference-engine/api-server/openapi/kg-write.yaml

.PHONY: build test run docker record check-spec-drift tidy

build:
	go build -o $(BINARY) ./cmd/kg-write-twin

test:
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
```

- [ ] **Step 2: Verify make targets**

Run:
```bash
cd kg-write-twin && make build && make test && make check-spec-drift
```
Expected: build succeeds, tests PASS, `openapi in sync`.

- [ ] **Step 3: Write the Dockerfile**

`kg-write-twin/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/kg-write-twin ./cmd/kg-write-twin

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/kg-write-twin /kg-write-twin
EXPOSE 8030
ENTRYPOINT ["/kg-write-twin", "--addr", ":8030"]
```

- [ ] **Step 4: Build the image**

Run: `cd kg-write-twin && docker build -t kg-write-twin:dev .`
Expected: image builds successfully.

- [ ] **Step 5: Write compose.yaml**

`kg-write-twin/compose.yaml`:
```yaml
services:
  kg-write-twin:
    build:
      context: .
      dockerfile: ./Dockerfile
    image: kg-write-twin:dev
    ports:
      - "8030:8030"
    environment:
      BASE_PATH: /api-server
    healthcheck:
      test: ["CMD", "/kg-write-twin", "--help"]
      interval: 10s
      timeout: 3s
      retries: 3
```

- [ ] **Step 6: Write the Tiltfile (cc composable)**

`kg-write-twin/Tiltfile`:
```python
# kg-write-twin — cc composable
#
# Usable standalone (tilt up) or loaded by an orchestrator Tiltfile via
# cc.use('kg-write-twin').

_current_dir = os.path.dirname(__file__)

def cc_export(cc):
    """Return the composable for kg-write-twin."""
    return cc.create(
        'kg-write-twin',
        _current_dir + '/compose.yaml',
        labels=['app'],
        symbols={'cc_setup': cc_setup},
    )

def cc_setup(composable_ctx):
    """Host-side build with file watching for hot reload."""
    twin_dir = os.path.dirname(composable_ctx.compose_path)
    local_resource(
        'kg-write-twin-build',
        cmd='cd "' + twin_dir + '" && make build',
        deps=[
            twin_dir + '/cmd',
            twin_dir + '/internal',
            twin_dir + '/go.mod',
        ],
        labels=['KgWriteTwin.Build'],
    )

# Orchestrator mode — standalone `tilt up -f Tiltfile`.
if __file__ == config.main_path:
    docker_compose(_current_dir + '/compose.yaml')
```

- [ ] **Step 7: Commit**

```bash
git add kg-write-twin/Makefile kg-write-twin/Dockerfile kg-write-twin/compose.yaml kg-write-twin/Tiltfile
git commit -m "feat(kg-write-twin): packaging — Makefile, Dockerfile, compose, cc Tiltfile"
```

---

## Task 17: README + divergence log + final verification

**Files:**
- Modify: `kg-write-twin/README.md`

- [ ] **Step 1: Write the full README**

Replace `kg-write-twin/README.md` with:
```markdown
# kg-write-twin

An in-memory behavioral clone ("digital twin") of the Asserts **Knowledge Graph Write API**
(`grafana/asserts-adi:inference-engine/api-server/openapi/kg-write.yaml`), for local
development and integration testing. Behavior was captured by probing a live dev instance.

Design: `../docs/superpowers/specs/2026-07-02-kg-write-twin-design.md`.

## Run

    make run                      # :8030, base path /api-server
    make build && ./bin/kg-write-twin --addr :8030 --base-path /api-server
    docker build -t kg-write-twin:dev . && docker run -p 8030:8030 kg-write-twin:dev

Flags/env: `--addr`/`PORT` (default `:8030`), `--base-path`/`BASE_PATH` (default `/api-server`),
`--seed-file`/`SEED_FILE` (optional JSON fixtures keyed by tenant).

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
```

- [ ] **Step 2: Final full verification**

Run:
```bash
cd kg-write-twin && go build ./... && go vet ./... && go test ./...
```
Expected: build clean, vet clean, all tests PASS.

- [ ] **Step 3: Commit**

```bash
git add kg-write-twin/README.md
git commit -m "docs(kg-write-twin): README with usage + divergence log"
```

---

## Self-Review Notes (for the implementer)

- **Spec coverage:** every design section maps to a task — model/validation (T2–T3), store + origin + TTL + query (T4–T7), tenant resolution (T10), spec handlers + content negotiation + errors (T8–T13), entrypoint (T14), continuous validation (T15), cc packaging (T16), divergence log (T17).
- **Golden dependency:** Task 15 requires the live api-server on `:8030` to record goldens. If it is unavailable, `TestTwinMatchesGoldens` self-skips per scenario (no golden file) — but you MUST record goldens before declaring the twin validated. Do not fabricate goldens from the twin's own output.
- **Type consistency:** result enums (`Created/Updated/OriginConflict`, `Deleted/NotFound/DeleteOriginConflict`, `RelOK/FromNotFound/ToNotFound/RelOriginConflict`), `store.EntityInput`/`RelationshipInput`, and `api` response structs are defined once and reused verbatim across tasks.
- **Known best-effort items** are documented in the divergence log and handled by `normalize.go` so conformance stays green.
```
