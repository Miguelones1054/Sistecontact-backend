package model

import "time"

// AccessSettings permiso de uso de SisteContact.
// Documento: users/{uid}/settings/access
type AccessSettings struct {
	SistecontactEnabled bool      `json:"sistecontact_enabled" firestore:"sistecontact_enabled"`
	UpdatedAt           time.Time `json:"updated_at" firestore:"updated_at"`
}
