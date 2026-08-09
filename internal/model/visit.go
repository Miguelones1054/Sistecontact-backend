package model

import "time"

// Visit es el registro de visita de un comercio por un usuario.
type Visit struct {
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
	Visited         bool      `json:"visited" firestore:"visited"`
	Notes           string    `json:"notes" firestore:"notes"`
	VisitResult     string    `json:"visit_result" firestore:"visit_result"`
	CreatedAt       time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" firestore:"updated_at"`
}

// UpsertVisitRequest body para marcar/actualizar una visita.
type UpsertVisitRequest struct {
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	Phone           string  `json:"phone"`
	Rating          float64 `json:"rating"`
	UserRatingCount int     `json:"user_rating_count"`
	GoogleMapsURI   string  `json:"google_maps_uri"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	OpenNow         *bool   `json:"open_now"`
	Notes           string  `json:"notes"`
	VisitResult     string  `json:"visit_result"`
}

// VisitorIdentity datos del usuario autenticado para la colección global.
type VisitorIdentity struct {
	UID         string
	Email       string
	DisplayName string
}

// GlobalVisitor es la entrada pública de quién visitó un comercio.
// Colección: business_visits/{placeId}/visitors/{uid}
type GlobalVisitor struct {
	UID          string    `json:"uid" firestore:"uid"`
	Email        string    `json:"email" firestore:"email"`
	DisplayName  string    `json:"display_name" firestore:"display_name"`
	PlaceID      string    `json:"place_id" firestore:"place_id"`
	BusinessName string    `json:"business_name" firestore:"business_name"`
	VisitResult  string    `json:"visit_result" firestore:"visit_result"`
	VisitedAt    time.Time `json:"visited_at" firestore:"visited_at"`
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"`
}
