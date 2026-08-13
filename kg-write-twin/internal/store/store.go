package store

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// OriginAPI is the origin assigned to all writes made through the spec API.
const OriginAPI = "api"

// Ref is an entity endpoint of a relationship.
type Ref struct {
	Domain string            `json:"domain"`
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Scope  map[string]string `json:"scope"`
}

// Entity is a stored entity (also the /_control/state representation).
type Entity struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin"`
	ExpiresAt  int64             `json:"expiresAt,omitempty"`
	Expired    int64             `json:"expired,omitempty"`
	TTLZero    bool              `json:"ttlZero"`
	CreatedAt  int64             `json:"createdAt"`
}

// Relationship is a stored relationship (also the /_control/state representation).
type Relationship struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	From       Ref               `json:"from"`
	To         Ref               `json:"to"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin"`
	ExpiresAt  int64             `json:"expiresAt,omitempty"`
	Expired    int64             `json:"expired,omitempty"`
	TTLZero    bool              `json:"ttlZero"`
	CreatedAt  int64             `json:"createdAt"`
}

// EntityInput is the write payload for an entity (origin + ttl explicit).
type EntityInput struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin,omitempty"`
	TTLSeconds int64             `json:"ttlSeconds"`
}

// RelationshipInput is the write payload for a relationship.
type RelationshipInput struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	From       Ref               `json:"from"`
	To         Ref               `json:"to"`
	Properties map[string]string `json:"properties,omitempty"`
	Origin     string            `json:"origin,omitempty"`
	TTLSeconds int64             `json:"ttlSeconds"`
}

type tenantData struct {
	entities      map[string]*Entity
	relationships map[string]*Relationship
}

// Store is an in-memory, tenant-partitioned graph store.
type Store struct {
	mu      sync.RWMutex
	now     func() int64 // epoch millis
	tenants map[string]*tenantData
}

// New creates a Store. now must return epoch milliseconds.
func New(now func() int64) *Store {
	return &Store{now: now, tenants: map[string]*tenantData{}}
}

func (s *Store) tenant(id string) *tenantData {
	td := s.tenants[id]
	if td == nil {
		td = &tenantData{entities: map[string]*Entity{}, relationships: map[string]*Relationship{}}
		s.tenants[id] = td
	}
	return td
}

func computeExpiry(nowMS, ttl int64) (expiresAt, expired int64, zero bool) {
	switch {
	case ttl == 0:
		return 0, nowMS, true
	case ttl < 0:
		return math.MaxInt64, 0, false
	default:
		return nowMS + ttl*1000, 0, false
	}
}

func scopeKey(scope map[string]string) string {
	if len(scope) == 0 {
		return ""
	}
	keys := make([]string, 0, len(scope))
	for k := range scope {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s;", k, scope[k])
	}
	return b.String()
}

func entityKey(domain, typ, name string, scope map[string]string) string {
	return domain + "\x00" + typ + "\x00" + name + "\x00" + scopeKey(scope)
}

func relKey(typ string, from, to Ref) string {
	return typ + "\x00" +
		entityKey(from.Domain, from.Type, from.Name, from.Scope) + "\x00->\x00" +
		entityKey(to.Domain, to.Type, to.Name, to.Scope)
}

func (e *Entity) isExpired(nowMS int64) bool {
	if e.TTLZero {
		return true
	}
	return e.ExpiresAt != math.MaxInt64 && nowMS >= e.ExpiresAt
}

func (r *Relationship) isExpired(nowMS int64) bool {
	if r.TTLZero {
		return true
	}
	return r.ExpiresAt != math.MaxInt64 && nowMS >= r.ExpiresAt
}
