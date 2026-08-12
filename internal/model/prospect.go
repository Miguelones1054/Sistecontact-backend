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
	VisitDate string    `json:"visit_date,omitempty" firestore:"visit_date"`
	CreatedAt time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time `json:"updated_at" firestore:"updated_at"`
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
	VisitDate       string  `json:"visit_date"`
	// ClearVisitDate fuerza quitar la fecha de visita agendada.
	ClearVisitDate bool `json:"clear_visit_date"`
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
