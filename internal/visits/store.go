package visits

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/sistecontact/api/internal/model"
	"github.com/sistecontact/api/internal/placeid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store persiste visitas en:
//   - users/{uid}/visits/{placeId}              (privado por usuario)
//   - business_visits/{placeId}/visitors/{uid}  (global: quién visitó)
type Store struct {
	db   *firestore.Client
	auth *auth.Client
}

func NewStore(db *firestore.Client, authClient *auth.Client) *Store {
	return &Store{db: db, auth: authClient}
}

func (s *Store) userVisits(uid string) *firestore.CollectionRef {
	return s.db.Collection("users").Doc(uid).Collection("visits")
}

func (s *Store) globalVisitors(placeID string) *firestore.CollectionRef {
	return s.db.Collection("business_visits").Doc(placeid.SanitizeDocID(placeID)).Collection("visitors")
}

func sanitizePlaceID(placeID string) string {
	return placeid.SanitizeDocID(placeID)
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

// ListGlobalVisitors lista quién ha visitado, contactado o tiene el comercio en prospectos.
func (s *Store) ListGlobalVisitors(ctx context.Context, placeID string) ([]model.GlobalVisitor, error) {
	placeID = placeid.Normalize(strings.TrimSpace(placeID))
	if placeID == "" {
		return nil, fmt.Errorf("place_id vacío")
	}

	byUID := map[string]model.GlobalVisitor{}
	mergeVisitor := func(item model.GlobalVisitor) {
		if item.UID == "" {
			return
		}
		prev, ok := byUID[item.UID]
		if !ok {
			byUID[item.UID] = item
			return
		}
		merged := prev
		if merged.Email == "" {
			merged.Email = item.Email
		}
		if merged.DisplayName == "" {
			merged.DisplayName = item.DisplayName
		}
		if merged.BusinessName == "" {
			merged.BusinessName = item.BusinessName
		}
		if merged.PlaceID == "" {
			merged.PlaceID = item.PlaceID
		}
		if merged.VisitResult == "" {
			merged.VisitResult = item.VisitResult
		}
		if merged.ContactOutcome == "" {
			merged.ContactOutcome = item.ContactOutcome
		}
		if merged.ContactStatus == "" {
			merged.ContactStatus = item.ContactStatus
		}
		if item.UpdatedAt.After(merged.UpdatedAt) {
			merged.UpdatedAt = item.UpdatedAt
		}
		if !item.VisitedAt.IsZero() && (merged.VisitedAt.IsZero() || item.VisitedAt.After(merged.VisitedAt)) {
			merged.VisitedAt = item.VisitedAt
		}
		byUID[item.UID] = merged
	}

	if docs, err := s.globalVisitors(placeID).Documents(ctx).GetAll(); err == nil {
		for _, d := range docs {
			var v model.GlobalVisitor
			if err := d.DataTo(&v); err != nil {
				continue
			}
			if v.UID == "" {
				v.UID = d.Ref.ID
			}
			mergeVisitor(v)
		}
	}

	cgOK := true
	for _, pid := range placeid.Variants(placeID) {
		docs, err := s.db.CollectionGroup("visits").Where("place_id", "==", pid).Documents(ctx).GetAll()
		if err != nil {
			slog.Warn("collection group visits por place_id", "place_id", pid, "err", err)
			cgOK = false
			break
		}
		for _, d := range docs {
			var visit model.Visit
			if err := d.DataTo(&visit); err != nil {
				continue
			}
			uid := parentUserID(d)
			if uid == "" {
				continue
			}
			mergeVisitor(model.GlobalVisitor{
				UID:          uid,
				PlaceID:      visit.PlaceID,
				BusinessName: visit.Name,
				VisitResult:  visit.VisitResult,
				VisitedAt:    visit.CreatedAt,
				UpdatedAt:    visit.UpdatedAt,
			})
		}
	}

	if cgOK {
		for _, pid := range placeid.Variants(placeID) {
			docs, err := s.db.CollectionGroup("prospects").Where("place_id", "==", pid).Documents(ctx).GetAll()
			if err != nil {
				slog.Warn("collection group prospects (visitors) por place_id", "place_id", pid, "err", err)
				cgOK = false
				break
			}
			for _, d := range docs {
				var p model.Prospect
				if err := d.DataTo(&p); err != nil {
					continue
				}
				uid := parentUserID(d)
				if uid == "" {
					continue
				}
				mergeVisitor(visitorFromProspect(uid, p))
			}
		}
	}

	// Fallback sin índice Firestore: users/{uid}/visits|prospects/{docId}.
	if !cgOK {
		if err := s.scanUsersForPlaceActivity(ctx, placeID, mergeVisitor); err != nil {
			slog.Warn("scan users activity", "place_id", placeID, "err", err)
		}
	}

	out := make([]model.GlobalVisitor, 0, len(byUID))
	for _, v := range byUID {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func parentUserID(d *firestore.DocumentSnapshot) string {
	if d == nil || d.Ref == nil || d.Ref.Parent == nil || d.Ref.Parent.Parent == nil {
		return ""
	}
	return d.Ref.Parent.Parent.ID
}

func visitorFromProspect(uid string, p model.Prospect) model.GlobalVisitor {
	outcome := strings.TrimSpace(p.ContactOutcome)
	status := strings.TrimSpace(p.ContactStatus)
	result := outcome
	if result == "" {
		result = status
	}
	if result == "" || result == model.ContactStatusNotContacted {
		result = "in_prospects"
	}
	return model.GlobalVisitor{
		UID:            uid,
		PlaceID:        p.PlaceID,
		BusinessName:   p.Name,
		VisitResult:    result,
		ContactOutcome: outcome,
		ContactStatus:  status,
		VisitedAt:      p.UpdatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func (s *Store) scanUsersForPlaceActivity(ctx context.Context, placeID string, merge func(model.GlobalVisitor)) error {
	docIDs := placeid.DocIDCandidates(placeID)
	seenUID := map[string]struct{}{}

	// 1) Collection group sin filtro (no requiere índice de place_id) + filtro en memoria.
	if err := s.scanProspectsGroup(ctx, placeID, merge, seenUID); err != nil {
		slog.Warn("scan prospects group", "place_id", placeID, "err", err)
	}
	if err := s.scanVisitsGroup(ctx, placeID, merge, seenUID); err != nil {
		slog.Warn("scan visits group", "place_id", placeID, "err", err)
	}

	// 2) Auth users: lee docs directos por ID (cubre usuarios sin doc padre en /users).
	if s.auth != nil {
		iter := s.auth.Users(ctx, "")
		for {
			u, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			uid := u.UID
			if _, ok := seenUID[uid]; ok {
				continue
			}
			userRef := s.db.Collection("users").Doc(uid)
			for _, docID := range docIDs {
				if doc, err := userRef.Collection("visits").Doc(docID).Get(ctx); err == nil {
					var visit model.Visit
					if err := doc.DataTo(&visit); err == nil {
						merge(model.GlobalVisitor{
							UID:          uid,
							PlaceID:      visit.PlaceID,
							BusinessName: visit.Name,
							VisitResult:  visit.VisitResult,
							VisitedAt:    visit.CreatedAt,
							UpdatedAt:    visit.UpdatedAt,
						})
					}
				}
				if doc, err := userRef.Collection("prospects").Doc(docID).Get(ctx); err == nil {
					var p model.Prospect
					if err := doc.DataTo(&p); err == nil {
						merge(visitorFromProspect(uid, p))
					}
				}
			}
		}
	}
	return nil
}

func (s *Store) scanProspectsGroup(ctx context.Context, placeID string, merge func(model.GlobalVisitor), seen map[string]struct{}) error {
	it := s.db.CollectionGroup("prospects").Documents(ctx)
	for {
		d, err := it.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return err
		}
		if !placeid.MatchesDoc(placeID, d.Ref.ID) {
			var p model.Prospect
			if err := d.DataTo(&p); err != nil || !placeid.Matches(placeID, p.PlaceID) {
				continue
			}
			uid := parentUserID(d)
			if uid == "" {
				continue
			}
			seen[uid] = struct{}{}
			merge(visitorFromProspect(uid, p))
			continue
		}
		var p model.Prospect
		if err := d.DataTo(&p); err != nil {
			continue
		}
		uid := parentUserID(d)
		if uid == "" {
			continue
		}
		seen[uid] = struct{}{}
		merge(visitorFromProspect(uid, p))
	}
}

func (s *Store) scanVisitsGroup(ctx context.Context, placeID string, merge func(model.GlobalVisitor), seen map[string]struct{}) error {
	it := s.db.CollectionGroup("visits").Documents(ctx)
	for {
		d, err := it.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return err
		}
		var visit model.Visit
		if err := d.DataTo(&visit); err != nil {
			continue
		}
		if !placeid.MatchesDoc(placeID, d.Ref.ID) && !placeid.Matches(placeID, visit.PlaceID) {
			continue
		}
		uid := parentUserID(d)
		if uid == "" {
			continue
		}
		seen[uid] = struct{}{}
		merge(model.GlobalVisitor{
			UID:          uid,
			PlaceID:      visit.PlaceID,
			BusinessName: visit.Name,
			VisitResult:  visit.VisitResult,
			VisitedAt:    visit.CreatedAt,
			UpdatedAt:    visit.UpdatedAt,
		})
	}
}

// Upsert marca/actualiza la visita privada y sincroniza la colección global.
func (s *Store) Upsert(ctx context.Context, identity model.VisitorIdentity, placeID string, req model.UpsertVisitRequest) (model.Visit, error) {
	placeID = placeid.Normalize(strings.TrimSpace(placeID))
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
