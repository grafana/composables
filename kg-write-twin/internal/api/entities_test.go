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
