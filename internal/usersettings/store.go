package usersettings

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sistecontact/api/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store: users/{uid}/settings/{scheduling|access}
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) schedulingRef(uid string) *firestore.DocumentRef {
	return s.db.Collection("users").Doc(uid).Collection("settings").Doc("scheduling")
}

func (s *Store) accessRef(uid string) *firestore.DocumentRef {
	return s.db.Collection("users").Doc(uid).Collection("settings").Doc("access")
}

// GetOrCreateAccess lee users/{uid}/settings/access.
// Si no existe, lo crea con sistecontact_enabled=false.
func (s *Store) GetOrCreateAccess(ctx context.Context, uid string) (model.AccessSettings, error) {
	if uid == "" {
		return model.AccessSettings{}, fmt.Errorf("uid vacío")
	}

	ref := s.accessRef(uid)
	doc, err := ref.Get(ctx)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return model.AccessSettings{}, fmt.Errorf("leer acceso: %w", err)
		}
		item := model.AccessSettings{
			SistecontactEnabled: false,
			UpdatedAt:           time.Now().UTC(),
		}
		if _, err := ref.Set(ctx, item); err != nil {
			return model.AccessSettings{}, fmt.Errorf("crear acceso: %w", err)
		}
		return item, nil
	}

	var item model.AccessSettings
	if err := doc.DataTo(&item); err != nil {
		return model.AccessSettings{
			SistecontactEnabled: false,
			UpdatedAt:           time.Now().UTC(),
		}, nil
	}
	return item, nil
}

func (s *Store) GetScheduling(ctx context.Context, uid string) (model.SchedulingSettings, error) {
	if uid == "" {
		return model.SchedulingSettings{}, fmt.Errorf("uid vacío")
	}

	doc, err := s.schedulingRef(uid).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return model.SchedulingSettings{
				AppointmentIntervalMinutes: model.DefaultAppointmentIntervalMinutes,
			}, nil
		}
		return model.SchedulingSettings{}, fmt.Errorf("leer configuración de agenda: %w", err)
	}

	var item model.SchedulingSettings
	if err := doc.DataTo(&item); err != nil {
		return model.SchedulingSettings{
			AppointmentIntervalMinutes: model.DefaultAppointmentIntervalMinutes,
		}, nil
	}
	item.AppointmentIntervalMinutes = model.NormalizeAppointmentInterval(item.AppointmentIntervalMinutes)
	return item, nil
}

func (s *Store) UpsertScheduling(
	ctx context.Context,
	uid string,
	req model.UpsertSchedulingSettingsRequest,
) (model.SchedulingSettings, error) {
	if uid == "" {
		return model.SchedulingSettings{}, fmt.Errorf("uid vacío")
	}
	if !model.ValidAppointmentInterval(req.AppointmentIntervalMinutes) {
		return model.SchedulingSettings{}, fmt.Errorf(
			"appointment_interval_minutes inválido (usa múltiplos de 10 entre %d y %d)",
			model.MinAppointmentIntervalMinutes,
			model.MaxAppointmentIntervalMinutes,
		)
	}

	item := model.SchedulingSettings{
		AppointmentIntervalMinutes: req.AppointmentIntervalMinutes,
		UpdatedAt:                  time.Now().UTC(),
	}
	if _, err := s.schedulingRef(uid).Set(ctx, item); err != nil {
		return model.SchedulingSettings{}, fmt.Errorf("guardar configuración de agenda: %w", err)
	}
	return item, nil
}
