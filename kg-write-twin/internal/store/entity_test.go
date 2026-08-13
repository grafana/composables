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
