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

// Store: users/{uid}/settings/scheduling
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) schedulingRef(uid string) *firestore.DocumentRef {
	return s.db.Collection("users").Doc(uid).Collection("settings").Doc("scheduling")
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
