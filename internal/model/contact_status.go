package model

import "time"

// ContactStatus valores permitidos para el seguimiento comercial.
const (
	ContactStatusNotContacted  = "not_contacted"
	ContactStatusContacted     = "contacted"
	ContactStatusNotInterested = "not_interested"
	ContactStatusAffiliated    = "affiliated"
	ContactStatusClosedSale    = "closed_sale"
)

// ContactOutcome resultado al marcar un comercio como contactado.
const (
	ContactOutcomeClosedSale    = "closed_sale"
	ContactOutcomeNotInterested = "not_interested"
	ContactOutcomeAffiliated    = "affiliated"
)

// ValidContactStatus indica si el valor es uno de los estados admitidos.
func ValidContactStatus(s string) bool {
	switch s {
	case ContactStatusNotContacted,
		ContactStatusContacted,
		ContactStatusNotInterested,
		ContactStatusAffiliated,
		ContactStatusClosedSale:
		return true
	default:
		return false
	}
}

// ValidContactOutcome indica si la clasificación de contacto es válida.
func ValidContactOutcome(s string) bool {
	switch s {
	case "",
		ContactOutcomeClosedSale,
		ContactOutcomeNotInterested,
		ContactOutcomeAffiliated:
		return true
	default:
		return false
	}
}

// ContactStatusOrder orden para listados (menor = primero).
func ContactStatusOrder(s string) int {
	switch s {
	case ContactStatusNotContacted, "":
		return 0
	case ContactStatusContacted:
		return 1
	case ContactStatusClosedSale:
		return 2
	case ContactStatusNotInterested:
		return 3
	case ContactStatusAffiliated:
		return 4
	default:
		return 99
	}
}

// ContactStatusRecord estado de contacto de un comercio para un usuario.
// Colección: users/{uid}/contact_status/{placeId}
type ContactStatusRecord struct {
	PlaceID        string    `json:"place_id" firestore:"place_id"`
	Name           string    `json:"name" firestore:"name"`
	Address        string    `json:"address" firestore:"address"`
	ContactStatus  string    `json:"contact_status" firestore:"contact_status"`
	ContactOutcome string    `json:"contact_outcome,omitempty" firestore:"contact_outcome"`
	ContactNotes   string    `json:"contact_notes,omitempty" firestore:"contact_notes"`
	UpdatedAt      time.Time `json:"updated_at" firestore:"updated_at"`
}

// UpsertContactStatusRequest body para guardar el estado.
type UpsertContactStatusRequest struct {
	Name           string `json:"name"`
	Address        string `json:"address"`
	ContactStatus  string `json:"contact_status"`
	ContactOutcome string `json:"contact_outcome"`
	ContactNotes   string `json:"contact_notes"`
}
