package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/model"
	"github.com/grafana/kg-write-twin/internal/store"
)

func toStoreRef(r *model.EntityRef) store.Ref {
	if r == nil {
		return store.Ref{}
	}
	return store.Ref{Domain: r.Domain, Type: r.Type, Name: r.Name, Scope: r.Scope}
}

func (h *handlers) upsertRelationship(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	var req model.RelationshipWriteRequest
	if derr := decodeBody(r, &req); derr != nil {
		writeError(w, r, h.now, *derr)
		return
	}
	if errs := model.ValidateRelationship(req); len(errs) > 0 {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), errs))
		return
	}
	in := store.RelationshipInput{
		Domain: req.Domain, Type: req.Type,
		From: toStoreRef(req.From), To: toStoreRef(req.To),
		Properties: req.Properties, Origin: store.OriginAPI, TTLSeconds: *req.TTLSeconds,
	}
	rel, res := h.store.UpsertRelationship(tenant, in)
	switch res {
	case store.FromNotFound:
		writeError(w, r, h.now, errNotFound("from entity not found"))
	case store.ToNotFound:
		writeError(w, r, h.now, errNotFound("to entity not found"))
	case store.RelOriginConflict:
		writeError(w, r, h.now, errConflict("Edge already exists with a non-API origin"))
	default: // RelOK — always 200
		writeSuccess(w, r, http.StatusOK, relationshipBody(rel))
	}
}

func (h *handlers) deleteRelationship(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	typ := r.PathValue("type")
	if te := model.ValidateType("deleteRelationship.type", typ); te != nil {
		writeError(w, r, h.now, errValidation("Invalid request", []model.FieldError{*te}))
		return
	}
	q := r.URL.Query()
	from := store.Ref{Domain: q.Get("from.domain"), Type: q.Get("from.type"), Name: q.Get("from.name"), Scope: parseScopeParams(r, "from.scope")}
	to := store.Ref{Domain: q.Get("to.domain"), Type: q.Get("to.type"), Name: q.Get("to.name"), Scope: parseScopeParams(r, "to.scope")}
	switch h.store.DeleteRelationship(tenant, typ, from, to) {
	case store.Deleted:
		w.WriteHeader(http.StatusNoContent)
	case store.DeleteOriginConflict:
		writeError(w, r, h.now, errForbidden("edge is not API-origin"))
	default: // NotFound
		writeError(w, r, h.now, errNotFound("relationship not found"))
	}
}
