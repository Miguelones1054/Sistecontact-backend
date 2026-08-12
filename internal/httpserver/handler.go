package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sistecontact/api/internal/contactstatus"
	"github.com/sistecontact/api/internal/model"
	"github.com/sistecontact/api/internal/prospects"
	"github.com/sistecontact/api/internal/search"
	"github.com/sistecontact/api/internal/usersettings"
	"github.com/sistecontact/api/internal/visits"
)

type Handler struct {
	svc           *search.Service
	visits        *visits.Store
	prospects     *prospects.Store
	contactStatus *contactstatus.Store
	settings      *usersettings.Store
}

func NewHandler(
	svc *search.Service,
	visitStore *visits.Store,
	prospectStore *prospects.Store,
	contactStore *contactstatus.Store,
	settingsStore *usersettings.Store,
) *Handler {
	return &Handler{
		svc:           svc,
		visits:        visitStore,
		prospects:     prospectStore,
		contactStatus: contactStore,
		settings:      settingsStore,
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
	// Si estaba en prospectos, se quita al marcar visitado.
	_ = h.prospects.Delete(r.Context(), identity.UID, placeID)
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

// GET /api/businesses/{placeId}/scheduled
func (h *Handler) listBusinessScheduled(w http.ResponseWriter, r *http.Request) {
	if _, ok := uidFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	placeID := r.PathValue("placeId")
	if placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id requerido")
		return
	}

	items, err := h.prospects.ListGlobalSchedulers(r.Context(), placeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// GET /api/prospects?place_ids=id1,id2 (opcional)
func (h *Handler) listProspects(w http.ResponseWriter, r *http.Request) {
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
		items []model.Prospect
		err   error
	)
	if len(placeIDs) > 0 {
		items, err = h.prospects.GetByPlaceIDs(r.Context(), uid, placeIDs)
	} else {
		items, err = h.prospects.List(r.Context(), uid)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, items)
}

// PUT /api/prospects/{placeId}
func (h *Handler) upsertProspect(w http.ResponseWriter, r *http.Request) {
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

	var req model.UpsertProspectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	settings, err := h.settings.GetScheduling(r.Context(), identity.UID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	item, err := h.prospects.Upsert(
		r.Context(),
		identity,
		placeID,
		req,
		settings.AppointmentIntervalMinutes,
	)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "visit_date") ||
			strings.Contains(msg, "visit_time") ||
			strings.Contains(msg, "call_date") ||
			strings.Contains(msg, "call_time") ||
			strings.Contains(msg, "ya tienes una llamada") ||
			strings.Contains(msg, "se cruza con otra") ||
			strings.Contains(msg, "contact_status") ||
			strings.Contains(msg, "name es obligatorio") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	_, _ = h.contactStatus.Upsert(r.Context(), identity.UID, placeID, model.UpsertContactStatusRequest{
		Name:           item.Name,
		Address:        item.Address,
		ContactStatus:  item.ContactStatus,
		ContactOutcome: item.ContactOutcome,
		ContactNotes:   item.ContactNotes,
	})
	writeJSON(w, http.StatusOK, item)
}

// DELETE /api/prospects/{placeId}
func (h *Handler) deleteProspect(w http.ResponseWriter, r *http.Request) {
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

	if err := h.prospects.Delete(r.Context(), uid, placeID); err != nil {
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
	_ = h.prospects.UpdateContactStatus(
		r.Context(),
		uid,
		placeID,
		item.ContactStatus,
		item.ContactOutcome,
		item.ContactNotes,
	)
	writeJSON(w, http.StatusOK, item)
}

// GET /api/settings/scheduling
func (h *Handler) getSchedulingSettings(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	item, err := h.settings.GetScheduling(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, item)
}

// PUT /api/settings/scheduling
func (h *Handler) upsertSchedulingSettings(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var req model.UpsertSchedulingSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	item, err := h.settings.UpsertScheduling(r.Context(), uid, req)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "appointment_interval_minutes") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
