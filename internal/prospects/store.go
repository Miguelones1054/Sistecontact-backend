package prospects

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sistecontact/api/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store: users/{uid}/prospects/{placeId}
// y espejo global de citas: business_scheduled/{placeId}/schedulers/{uid}
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) col(uid string) *firestore.CollectionRef {
	return s.db.Collection("users").Doc(uid).Collection("prospects")
}

func (s *Store) globalSchedulers(placeID string) *firestore.CollectionRef {
	return s.db.Collection("business_scheduled").Doc(sanitizePlaceID(placeID)).Collection("schedulers")
}

func sanitizePlaceID(placeID string) string {
	return strings.ReplaceAll(placeID, "/", "_")
}

func (s *Store) List(ctx context.Context, uid string) ([]model.Prospect, error) {
	docs, err := s.col(uid).OrderBy("updated_at", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("listar prospectos: %w", err)
	}
	out := make([]model.Prospect, 0, len(docs))
	for _, d := range docs {
		var item model.Prospect
		if err := d.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) GetByPlaceIDs(ctx context.Context, uid string, placeIDs []string) ([]model.Prospect, error) {
	if len(placeIDs) == 0 {
		return s.List(ctx, uid)
	}
	out := make([]model.Prospect, 0, len(placeIDs))
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
			return nil, fmt.Errorf("leer prospecto %s: %w", id, err)
		}
		var item model.Prospect
		if err := doc.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) ListGlobalSchedulers(ctx context.Context, placeID string) ([]model.GlobalScheduledVisit, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return nil, fmt.Errorf("place_id vacío")
	}

	docs, err := s.globalSchedulers(placeID).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("listar citas programadas: %w", err)
	}

	out := make([]model.GlobalScheduledVisit, 0, len(docs))
	for _, d := range docs {
		var item model.GlobalScheduledVisit
		if err := d.DataTo(&item); err != nil {
			continue
		}
		out = append(out, item)
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].VisitDate, out[j].VisitDate
		if a == b {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a < b
	})
	return out, nil
}

func (s *Store) Upsert(ctx context.Context, identity model.VisitorIdentity, placeID string, req model.UpsertProspectRequest) (model.Prospect, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return model.Prospect{}, fmt.Errorf("place_id vacío")
	}
	if identity.UID == "" {
		return model.Prospect{}, fmt.Errorf("uid vacío")
	}

	ref := s.col(identity.UID).Doc(sanitizePlaceID(placeID))
	globalRef := s.globalSchedulers(placeID).Doc(identity.UID)
	now := time.Now().UTC()
	createdAt := now

	doc, err := ref.Get(ctx)
	var existing model.Prospect
	if err == nil {
		if err := doc.DataTo(&existing); err == nil && !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt
		}
	} else if status.Code(err) != codes.NotFound {
		return model.Prospect{}, fmt.Errorf("leer prospecto: %w", err)
	}

	contactStatus := strings.TrimSpace(req.ContactStatus)
	if contactStatus == "" {
		contactStatus = existing.ContactStatus
	}
	if contactStatus == "" {
		contactStatus = model.ContactStatusNotContacted
	}
	if !model.ValidContactStatus(contactStatus) {
		return model.Prospect{}, fmt.Errorf("contact_status inválido")
	}

	visitDate := strings.TrimSpace(req.VisitDate)
	if req.ClearVisitDate {
		visitDate = ""
	} else if visitDate == "" {
		visitDate = existing.VisitDate
	}
	if visitDate != "" {
		if _, err := time.Parse("2006-01-02", visitDate); err != nil {
			return model.Prospect{}, fmt.Errorf("visit_date inválida (usa YYYY-MM-DD)")
		}
	}

	item := model.Prospect{
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
		VisitDate:       visitDate,
		CreatedAt:       createdAt,
		UpdatedAt:       now,
	}
	if item.Name == "" {
		item.Name = existing.Name
	}
	if item.Address == "" {
		item.Address = existing.Address
	}
	if item.Phone == "" {
		item.Phone = existing.Phone
	}
	if item.GoogleMapsURI == "" {
		item.GoogleMapsURI = existing.GoogleMapsURI
	}
	if item.Rating == 0 {
		item.Rating = existing.Rating
	}
	if item.UserRatingCount == 0 {
		item.UserRatingCount = existing.UserRatingCount
	}
	if item.Latitude == 0 && item.Longitude == 0 {
		item.Latitude = existing.Latitude
		item.Longitude = existing.Longitude
	}
	if item.OpenNow == nil {
		item.OpenNow = existing.OpenNow
	}
	if item.Name == "" {
		return model.Prospect{}, fmt.Errorf("name es obligatorio")
	}

	batch := s.db.Batch()
	batch.Set(ref, item)

	if visitDate != "" {
		displayName := strings.TrimSpace(identity.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(identity.Email)
		}
		batch.Set(globalRef, model.GlobalScheduledVisit{
			UID:          identity.UID,
			Email:        strings.TrimSpace(identity.Email),
			DisplayName:  displayName,
			PlaceID:      placeID,
			BusinessName: item.Name,
			VisitDate:    visitDate,
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		})
	} else {
		batch.Delete(globalRef)
	}

	if _, err := batch.Commit(ctx); err != nil {
		return model.Prospect{}, fmt.Errorf("guardar prospecto: %w", err)
	}
	return item, nil
}

func (s *Store) Delete(ctx context.Context, uid, placeID string) error {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return fmt.Errorf("place_id vacío")
	}
	if uid == "" {
		return fmt.Errorf("uid vacío")
	}

	batch := s.db.Batch()
	batch.Delete(s.col(uid).Doc(sanitizePlaceID(placeID)))
	batch.Delete(s.globalSchedulers(placeID).Doc(uid))
	if _, err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("eliminar prospecto: %w", err)
	}
	return nil
}

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
		return fmt.Errorf("leer prospecto: %w", err)
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
