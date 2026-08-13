package model

import "time"

// Prospect es un comercio en la lista de prospectos de un usuario.
// Colección: users/{uid}/prospects/{placeId}
type Prospect struct {
	PlaceID         string    `json:"place_id" firestore:"place_id"`
	Name            string    `json:"name" firestore:"name"`
	Address         string    `json:"address" firestore:"address"`
	Phone           string    `json:"phone,omitempty" firestore:"phone"`
	Rating          float64   `json:"rating,omitempty" firestore:"rating"`
	UserRatingCount int       `json:"user_rating_count,omitempty" firestore:"user_rating_count"`
	GoogleMapsURI   string    `json:"google_maps_uri,omitempty" firestore:"google_maps_uri"`
	Latitude        float64   `json:"latitude,omitempty" firestore:"latitude"`
	Longitude       float64   `json:"longitude,omitempty" firestore:"longitude"`
	OpenNow         *bool     `json:"open_now,omitempty" firestore:"open_now"`
	ContactStatus   string    `json:"contact_status,omitempty" firestore:"contact_status"`
	ContactOutcome  string    `json:"contact_outcome,omitempty" firestore:"contact_outcome"`
	ContactNotes    string    `json:"contact_notes,omitempty" firestore:"contact_notes"`
	// VisitDate día planificado de visita en formato YYYY-MM-DD (opcional hasta agendar).
	VisitDate string `json:"visit_date,omitempty" firestore:"visit_date,omitempty"`
	// VisitTime hora planificada de visita en formato HH:MM (según intervalo del usuario).
	VisitTime string `json:"visit_time,omitempty" firestore:"visit_time,omitempty"`
	// CallDate día planificado de llamada en formato YYYY-MM-DD (opcional hasta agendar).
	CallDate string `json:"call_date,omitempty" firestore:"call_date,omitempty"`
	// CallTime hora planificada de llamada en formato HH:MM.
	CallTime string `json:"call_time,omitempty" firestore:"call_time,omitempty"`
	// CallGoogleEventID ID del evento en Google Calendar (llamada).
	CallGoogleEventID string `json:"call_google_event_id,omitempty" firestore:"call_google_event_id,omitempty"`
	// VisitGoogleEventID ID del evento en Google Calendar (visita).
	VisitGoogleEventID string `json:"visit_google_event_id,omitempty" firestore:"visit_google_event_id,omitempty"`
	// CalendarSyncStatus: synced | skipped | error (solo respuesta API, no Firestore).
	CalendarSyncStatus string `json:"calendar_sync_status,omitempty" firestore:"-"`
	// CalendarSyncError detalle si falló la sync con Calendar.
	CalendarSyncError string    `json:"calendar_sync_error,omitempty" firestore:"-"`
	CreatedAt         time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" firestore:"updated_at"`
}

// UpsertProspectRequest body para agregar/actualizar un prospecto.
type UpsertProspectRequest struct {
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	Phone           string  `json:"phone"`
	Rating          float64 `json:"rating"`
	UserRatingCount int     `json:"user_rating_count"`
	GoogleMapsURI   string  `json:"google_maps_uri"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	OpenNow         *bool   `json:"open_now"`
	ContactStatus   string  `json:"contact_status"`
	ContactOutcome  string  `json:"contact_outcome"`
	ContactNotes    string  `json:"contact_notes"`
	VisitDate       string `json:"visit_date"`
	VisitTime       string `json:"visit_time"`
	// ClearVisitDate fuerza quitar la fecha/hora de visita agendada.
	ClearVisitDate bool   `json:"clear_visit_date"`
	CallDate       string `json:"call_date"`
	CallTime       string `json:"call_time"`
	// ClearCallDate fuerza quitar la fecha/hora de llamada agendada.
	ClearCallDate bool `json:"clear_call_date"`
}

// GlobalScheduledVisit es la cita programada visible entre asesores.
// Colección: business_scheduled/{placeId}/schedulers/{uid}
type GlobalScheduledVisit struct {
	UID          string    `json:"uid" firestore:"uid"`
	Email        string    `json:"email" firestore:"email"`
	DisplayName  string    `json:"display_name" firestore:"display_name"`
	PlaceID      string    `json:"place_id" firestore:"place_id"`
	BusinessName string    `json:"business_name" firestore:"business_name"`
	VisitDate    string    `json:"visit_date" firestore:"visit_date"`
	CreatedAt    time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"`
}
