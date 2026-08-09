package model

import "time"

// ToVisit es un comercio pendiente de visitar para un usuario.
// Colección: users/{uid}/to_visit/{placeId}
type ToVisit struct {
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
	CreatedAt       time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" firestore:"updated_at"`
}

// UpsertToVisitRequest body para agregar/actualizar un comercio por visitar.
type UpsertToVisitRequest struct {
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
}
