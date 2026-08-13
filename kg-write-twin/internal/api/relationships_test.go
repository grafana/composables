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
			From:   store.Ref{Domain: "irm", Type: "Team", Name: "team-a"},
			To:     store.Ref{Domain: "irm", Type: "Service", Name: "svc-a"},
			Origin: "inference", TTLSeconds: 3600}})
	srv := httptest.NewServer(NewServer(st, "/api-server", func() int64 { return 1000 }))
	defer srv.Close()
	body := `{"domain":"irm","type":"owns","from":{"domain":"irm","type":"Team","name":"team-a"},"to":{"domain":"irm","type":"Service","name":"svc-a"},"ttlSeconds":3600}`
	if code, _ := do(t, srv, "POST", relPath, body, jsonHdr); code != 409 {
		t.Errorf("rel conflict = %d, want 409", code)
	}
}
