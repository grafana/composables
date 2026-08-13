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
