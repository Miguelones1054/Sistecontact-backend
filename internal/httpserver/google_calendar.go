package httpserver

import (
	"net/http"
	"time"

	"github.com/sistecontact/api/internal/googlecalendar"
	"github.com/sistecontact/api/internal/model"
)

// GET /api/integrations/google-calendar
func (h *Handler) googleCalendarStatus(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	configured := h.gcalOAuth != nil && h.gcalOAuth.Configured()
	resp := model.GoogleCalendarStatus{Configured: configured}
	if !configured || h.gcalStore == nil {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		writeJSON(w, http.StatusOK, resp)
		return
	}

	doc, err := h.gcalStore.Get(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if doc != nil && doc.RefreshToken != "" {
		resp.Connected = true
		resp.Email = doc.Email
		if !doc.ConnectedAt.IsZero() {
			resp.ConnectedAt = doc.ConnectedAt.UTC().Format(time.RFC3339)
		}
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/integrations/google-calendar/connect
func (h *Handler) googleCalendarConnect(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	if h.gcalOAuth == nil || !h.gcalOAuth.Configured() {
		writeError(w, http.StatusServiceUnavailable, "Google Calendar no está configurado en el servidor")
		return
	}

	authURL, err := h.gcalOAuth.AuthCodeURL(uid)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.GoogleCalendarConnectResponse{AuthURL: authURL})
}

// GET /api/integrations/google-calendar/callback
func (h *Handler) googleCalendarCallback(w http.ResponseWriter, r *http.Request) {
	if h.gcalOAuth == nil || !h.gcalOAuth.Configured() || h.gcalStore == nil {
		http.Error(w, "Google Calendar no configurado", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		target := h.gcalOAuth.FrontendRedirect("error")
		if googlecalendar.LooksLikeLoginState(q.Get("state")) {
			target = h.gcalOAuth.FrontendLoginRedirect("error")
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		target := h.gcalOAuth.FrontendRedirect("error")
		if googlecalendar.LooksLikeLoginState(state) {
			target = h.gcalOAuth.FrontendLoginRedirect("error")
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	uid, err := h.gcalOAuth.ParseState(state)
	if err != nil {
		target := h.gcalOAuth.FrontendRedirect("error")
		if googlecalendar.LooksLikeLoginState(state) {
			target = h.gcalOAuth.FrontendLoginRedirect("error")
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	if uid == googlecalendar.LoginStateUID || uid == googlecalendar.RegisterStateUID {
		h.finishGoogleLogin(w, r, code, uid == googlecalendar.RegisterStateUID)
		return
	}

	tok, err := h.gcalOAuth.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendRedirect("error"), http.StatusFound)
		return
	}

	email, _ := h.gcalOAuth.FetchEmail(r.Context(), tok)
	if err := h.gcalStore.Save(r.Context(), uid, email, tok, googlecalendar.TokenScope(tok)); err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendRedirect("error"), http.StatusFound)
		return
	}

	http.Redirect(w, r, h.gcalOAuth.FrontendRedirect("connected"), http.StatusFound)
}

// DELETE /api/integrations/google-calendar
func (h *Handler) googleCalendarDisconnect(w http.ResponseWriter, r *http.Request) {
	uid, ok := uidFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	if h.gcalStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Calendar no está configurado en el servidor")
		return
	}

	doc, err := h.gcalStore.Get(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if doc != nil && h.gcalOAuth != nil {
		token := doc.RefreshToken
		if token == "" {
			token = doc.AccessToken
		}
		_ = h.gcalOAuth.Revoke(r.Context(), token)
	}
	if err := h.gcalStore.Delete(r.Context(), uid); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disconnected": true})
}
