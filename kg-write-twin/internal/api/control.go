package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/grafana/kg-write-twin/internal/store"
)

// seedRequest is the /_control/seed body.
type seedRequest struct {
	Entities []struct {
		Domain     string            `json:"domain"`
		Type       string            `json:"type"`
		Name       string            `json:"name"`
		Scope      map[string]string `json:"scope"`
		Properties map[string]string `json:"properties"`
		Origin     string            `json:"origin"`
		TTLSeconds int64             `json:"ttlSeconds"`
	} `json:"entities"`
	Relationships []struct {
		Domain     string            `json:"domain"`
		Type       string            `json:"type"`
		From       store.Ref         `json:"from"`
		To         store.Ref         `json:"to"`
		Properties map[string]string `json:"properties"`
		Origin     string            `json:"origin"`
		TTLSeconds int64             `json:"ttlSeconds"`
	} `json:"relationships"`
}

func defaultOrigin(o string) string {
	if o == "" {
		return store.OriginAPI
	}
	return o
}

func (h *handlers) reset(w http.ResponseWriter, r *http.Request) {
	h.store.Reset(r.URL.Query().Get("tenant"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) state(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	includeExpired := q.Get("includeExpired") != "false" // default true
	res := h.store.Query(store.Filter{
		Tenant: q.Get("tenant"), Kind: q.Get("kind"), Domain: q.Get("domain"),
		Type: q.Get("type"), Name: q.Get("name"), Origin: q.Get("origin"),
		IncludeExpired: includeExpired,
	})
	writeJSON(w, http.StatusOK, res)
}

func (h *handlers) seed(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		tenant = r.Header.Get("X-Scope-OrgID")
	}
	data, _ := io.ReadAll(r.Body)
	var req seedRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ents := make([]store.EntityInput, 0, len(req.Entities))
	for _, e := range req.Entities {
		ents = append(ents, store.EntityInput{
			Domain: e.Domain, Type: e.Type, Name: e.Name, Scope: e.Scope, Properties: e.Properties,
			Origin: defaultOrigin(e.Origin), TTLSeconds: e.TTLSeconds,
		})
	}
	rels := make([]store.RelationshipInput, 0, len(req.Relationships))
	for _, rl := range req.Relationships {
		rels = append(rels, store.RelationshipInput{
			Domain: rl.Domain, Type: rl.Type, From: rl.From, To: rl.To, Properties: rl.Properties,
			Origin: defaultOrigin(rl.Origin), TTLSeconds: rl.TTLSeconds,
		})
	}
	h.store.Seed(tenant, ents, rels)
	writeJSON(w, http.StatusOK, map[string]any{"seededEntities": len(ents), "seededRelationships": len(rels)})
}

func (h *handlers) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
