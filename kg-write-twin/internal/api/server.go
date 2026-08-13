package api

import (
	"net/http"
	"strings"

	"github.com/grafana/kg-write-twin/internal/store"
)

type handlers struct {
	store    *store.Store
	now      func() int64
	basePath string
}

// NewServer builds the HTTP handler. basePath defaults to "/api-server".
func NewServer(st *store.Store, basePath string, now func() int64) http.Handler {
	h := &handlers{store: st, now: now, basePath: basePath}
	mux := http.NewServeMux()
	p := basePath

	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities", h.entities)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/entities/{type}/{name}", h.entityItem)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships", h.relationships)
	mux.HandleFunc(p+"/apis/kg.grafana.com/v1alpha1/namespaces/{namespace}/relationships/{type}", h.relationshipItem)
	mux.HandleFunc(p+"/_control/reset", h.methodGate(http.MethodPost, h.reset))
	mux.HandleFunc(p+"/_control/state", h.methodGate(http.MethodGet, h.state))
	mux.HandleFunc(p+"/_control/seed", h.methodGate(http.MethodPost, h.seed))
	mux.HandleFunc(p+"/_control/healthz", h.methodGate(http.MethodGet, h.healthz))
	mux.HandleFunc("/", h.notFound)

	return securityHeaders(mux)
}

// methodGate returns a handler that enforces a single method, else 405.
func (h *handlers) methodGate(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeError(w, r, h.now, errMethodNotAllowed(r.Method))
			return
		}
		next(w, r)
	}
}

// entities routes POST; other methods -> 405.
func (h *handlers) entities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.upsertEntity(w, r)
}

func (h *handlers) entityItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.deleteEntity(w, r)
}

func (h *handlers) relationships(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.upsertRelationship(w, r)
}

func (h *handlers) relationshipItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, r, h.now, errMethodNotAllowed(r.Method))
		return
	}
	h.deleteRelationship(w, r)
}

// notFound reproduces the real "No static resource <path>." message.
func (h *handlers) notFound(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, h.basePath+"/")
	writeError(w, r, h.now, errNotFound("No static resource "+rel+"."))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("X-XSS-Protection", "0")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		hd.Set("Pragma", "no-cache")
		hd.Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// bodyValidationMessage reproduces the Spring ServletWebRequest message.
func bodyValidationMessage(r *http.Request) string {
	return "Invalid request: ServletWebRequest: uri=" + r.URL.Path + ";client=" + clientIP(r)
}

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return "127.0.0.1"
	}
	return host
}
