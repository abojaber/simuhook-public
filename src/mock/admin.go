package mock

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AdminEndpointsPath is the admin API collection route. The item route is
// AdminEndpointsPath + "/". main mounts NewAdminHandler on both.
const AdminEndpointsPath = "/_admin/endpoints"

type adminHandler struct {
	registry *Registry
}

// NewAdminHandler returns the admin REST API handler. It serves both the
// collection route (AdminEndpointsPath) and the item route
// (AdminEndpointsPath + "/{id}"); mount it on both. It panics if registry is nil.
func NewAdminHandler(registry *Registry) http.Handler {
	if registry == nil {
		panic("mock.NewAdminHandler: registry must not be nil")
	}
	return &adminHandler{registry: registry}
}

func (h *adminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == AdminEndpointsPath {
		h.serveCollection(w, r)
		return
	}
	h.serveItem(w, r)
}

func (h *adminHandler) serveCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.registry.List())

	case http.MethodPost:
		var ep Endpoint
		if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.registry.Add(&ep); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		// NOTE: no Content-Type is set here — preserved byte-for-byte from
		// main.go (net/http sniffs text/plain; charset=utf-8). Approved Q1.
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&ep)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *adminHandler) serveItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, AdminEndpointsPath+"/")
	if id == "" {
		http.Error(w, "missing endpoint id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		ep, err := h.registry.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ep)

	case http.MethodPut:
		var ep Endpoint
		if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// registry.Update sets ep.ID = id — the response reports the PATH id.
		if err := h.registry.Update(id, &ep); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&ep)

	case http.MethodDelete:
		if err := h.registry.Remove(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
