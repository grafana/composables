package api

import (
	"net/http"

	"github.com/grafana/kg-write-twin/internal/model"
	"github.com/grafana/kg-write-twin/internal/store"
)

func (h *handlers) upsertEntity(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	var req model.EntityWriteRequest
	if derr := decodeBody(r, &req); derr != nil {
		writeError(w, r, h.now, *derr)
		return
	}
	if errs := model.ValidateEntity(req); len(errs) > 0 {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), errs))
		return
	}
	in := store.EntityInput{
		Domain: req.Domain, Type: req.Type, Name: req.Name,
		Scope: req.Scope, Properties: req.Properties,
		Origin: store.OriginAPI, TTLSeconds: *req.TTLSeconds,
	}
	e, res := h.store.UpsertEntity(tenant, in)
	switch res {
	case store.OriginConflict:
		writeError(w, r, h.now, errConflict("Target already exists with a non-API origin"))
	case store.Created:
		writeSuccess(w, r, http.StatusCreated, entityBody(e))
	default: // Updated
		writeSuccess(w, r, http.StatusOK, entityBody(e))
	}
}

func (h *handlers) deleteEntity(w http.ResponseWriter, r *http.Request) {
	tenant, terr := resolveTenant(r, r.PathValue("namespace"))
	if terr != nil {
		writeError(w, r, h.now, *terr)
		return
	}
	typ := r.PathValue("type")
	if te := model.ValidateType("deleteEntity.type", typ); te != nil {
		writeError(w, r, h.now, errValidation("Invalid request", []model.FieldError{*te}))
		return
	}
	domain := r.URL.Query().Get("domain")
	if de := model.ValidateDomain("domain", domain); de != nil {
		writeError(w, r, h.now, errValidation(bodyValidationMessage(r), []model.FieldError{*de}))
		return
	}
	scope := parseScopeParams(r, "scope")
	switch h.store.DeleteEntity(tenant, domain, typ, r.PathValue("name"), scope) {
	case store.Deleted:
		w.WriteHeader(http.StatusNoContent)
	case store.DeleteOriginConflict:
		writeError(w, r, h.now, errForbidden("target is not API-origin"))
	default: // NotFound
		writeError(w, r, h.now, errNotFound("entity not found"))
	}
}
