package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/sistecontact/api/internal/googlecalendar"
)

const loginTicketTTL = 5 * time.Minute

// GET /api/auth/google
func (h *Handler) googleLoginStart(w http.ResponseWriter, r *http.Request) {
	if h.gcalOAuth == nil || !h.gcalOAuth.Configured() {
		writeError(w, http.StatusServiceUnavailable, "inicio de sesión con Google no configurado")
		return
	}
	authURL, err := h.gcalOAuth.AuthCodeURL(googlecalendar.LoginStateUID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

// POST /api/auth/google/complete
func (h *Handler) googleLoginComplete(w http.ResponseWriter, r *http.Request) {
	if h.gcalStore == nil {
		writeError(w, http.StatusServiceUnavailable, "inicio de sesión con Google no configurado")
		return
	}
	var req struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Ticket) == "" {
		writeError(w, http.StatusBadRequest, "ticket inválido")
		return
	}
	token, err := h.gcalStore.ConsumeLoginTicket(r.Context(), strings.TrimSpace(req.Ticket))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no se pudo completar el inicio de sesión con Google")
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, map[string]string{"custom_token": token})
}

func (h *Handler) finishGoogleLogin(w http.ResponseWriter, r *http.Request, code string) {
	fail := h.gcalOAuth.FrontendLoginRedirect("error")
	if h.gcalStore == nil || h.auth == nil {
		http.Redirect(w, r, fail, http.StatusFound)
		return
	}

	tok, err := h.gcalOAuth.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	info, err := h.gcalOAuth.FetchUserInfo(r.Context(), tok)
	if err != nil || strings.TrimSpace(info.Email) == "" {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	user, err := h.ensureFirebaseUser(r.Context(), info.Email, info.Name)
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	if err := h.gcalStore.Save(r.Context(), user.UID, info.Email, tok, googlecalendar.TokenScope(tok)); err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	customToken, err := h.auth.CustomToken(r.Context(), user.UID)
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	ticket, err := newLoginTicketID()
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}
	if err := h.gcalStore.SaveLoginTicket(r.Context(), ticket, customToken, user.UID, loginTicketTTL); err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect(ticket), http.StatusFound)
}

func (h *Handler) ensureFirebaseUser(ctx context.Context, email, name string) (*fbauth.UserRecord, error) {
	user, err := h.auth.GetUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !fbauth.IsUserNotFound(err) {
		return nil, err
	}

	params := (&fbauth.UserToCreate{}).Email(email).EmailVerified(true)
	if strings.TrimSpace(name) != "" {
		params = params.DisplayName(strings.TrimSpace(name))
	}
	return h.auth.CreateUser(ctx, params)
}

func newLoginTicketID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
