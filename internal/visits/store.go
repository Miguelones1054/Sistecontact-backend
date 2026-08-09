package visits

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

// Store persiste visitas en:
//   - users/{uid}/visits/{placeId}              (privado por usuario)
//   - business_visits/{placeId}/visitors/{uid}  (global: quién visitó)
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) userVisits(uid string) *firestore.CollectionRef {
	return s.db.Collection("users").Doc(uid).Collection("visits")
}

func (s *Store) globalVisitors(placeID string) *firestore.CollectionRef {
	return s.db.Collection("business_visits").Doc(sanitizePlaceID(placeID)).Collection("visitors")
}

func sanitizePlaceID(placeID string) string {
	return strings.ReplaceAll(placeID, "/", "_")
}

// GetByPlaceIDs devuelve las visitas del usuario para los place_ids dados.
func (s *Store) GetByPlaceIDs(ctx context.Context, uid string, placeIDs []string) ([]model.Visit, error) {
	if len(placeIDs) == 0 {
		docs, err := s.userVisits(uid).Documents(ctx).GetAll()
		if err != nil {
			return nil, fmt.Errorf("listar visitas: %w", err)
		}
		out := make([]model.Visit, 0, len(docs))
		for _, d := range docs {
			var v model.Visit
			if err := d.DataTo(&v); err != nil {
				continue
			}
			out = append(out, v)
		}
		return out, nil
	}

	out := make([]model.Visit, 0, len(placeIDs))
	for _, id := range placeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		doc, err := s.userVisits(uid).Doc(sanitizePlaceID(id)).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return nil, fmt.Errorf("leer visita %s: %w", id, err)
		}
		var v model.Visit
		if err := doc.DataTo(&v); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// ListGlobalVisitors lista quién ha visitado un comercio (colección global).
func (s *Store) ListGlobalVisitors(ctx context.Context, placeID string) ([]model.GlobalVisitor, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return nil, fmt.Errorf("place_id vacío")
	}

	docs, err := s.globalVisitors(placeID).OrderBy("updated_at", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("listar visitantes globales: %w", err)
	}

	out := make([]model.GlobalVisitor, 0, len(docs))
	for _, d := range docs {
		var v model.GlobalVisitor
		if err := d.DataTo(&v); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// Upsert marca/actualiza la visita privada y sincroniza la colección global.
func (s *Store) Upsert(ctx context.Context, identity model.VisitorIdentity, placeID string, req model.UpsertVisitRequest) (model.Visit, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return model.Visit{}, fmt.Errorf("place_id vacío")
	}
	if identity.UID == "" {
		return model.Visit{}, fmt.Errorf("uid vacío")
	}

	userRef := s.userVisits(identity.UID).Doc(sanitizePlaceID(placeID))
	globalRef := s.globalVisitors(placeID).Doc(identity.UID)
	now := time.Now().UTC()

	var existing model.Visit
	createdAt := now
	doc, err := userRef.Get(ctx)
	if err == nil {
		_ = doc.DataTo(&existing)
		if !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt
		}
	} else if status.Code(err) != codes.NotFound {
		return model.Visit{}, fmt.Errorf("leer visita: %w", err)
	}

	visit := model.Visit{
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
		Visited:         true,
		Notes:           strings.TrimSpace(req.Notes),
		VisitResult:     strings.TrimSpace(req.VisitResult),
		CreatedAt:       createdAt,
		UpdatedAt:       now,
	}
	if visit.Name == "" {
		visit.Name = existing.Name
	}
	if visit.Address == "" {
		visit.Address = existing.Address
	}
	if visit.Phone == "" {
		visit.Phone = existing.Phone
	}
	if visit.GoogleMapsURI == "" {
		visit.GoogleMapsURI = existing.GoogleMapsURI
	}
	if visit.Rating == 0 {
		visit.Rating = existing.Rating
	}
	if visit.UserRatingCount == 0 {
		visit.UserRatingCount = existing.UserRatingCount
	}
	if visit.Latitude == 0 && visit.Longitude == 0 {
		visit.Latitude = existing.Latitude
		visit.Longitude = existing.Longitude
	}
	if visit.OpenNow == nil {
		visit.OpenNow = existing.OpenNow
	}

	displayName := strings.TrimSpace(identity.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(identity.Email)
	}

	global := model.GlobalVisitor{
		UID:          identity.UID,
		Email:        strings.TrimSpace(identity.Email),
		DisplayName:  displayName,
		PlaceID:      placeID,
		BusinessName: visit.Name,
		VisitResult:  visit.VisitResult,
		VisitedAt:    createdAt,
		UpdatedAt:    now,
	}

	batch := s.db.Batch()
	batch.Set(userRef, visit)
	batch.Set(globalRef, global)
	if _, err := batch.Commit(ctx); err != nil {
		return model.Visit{}, fmt.Errorf("guardar visita: %w", err)
	}
	return visit, nil
}

// Delete elimina la visita privada y la entrada global.
func (s *Store) Delete(ctx context.Context, uid, placeID string) error {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return fmt.Errorf("place_id vacío")
	}
	if uid == "" {
		return fmt.Errorf("uid vacío")
	}

	batch := s.db.Batch()
	batch.Delete(s.userVisits(uid).Doc(sanitizePlaceID(placeID)))
	batch.Delete(s.globalVisitors(placeID).Doc(uid))
	if _, err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("eliminar visita: %w", err)
	}
	return nil
}
