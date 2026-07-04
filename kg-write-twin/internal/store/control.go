package store

import "sort"

// Filter selects objects for Query. Empty string fields match anything.
type Filter struct {
	Tenant         string
	Kind           string // "", "entity", "relationship"
	Domain         string
	Type           string
	Name           string
	Origin         string
	IncludeExpired bool
}

// QueryResult is the /_control/state payload.
type QueryResult struct {
	Entities      []Entity       `json:"entities"`
	Relationships []Relationship `json:"relationships"`
}

// Reset clears one tenant, or all tenants when tenant == "".
func (s *Store) Reset(tenant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenant == "" {
		s.tenants = map[string]*tenantData{}
		return
	}
	delete(s.tenants, tenant)
}

// Seed inserts entities/relationships with explicit origin (used to create
// non-API-origin objects for 409/403 testing, and to preload fixtures).
func (s *Store) Seed(tenant string, ents []EntityInput, rels []RelationshipInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td := s.tenant(tenant)
	now := s.now()
	for _, in := range ents {
		ea, ex, zero := computeExpiry(now, in.TTLSeconds)
		td.entities[entityKey(in.Domain, in.Type, in.Name, in.Scope)] = &Entity{
			Domain: in.Domain, Type: in.Type, Name: in.Name, Scope: in.Scope, Properties: in.Properties,
			Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
		}
	}
	for _, in := range rels {
		ea, ex, zero := computeExpiry(now, in.TTLSeconds)
		from, to := normalizeRefScope(in.From), normalizeRefScope(in.To)
		td.relationships[relKey(in.Type, from, to)] = &Relationship{
			Domain: in.Domain, Type: in.Type, From: from, To: to, Properties: in.Properties,
			Origin: in.Origin, ExpiresAt: ea, Expired: ex, TTLZero: zero, CreatedAt: now,
		}
	}
}

// Query returns matching objects, sorted for deterministic output.
func (s *Store) Query(f Filter) QueryResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	res := QueryResult{Entities: []Entity{}, Relationships: []Relationship{}}

	forTenant := func(td *tenantData) {
		if f.Kind != "relationship" {
			for _, e := range td.entities {
				if !f.IncludeExpired && e.isExpired(now) {
					continue
				}
				if (f.Domain == "" || e.Domain == f.Domain) &&
					(f.Type == "" || e.Type == f.Type) &&
					(f.Name == "" || e.Name == f.Name) &&
					(f.Origin == "" || e.Origin == f.Origin) {
					res.Entities = append(res.Entities, *e)
				}
			}
		}
		if f.Kind != "entity" {
			for _, r := range td.relationships {
				if !f.IncludeExpired && r.isExpired(now) {
					continue
				}
				if (f.Domain == "" || r.Domain == f.Domain) &&
					(f.Type == "" || r.Type == f.Type) &&
					(f.Origin == "" || r.Origin == f.Origin) {
					res.Relationships = append(res.Relationships, *r)
				}
			}
		}
	}

	if f.Tenant != "" {
		if td := s.tenants[f.Tenant]; td != nil {
			forTenant(td)
		}
	} else {
		for _, td := range s.tenants {
			forTenant(td)
		}
	}

	sort.Slice(res.Entities, func(i, j int) bool {
		return res.Entities[i].Type+res.Entities[i].Name < res.Entities[j].Type+res.Entities[j].Name
	})
	sort.Slice(res.Relationships, func(i, j int) bool {
		return res.Relationships[i].Type+res.Relationships[i].From.Name < res.Relationships[j].Type+res.Relationships[j].From.Name
	})
	return res
}
