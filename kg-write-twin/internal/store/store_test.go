package store

import (
	"math"
	"testing"
)

func TestComputeExpiry(t *testing.T) {
	const now = int64(1000)
	cases := []struct {
		ttl           int64
		wantExpiresAt int64
		wantExpired   int64
		wantZero      bool
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
