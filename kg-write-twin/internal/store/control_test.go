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
