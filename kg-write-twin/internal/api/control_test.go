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
