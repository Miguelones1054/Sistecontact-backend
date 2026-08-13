package model

import "time"

// GoogleCalendarStatus es la respuesta pública (sin tokens).
type GoogleCalendarStatus struct {
	Configured  bool   `json:"configured"`
	Connected   bool   `json:"connected"`
	Email       string `json:"email,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
}

// GoogleCalendarConnectResponse contiene la URL de consentimiento de Google.
type GoogleCalendarConnectResponse struct {
	AuthURL string `json:"auth_url"`
}

// GoogleCalendarTokenDoc se guarda en Firestore (users/{uid}/integrations/google_calendar).
type GoogleCalendarTokenDoc struct {
	Email        string    `firestore:"email"`
	AccessToken  string    `firestore:"access_token"`
	RefreshToken string    `firestore:"refresh_token"`
	TokenType    string    `firestore:"token_type"`
	Expiry       time.Time `firestore:"expiry"`
	Scope        string    `firestore:"scope"`
	ConnectedAt  time.Time `firestore:"connected_at"`
	UpdatedAt    time.Time `firestore:"updated_at"`
}
