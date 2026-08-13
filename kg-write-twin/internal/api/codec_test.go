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
