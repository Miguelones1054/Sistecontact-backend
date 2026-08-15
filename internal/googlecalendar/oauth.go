package googlecalendar

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	ScopeCalendarEvents = "https://www.googleapis.com/auth/calendar.events"
	ScopeUserEmail      = "https://www.googleapis.com/auth/userinfo.email"
	ScopeUserProfile    = "https://www.googleapis.com/auth/userinfo.profile"
	LoginStateUID       = "__google_login__"
	RegisterStateUID    = "__google_register__"
	stateTTL            = 10 * time.Minute
)

// OAuth holds Google OAuth client settings.
type OAuth struct {
	Config         *oauth2.Config
	StateSecret    string
	FrontendOrigin string
}

func NewOAuth(clientID, clientSecret, redirectURL, stateSecret, frontendOrigin string) *OAuth {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	if stateSecret == "" {
		stateSecret = clientSecret
	}
	frontendOrigin = strings.TrimRight(frontendOrigin, "/")
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:5173"
	}
	return &OAuth{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{ScopeCalendarEvents, ScopeUserEmail},
			Endpoint:     google.Endpoint,
		},
		StateSecret:    stateSecret,
		FrontendOrigin: frontendOrigin,
	}
}

func (o *OAuth) Configured() bool {
	return o != nil && o.Config != nil && o.Config.ClientID != ""
}

func (o *OAuth) AuthCodeURL(uid string) (string, error) {
	if !o.Configured() {
		return "", fmt.Errorf("Google Calendar OAuth no configurado")
	}
	state, err := o.makeState(uid)
	if err != nil {
		return "", err
	}
	return o.Config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	), nil
}

func (o *OAuth) configWithScopes(scopes []string) *oauth2.Config {
	cfg := *o.Config
	cfg.Scopes = scopes
	return &cfg
}

// AuthCodeURLForSignIn inicia login o registro con Google.
// Login: solo cuenta/email, sin volver a pedir Calendar.
// Registro: Calendar + consentimiento completo.
func (o *OAuth) AuthCodeURLForSignIn(register bool) (string, error) {
	if !o.Configured() {
		return "", fmt.Errorf("Google Calendar OAuth no configurado")
	}
	uid := LoginStateUID
	scopes := []string{ScopeUserEmail, ScopeUserProfile}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("prompt", "select_account"),
	}
	if register {
		uid = RegisterStateUID
		scopes = []string{ScopeCalendarEvents, ScopeUserEmail, ScopeUserProfile}
		opts = []oauth2.AuthCodeOption{
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		}
	}
	state, err := o.makeState(uid)
	if err != nil {
		return "", err
	}
	return o.configWithScopes(scopes).AuthCodeURL(state, opts...), nil
}

func (o *OAuth) makeState(uid string) (string, error) {
	if uid == "" {
		return "", fmt.Errorf("uid vacío")
	}
	exp := time.Now().Add(stateTTL).Unix()
	payload := uid + "|" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, []byte(o.StateSecret))
	_, _ = mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	raw := payload + "|" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func (o *OAuth) ParseState(state string) (string, error) {
	if state == "" {
		return "", fmt.Errorf("state vacío")
	}
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", fmt.Errorf("state inválido")
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", fmt.Errorf("state inválido")
	}
	uid, expStr, sig := parts[0], parts[1], parts[2]
	payload := uid + "|" + expStr
	mac := hmac.New(sha256.New, []byte(o.StateSecret))
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", fmt.Errorf("state inválido")
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", fmt.Errorf("state expirado")
	}
	if uid == "" {
		return "", fmt.Errorf("state inválido")
	}
	return uid, nil
}

func (o *OAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if !o.Configured() {
		return nil, fmt.Errorf("Google Calendar OAuth no configurado")
	}
	return o.Config.Exchange(ctx, code)
}

type UserInfo struct {
	Email string
	Name  string
}

func (o *OAuth) FetchEmail(ctx context.Context, tok *oauth2.Token) (string, error) {
	info, err := o.FetchUserInfo(ctx, tok)
	if err != nil {
		return "", err
	}
	return info.Email, nil
}

func (o *OAuth) FetchUserInfo(ctx context.Context, tok *oauth2.Token) (UserInfo, error) {
	client := o.Config.Client(ctx, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return UserInfo{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("obtener email de Google: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return UserInfo{}, fmt.Errorf("obtener email de Google: status %d", res.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return UserInfo{}, fmt.Errorf("parsear email de Google: %w", err)
	}
	return UserInfo{Email: info.Email, Name: info.Name}, nil
}

func (o *OAuth) FrontendLoginRedirect(ticket string) string {
	q := url.Values{}
	q.Set("google_login", ticket)
	path := "/login"
	if ticket != "error" {
		path = "/"
	}
	return o.FrontendOrigin + path + "?" + q.Encode()
}

func LooksLikeLoginState(state string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(raw), LoginStateUID+"|") ||
		strings.HasPrefix(string(raw), RegisterStateUID+"|")
}

func (o *OAuth) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	form := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://oauth2.googleapis.com/revoke",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revocar token Google: %w", err)
	}
	defer res.Body.Close()
	// 200 = ok, 400 a veces si ya estaba revocado
	if res.StatusCode >= 500 {
		return fmt.Errorf("revocar token Google: status %d", res.StatusCode)
	}
	return nil
}

func (o *OAuth) FrontendRedirect(result string) string {
	q := url.Values{}
	q.Set("google_calendar", result)
	return o.FrontendOrigin + "/?" + q.Encode()
}

func TokenScope(tok *oauth2.Token) string {
	if tok == nil {
		return ""
	}
	if s, ok := tok.Extra("scope").(string); ok {
		return s
	}
	return ""
}
