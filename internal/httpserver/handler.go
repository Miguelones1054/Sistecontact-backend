package httpserver

import (
	"net/http"

	"github.com/sistecontact/api/internal/search"
)

type Handler struct {
	svc *search.Service
}

func NewHandler(svc *search.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/zones?q=<texto>
func (h *Handler) zones(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "parámetro 'q' es obligatorio")
		return
	}
	zones, err := h.svc.FindZones(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, zones)
}

// GET /api/search?type=<tipo>&zone=<zona o place_id>
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	businessType := r.URL.Query().Get("type")
	zone := r.URL.Query().Get("zone")
	if businessType == "" || zone == "" {
		writeError(w, http.StatusBadRequest, "parámetros 'type' y 'zone' son obligatorios")
		return
	}

	resp, err := h.svc.Search(r.Context(), businessType, zone)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Evita que el navegador cachee la respuesta de la API.
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, resp)
}
