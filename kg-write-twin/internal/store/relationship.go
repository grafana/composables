package store

// RelUpsertResult is the outcome of a relationship upsert.
type RelUpsertResult int

const (
	RelOK RelUpsertResult = iota // create or update — always 200
	FromNotFound
	ToNotFound
	RelOriginConflict
)

func normalizeRefScope(r Ref) Ref {
	if r.Scope == nil {
		r.Scope = map[string]string{}
	}
	return r
}

// UpsertRelationship creates or updates an edge. Both endpoints must already
// exist (expiry ignored); "from" is checked before "to". A matching non-API
// edge causes RelOriginConflict.
func (s *Store) UpsertRelationship(tenant string, in RelationshipInput) (Relationship, RelUpsertResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	if !s.entityExists(td, in.From) {
		return Relationship{}, FromNotFound
	}
	if !s.entityExists(td, in.To) {
		return Relationship{}, ToNotFound
	}
	from := normalizeRefScope(in.From)
	to := normalizeRefScope(in.To)
	key := relKey(in.Type, from, to)
	if existing, ok := td.relationships[key]; ok && existing.Origin != OriginAPI {
		return *existing, RelOriginConflict
	}
	now := s.now()
	ea, ex, zero := computeExpiry(now, in.TTLSeconds)
	rel := &Relationship{
		Domain: in.Domain, Type: in.Type, From: from, To: to,
		Properties: in.Properties, Origin: in.Origin,
		ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
	}
	td.relationships[key] = rel
	return *rel, RelOK
}

// DeleteRelationship removes an edge by (type, from, to). Non-API edges cause
// DeleteOriginConflict; absent edges cause NotFound.
func (s *Store) DeleteRelationship(tenant, typ string, from, to Ref) DeleteResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := relKey(typ, normalizeRefScope(from), normalizeRefScope(to))
	existing, ok := td.relationships[key]
	if !ok {
		return NotFound
	}
	if existing.Origin != OriginAPI {
		return DeleteOriginConflict
	}
	delete(td.relationships, key)
	return Deleted
}
