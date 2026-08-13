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
