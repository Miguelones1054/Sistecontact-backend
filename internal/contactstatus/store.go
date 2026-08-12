package contactstatus

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

// Store: users/{uid}/contact_status/{placeId}
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) col(uid string) *firestore.CollectionRef {
	return s.db.Collection("users").Doc(uid).Collection("contact_status")
}

func sanitizePlaceID(placeID string) string {
	return strings.ReplaceAll(placeID, "/", "_")
}

func (s *Store) GetByPlaceIDs(ctx context.Context, uid string, placeIDs []string) ([]model.ContactStatusRecord, error) {
	if len(placeIDs) == 0 {
		docs, err := s.col(uid).Documents(ctx).GetAll()
		if err != nil {
			return nil, fmt.Errorf("listar estados: %w", err)
		}
		out := make([]model.ContactStatusRecord, 0, len(docs))
		for _, d := range docs {
			var item model.ContactStatusRecord
			if err := d.DataTo(&item); err != nil {
				continue
			}
			out = append(out, item)
		}
		return out, nil
	}

	out := make([]model.ContactStatusRecord, 0, len(placeIDs))
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
			return nil, fmt.Errorf("leer estado %s: %w", id, err)
		}
		var item model.ContactStatusRecord
		if err := doc.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) Upsert(ctx context.Context, uid, placeID string, req model.UpsertContactStatusRequest) (model.ContactStatusRecord, error) {
	placeID = strings.TrimSpace(placeID)
	statusValue := strings.TrimSpace(req.ContactStatus)
	if placeID == "" {
		return model.ContactStatusRecord{}, fmt.Errorf("place_id vacío")
	}
	if uid == "" {
		return model.ContactStatusRecord{}, fmt.Errorf("uid vacío")
	}
	if !model.ValidContactStatus(statusValue) {
		return model.ContactStatusRecord{}, fmt.Errorf("contact_status inválido")
	}

	outcome := strings.TrimSpace(req.ContactOutcome)
	notes := strings.TrimSpace(req.ContactNotes)
	if statusValue == model.ContactStatusNotContacted {
		outcome = ""
		notes = ""
	}
	if !model.ValidContactOutcome(outcome) {
		return model.ContactStatusRecord{}, fmt.Errorf("contact_outcome inválido")
	}

	item := model.ContactStatusRecord{
		PlaceID:        placeID,
		Name:           strings.TrimSpace(req.Name),
		Address:        strings.TrimSpace(req.Address),
		ContactStatus:  statusValue,
		ContactOutcome: outcome,
		ContactNotes:   notes,
		UpdatedAt:      time.Now().UTC(),
	}

	ref := s.col(uid).Doc(sanitizePlaceID(placeID))
	if item.Name == "" || item.Address == "" {
		if doc, err := ref.Get(ctx); err == nil {
			var existing model.ContactStatusRecord
			if err := doc.DataTo(&existing); err == nil {
				if item.Name == "" {
					item.Name = existing.Name
				}
				if item.Address == "" {
					item.Address = existing.Address
				}
			}
		}
	}

	if _, err := ref.Set(ctx, item); err != nil {
		return model.ContactStatusRecord{}, fmt.Errorf("guardar estado: %w", err)
	}
	return item, nil
}
