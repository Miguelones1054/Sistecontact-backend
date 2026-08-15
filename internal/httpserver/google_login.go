package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	intent := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("intent")))
	register := intent == "register"
	authURL, err := h.gcalOAuth.AuthCodeURLForSignIn(register)
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

// POST /api/auth/google/session  { "id_token": "..." }
func (h *Handler) googleLoginWithIDToken(w http.ResponseWriter, r *http.Request) {
	if h.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "inicio de sesión con Google no configurado")
		return
	}
	var req struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.IDToken) == "" {
		writeError(w, http.StatusBadRequest, "id_token inválido")
		return
	}

	info, err := verifyGoogleIDToken(r.Context(), strings.TrimSpace(req.IDToken))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "token de Google inválido")
		return
	}

	user, err := h.ensureFirebaseGoogleUser(r.Context(), info.Email, info.Name, info.Sub)
	if err != nil {
		writeError(w, http.StatusBadGateway, "no se pudo crear o vincular el usuario")
		return
	}

	customToken, err := h.auth.CustomToken(r.Context(), user.UID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "no se pudo completar el inicio de sesión con Google")
		return
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	writeJSON(w, http.StatusOK, map[string]string{"custom_token": customToken})
}

type googleIDPayload struct {
	Sub             string
	Email           string
	Name            string
	EmailVerified   bool
}

func verifyGoogleIDToken(ctx context.Context, rawToken string) (googleIDPayload, error) {
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(rawToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return googleIDPayload{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return googleIDPayload{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return googleIDPayload{}, fmt.Errorf("tokeninfo status %d", res.StatusCode)
	}

	var raw struct {
		Iss            string `json:"iss"`
		Aud            string `json:"aud"`
		Azp            string `json:"azp"`
		Sub            string `json:"sub"`
		Email          string `json:"email"`
		EmailVerified  any    `json:"email_verified"`
		Name           string `json:"name"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return googleIDPayload{}, err
	}
	if raw.Sub == "" || strings.TrimSpace(raw.Email) == "" {
		return googleIDPayload{}, fmt.Errorf("token incompleto")
	}
	iss := strings.TrimRight(raw.Iss, "/")
	if iss != "https://accounts.google.com" && iss != "accounts.google.com" {
		return googleIDPayload{}, fmt.Errorf("emisor inválido")
	}
	if !googleAudienceAllowed(raw.Aud, raw.Azp) {
		return googleIDPayload{}, fmt.Errorf("audiencia inválida")
	}

	verified := false
	switch v := raw.EmailVerified.(type) {
	case bool:
		verified = v
	case string:
		verified = strings.EqualFold(v, "true")
	}
	if !verified {
		return googleIDPayload{}, fmt.Errorf("email no verificado")
	}

	return googleIDPayload{
		Sub:   raw.Sub,
		Email: raw.Email,
		Name:  raw.Name,
	}, nil
}

const firebaseProjectNumber = "811424258609"

func googleAudienceAllowed(aud, azp string) bool {
	allowed := []string{
		os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		os.Getenv("FIREBASE_GOOGLE_CLIENT_ID"),
	}
	for _, id := range allowed {
		id = strings.TrimSpace(id)
		if id != "" && (aud == id || azp == id) {
			return true
		}
	}
	return strings.Contains(aud, firebaseProjectNumber) || strings.Contains(azp, firebaseProjectNumber)
}

func (h *Handler) finishGoogleLogin(w http.ResponseWriter, r *http.Request, code string, linkCalendar bool) {
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

	user, err := h.ensureFirebaseGoogleUser(r.Context(), info.Email, info.Name, info.ID)
	if err != nil {
		http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
		return
	}

	if linkCalendar {
		if err := h.gcalStore.Save(r.Context(), user.UID, info.Email, tok, googlecalendar.TokenScope(tok)); err != nil {
			http.Redirect(w, r, h.gcalOAuth.FrontendLoginRedirect("error"), http.StatusFound)
			return
		}
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
	return h.ensureFirebaseGoogleUser(ctx, email, name, "")
}

func (h *Handler) ensureFirebaseGoogleUser(ctx context.Context, email, name, googleUID string) (*fbauth.UserRecord, error) {
	user, err := h.auth.GetUserByEmail(ctx, email)
	if err != nil {
		if !fbauth.IsUserNotFound(err) {
			return nil, err
		}
		params := (&fbauth.UserToCreate{}).Email(email).EmailVerified(true)
		if strings.TrimSpace(name) != "" {
			params = params.DisplayName(strings.TrimSpace(name))
		}
		user, err = h.auth.CreateUser(ctx, params)
		if err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(googleUID) == "" {
		return user, nil
	}
	for _, p := range user.ProviderUserInfo {
		if p.ProviderID == "google.com" {
			return user, nil
		}
	}

	updated, err := h.auth.UpdateUser(ctx, user.UID, (&fbauth.UserToUpdate{}).ProviderToLink(&fbauth.UserProvider{
		UID:         googleUID,
		ProviderID:  "google.com",
		Email:       email,
		DisplayName: name,
	}))
	if err != nil {
		return user, nil
	}
	return updated, nil
}

func newLoginTicketID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
