package tovisit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sistecontact/api/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store: users/{uid}/to_visit/{placeId}
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) col(uid string) *firestore.CollectionRef {
	return s.db.Collection("users").Doc(uid).Collection("to_visit")
}

func sanitizePlaceID(placeID string) string {
	return strings.ReplaceAll(placeID, "/", "_")
}

func (s *Store) List(ctx context.Context, uid string) ([]model.ToVisit, error) {
	docs, err := s.col(uid).OrderBy("updated_at", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("listar por visitar: %w", err)
	}
	out := make([]model.ToVisit, 0, len(docs))
	for _, d := range docs {
		var item model.ToVisit
		if err := d.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) GetByPlaceIDs(ctx context.Context, uid string, placeIDs []string) ([]model.ToVisit, error) {
	if len(placeIDs) == 0 {
		return s.List(ctx, uid)
	}
	out := make([]model.ToVisit, 0, len(placeIDs))
	for _, id := range placeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		doc, err := s.col(uid).Doc(sanitizePlaceID(id)).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return nil, fmt.Errorf("leer por visitar %s: %w", id, err)
		}
		var item model.ToVisit
		if err := doc.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) Upsert(ctx context.Context, uid, placeID string, req model.UpsertToVisitRequest) (model.ToVisit, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return model.ToVisit{}, fmt.Errorf("place_id vacío")
	}
	if uid == "" {
		return model.ToVisit{}, fmt.Errorf("uid vacío")
	}

	ref := s.col(uid).Doc(sanitizePlaceID(placeID))
	now := time.Now().UTC()
	createdAt := now

	doc, err := ref.Get(ctx)
	var existing model.ToVisit
	if err == nil {
		if err := doc.DataTo(&existing); err == nil && !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt
		}
	} else if status.Code(err) != codes.NotFound {
		return model.ToVisit{}, fmt.Errorf("leer por visitar: %w", err)
	}

	contactStatus := strings.TrimSpace(req.ContactStatus)
	if contactStatus == "" {
		contactStatus = existing.ContactStatus
	}
	if contactStatus == "" {
		contactStatus = model.ContactStatusNotContacted
	}
	if !model.ValidContactStatus(contactStatus) {
		return model.ToVisit{}, fmt.Errorf("contact_status inválido")
	}

	item := model.ToVisit{
		PlaceID:         placeID,
		Name:            strings.TrimSpace(req.Name),
		Address:         strings.TrimSpace(req.Address),
		Phone:           strings.TrimSpace(req.Phone),
		Rating:          req.Rating,
		UserRatingCount: req.UserRatingCount,
		GoogleMapsURI:   strings.TrimSpace(req.GoogleMapsURI),
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
		OpenNow:         req.OpenNow,
		ContactStatus:   contactStatus,
		CreatedAt:       createdAt,
		UpdatedAt:       now,
	}
	if item.Name == "" {
		item.Name = existing.Name
	}
	if item.Address == "" {
		item.Address = existing.Address
	}
	if item.Name == "" {
		return model.ToVisit{}, fmt.Errorf("name es obligatorio")
	}

	if _, err := ref.Set(ctx, item); err != nil {
		return model.ToVisit{}, fmt.Errorf("guardar por visitar: %w", err)
	}
	return item, nil
}

func (s *Store) Delete(ctx context.Context, uid, placeID string) error {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return fmt.Errorf("place_id vacío")
	}
	_, err := s.col(uid).Doc(sanitizePlaceID(placeID)).Delete(ctx)
	if err != nil {
		return fmt.Errorf("eliminar por visitar: %w", err)
	}
	return nil
}

// UpdateContactStatus actualiza solo el estado si el comercio está en por visitar.
func (s *Store) UpdateContactStatus(ctx context.Context, uid, placeID, contactStatus string) error {
	placeID = strings.TrimSpace(placeID)
	contactStatus = strings.TrimSpace(contactStatus)
	if placeID == "" {
		return fmt.Errorf("place_id vacío")
	}
	if !model.ValidContactStatus(contactStatus) {
		return fmt.Errorf("contact_status inválido")
	}

	ref := s.col(uid).Doc(sanitizePlaceID(placeID))
	_, err := ref.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return fmt.Errorf("leer por visitar: %w", err)
	}

	_, err = ref.Update(ctx, []firestore.Update{
		{Path: "contact_status", Value: contactStatus},
		{Path: "updated_at", Value: time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("actualizar estado: %w", err)
	}
	return nil
}
