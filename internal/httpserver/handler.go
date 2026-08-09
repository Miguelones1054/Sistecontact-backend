package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sistecontact/api/internal/model"
	"github.com/sistecontact/api/internal/search"
	"github.com/sistecontact/api/internal/contactstatus"
	"github.com/sistecontact/api/internal/tovisit"
	"github.com/sistecontact/api/internal/visits"
)

type Handler struct {
	svc            *search.Service
	visits         *visits.Store
	tovisit        *tovisit.Store
	contactStatus  *contactstatus.Store
}

func NewHandler(
	svc *search.Service,
	visitStore *visits.Store,
	toVisitStore *tovisit.Store,
	contactStore *contactstatus.Store,
) *Handler {
	return &Handler{
		svc:           svc,
		visits:        visitStore,
		tovisit:       toVisitStore,
		contactStatus: contactStore,
	}
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

// GET /api/search?type=<tipo>&zone=<zona o place_id>&radius_km=<opcional>
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	businessType := r.URL.Query().Get("type")
	zone := r.URL.Query().Get("zone")
	if businessType == "" || zone == "" {
		writeError(w, http.StatusBadRequest, "parámetros 'type' y 'zone' son obligatorios")
		return
	}

	var radiusKm float64
	if raw := r.URL.Query().Get("radius_km"); raw != "" {
		v, err := parseRadius(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "parámetro 'radius_km' inválido")
			return
		}
		radiusKm = v
	}

	resp, err := h.svc.Search(r.Context(), businessType, zone, radiusKm)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/visits?place_ids=id1,id2
func (h *Handler) listVisits(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var placeIDs []string
	if raw := r.URL.Query().Get("place_ids"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				placeIDs = append(placeIDs, p)
			}
		}
	}

	items, err := h.visits.GetByPlaceIDs(r.Context(), uid, placeIDs)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// PUT /api/visits/{placeId}
func (h *Handler) upsertVisit(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	var req model.UpsertVisitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	visit, err := h.visits.Upsert(r.Context(), identity, placeID, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Si estaba en "por visitar", se quita al marcar visitado.
	_ = h.tovisit.Delete(r.Context(), identity.UID, placeID)
	writeJSON(w, http.StatusOK, visit)
}

// DELETE /api/visits/{placeId}
func (h *Handler) deleteVisit(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	if err := h.visits.Delete(r.Context(), uid, placeID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/businesses/{placeId}/visitors
func (h *Handler) listBusinessVisitors(w http.ResponseWriter, r *http.Request) {
	if _, ok := uidFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	items, err := h.visits.ListGlobalVisitors(r.Context(), placeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// GET /api/to-visit?place_ids=id1,id2 (opcional)
func (h *Handler) listToVisit(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var placeIDs []string
	if raw := r.URL.Query().Get("place_ids"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				placeIDs = append(placeIDs, p)
			}
		}
	}

	var (
		items []model.ToVisit
		err   error
	)
	if len(placeIDs) > 0 {
		items, err = h.tovisit.GetByPlaceIDs(r.Context(), uid, placeIDs)
	} else {
		items, err = h.tovisit.List(r.Context(), uid)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// PUT /api/to-visit/{placeId}
func (h *Handler) upsertToVisit(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	var req model.UpsertToVisitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	item, err := h.tovisit.Upsert(r.Context(), uid, placeID, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Mantén colección de estados alineada.
	_, _ = h.contactStatus.Upsert(r.Context(), uid, placeID, model.UpsertContactStatusRequest{
		Name:          item.Name,
		Address:       item.Address,
		ContactStatus: item.ContactStatus,
	})
	writeJSON(w, http.StatusOK, item)
}

// DELETE /api/to-visit/{placeId}
func (h *Handler) deleteToVisit(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	if err := h.tovisit.Delete(r.Context(), uid, placeID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/contact-status?place_ids=id1,id2
func (h *Handler) listContactStatus(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var placeIDs []string
	if raw := r.URL.Query().Get("place_ids"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				placeIDs = append(placeIDs, p)
			}
		}
	}

	items, err := h.contactStatus.GetByPlaceIDs(r.Context(), uid, placeIDs)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// PUT /api/contact-status/{placeId}
func (h *Handler) upsertContactStatus(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	var req model.UpsertContactStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	item, err := h.contactStatus.Upsert(r.Context(), uid, placeID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Sincroniza el estado en "por visitar" si el comercio ya está ahí.
	_ = h.tovisit.UpdateContactStatus(r.Context(), uid, placeID, item.ContactStatus)
	writeJSON(w, http.StatusOK, item)
}
