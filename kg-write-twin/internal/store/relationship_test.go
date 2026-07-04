package store

import "testing"

func seedEntity(s *Store, name string) {
	s.UpsertEntity("13", EntityInput{Domain: "irm", Type: "Service", Name: name, Origin: OriginAPI, TTLSeconds: 3600})
}

func ref(name string) Ref { return Ref{Domain: "irm", Type: "Service", Name: name} }

func TestUpsertRelationship(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "a")
	seedEntity(s, "b")
	in := RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60}
	if _, r := s.UpsertRelationship("13", in); r != RelOK {
		t.Fatalf("create rel = %v, want RelOK", r)
	}
	if _, r := s.UpsertRelationship("13", in); r != RelOK {
		t.Fatalf("update rel = %v, want RelOK (always 200)", r)
	}
}

func TestUpsertRelationshipMissingEndpoints(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "b")
	// from missing (checked first)
	if _, r := s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("ghost"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60}); r != FromNotFound {
		t.Fatalf("missing from = %v, want FromNotFound", r)
	}
	seedEntity(s, "a")
	if _, r := s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("ghost"), Origin: OriginAPI, TTLSeconds: 60}); r != ToNotFound {
		t.Fatalf("missing to = %v, want ToNotFound", r)
	}
}

func TestDeleteRelationship(t *testing.T) {
	s := New(fixedNow)
	seedEntity(s, "a")
	seedEntity(s, "b")
	s.UpsertRelationship("13", RelationshipInput{Domain: "irm", Type: "uses", From: ref("a"), To: ref("b"), Origin: OriginAPI, TTLSeconds: 60})
	if r := s.DeleteRelationship("13", "uses", ref("a"), ref("b")); r != Deleted {
		t.Fatalf("delete rel = %v, want Deleted", r)
	}
	if r := s.DeleteRelationship("13", "uses", ref("a"), ref("b")); r != NotFound {
		t.Fatalf("re-delete rel = %v, want NotFound", r)
	}
}
