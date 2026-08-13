package store

// UpsertResult is the outcome of an entity/edge upsert.
type UpsertResult int

const (
	Created UpsertResult = iota
	Updated
	OriginConflict
)

// DeleteResult is the outcome of a delete.
type DeleteResult int

const (
	Deleted DeleteResult = iota
	NotFound
	DeleteOriginConflict
)

// UpsertEntity creates or updates an entity. Existing objects with a non-API
// origin cause OriginConflict. Expiry is metadata only and never blocks writes.
func (s *Store) UpsertEntity(tenant string, in EntityInput) (Entity, UpsertResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := entityKey(in.Domain, in.Type, in.Name, in.Scope)
	existing, ok := td.entities[key]
	if ok && existing.Origin != OriginAPI {
		return *existing, OriginConflict
	}
	now := s.now()
	ea, ex, zero := computeExpiry(now, in.TTLSeconds)
	e := &Entity{
		Domain: in.Domain, Type: in.Type, Name: in.Name,
		Scope: in.Scope, Properties: in.Properties,
		Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
	}
	td.entities[key] = e
	if ok {
		return *e, Updated
	}
	return *e, Created
}

// DeleteEntity removes an entity by identity. Non-API-origin targets cause
// DeleteOriginConflict; absent targets cause NotFound.
func (s *Store) DeleteEntity(tenant, domain, typ, name string, scope map[string]string) DeleteResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	key := entityKey(domain, typ, name, scope)
	existing, ok := td.entities[key]
	if !ok {
		return NotFound
	}
	if existing.Origin != OriginAPI {
		return DeleteOriginConflict
	}
	delete(td.entities, key)
	return Deleted
}

// entityExists reports whether an entity identity is present (expiry ignored).
// Caller must hold the lock.
func (s *Store) entityExists(td *tenantData, ref Ref) bool {
	_, ok := td.entities[entityKey(ref.Domain, ref.Type, ref.Name, ref.Scope)]
	return ok
}
