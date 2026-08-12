package model

import "time"

const (
	DefaultAppointmentIntervalMinutes = 60
	MinAppointmentIntervalMinutes     = 10
	MaxAppointmentIntervalMinutes     = 180
)

// SchedulingSettings preferencias de agenda del usuario.
// Documento: users/{uid}/settings/scheduling
type SchedulingSettings struct {
	AppointmentIntervalMinutes int       `json:"appointment_interval_minutes" firestore:"appointment_interval_minutes"`
	UpdatedAt                  time.Time `json:"updated_at" firestore:"updated_at"`
}

// UpsertSchedulingSettingsRequest body para guardar preferencias de agenda.
type UpsertSchedulingSettingsRequest struct {
	AppointmentIntervalMinutes int `json:"appointment_interval_minutes"`
}

// ValidAppointmentInterval indica si el intervalo es múltiplo de 10 y está en rango.
func ValidAppointmentInterval(minutes int) bool {
	if minutes < MinAppointmentIntervalMinutes || minutes > MaxAppointmentIntervalMinutes {
		return false
	}
	return minutes%10 == 0
}

// NormalizeAppointmentInterval devuelve un intervalo válido (default 60).
func NormalizeAppointmentInterval(minutes int) int {
	if ValidAppointmentInterval(minutes) {
		return minutes
	}
	return DefaultAppointmentIntervalMinutes
}
