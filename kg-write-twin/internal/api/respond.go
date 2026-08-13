package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/store"
)

// enrichedProps builds the response "properties": user props merged with
// computed _domain, _origin, and _expires_at (or _expired when ttl==0).
func enrichedProps(domain, origin string, props map[string]string, expiresAt, expired int64, ttlZero bool) map[string]any {
	out := map[string]any{"_domain": domain}
	for k, v := range props {
		out[k] = v
	}
	out["_origin"] = origin
	if ttlZero {
		out["_expired"] = expired
	} else {
		out["_expires_at"] = expiresAt
	}
	return out
}

type entityResponse struct {
	Domain     string            `json:"domain"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Scope      map[string]string `json:"scope,omitempty"`
	Properties map[string]any    `json:"properties"`
}

func entityBody(e store.Entity) entityResponse {
	return entityResponse{
		Domain: e.Domain, Type: e.Type, Name: e.Name, Scope: e.Scope,
		Properties: enrichedProps(e.Domain, e.Origin, e.Properties, e.ExpiresAt, e.Expired, e.TTLZero),
	}
}

type refResponse struct {
	Domain string            `json:"domain"`
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Scope  map[string]string `json:"scope"` // always present, {} when empty
}

func refBody(r store.Ref) refResponse {
	sc := r.Scope
	if sc == nil {
		sc = map[string]string{}
	}
	return refResponse{Domain: r.Domain, Type: r.Type, Name: r.Name, Scope: sc}
}

type relationshipResponse struct {
	Domain     string         `json:"domain"`
	Type       string         `json:"type"`
	From       refResponse    `json:"from"`
	To         refResponse    `json:"to"`
	Properties map[string]any `json:"properties"`
}

func relationshipBody(r store.Relationship) relationshipResponse {
	return relationshipResponse{
		Domain: r.Domain, Type: r.Type, From: refBody(r.From), To: refBody(r.To),
		Properties: enrichedProps(r.Domain, r.Origin, r.Properties, r.ExpiresAt, r.Expired, r.TTLZero),
	}
}

// writeError renders an apiError to the client (respecting Accept).
func writeError(w http.ResponseWriter, r *http.Request, now func() int64, e apiError) {
	f := responseFormat(r)
	w.Header().Set("Content-Type", contentType(f))
	w.WriteHeader(e.httpCode)
	jb := e.body(now) // JSON bytes
	if f == formatYAML {
		w.Write(encode(formatYAML, rawJSON(jb)))
		return
	}
	w.Write(jb)
}

// writeSuccess renders a success payload (respecting Accept).
func writeSuccess(w http.ResponseWriter, r *http.Request, status int, payload any) {
	f := responseFormat(r)
	w.Header().Set("Content-Type", contentType(f))
	w.WriteHeader(status)
	w.Write(encode(f, payload))
}

// writeJSON renders JSON only (used by the /_control surface).
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(encode(formatJSON, payload))
}
