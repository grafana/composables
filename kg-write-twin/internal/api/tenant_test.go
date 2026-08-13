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
