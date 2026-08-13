package googlecalendar

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sistecontact/api/internal/model"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store: users/{uid}/integrations/google_calendar
type Store struct {
	db *firestore.Client
}

func NewStore(db *firestore.Client) *Store {
	return &Store{db: db}
}

func (s *Store) ref(uid string) *firestore.DocumentRef {
	return s.db.Collection("users").Doc(uid).Collection("integrations").Doc("google_calendar")
}

func (s *Store) Get(ctx context.Context, uid string) (*model.GoogleCalendarTokenDoc, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid vacío")
	}
	doc, err := s.ref(uid).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("leer integración Google Calendar: %w", err)
	}
	var item model.GoogleCalendarTokenDoc
	if err := doc.DataTo(&item); err != nil {
		return nil, fmt.Errorf("parsear integración Google Calendar: %w", err)
	}
	if item.RefreshToken == "" && item.AccessToken == "" {
		return nil, nil
	}
	return &item, nil
}

func (s *Store) Save(ctx context.Context, uid string, email string, tok *oauth2.Token, scope string) error {
	if uid == "" {
		return fmt.Errorf("uid vacío")
	}
	if tok == nil {
		return fmt.Errorf("token vacío")
	}

	now := time.Now().UTC()
	existing, _ := s.Get(ctx, uid)

	refresh := tok.RefreshToken
	if refresh == "" && existing != nil {
		refresh = existing.RefreshToken
	}
	connectedAt := now
	if existing != nil && !existing.ConnectedAt.IsZero() {
		connectedAt = existing.ConnectedAt
	}
	if scope == "" && existing != nil {
		scope = existing.Scope
	}
	if email == "" && existing != nil {
		email = existing.Email
	}

	item := model.GoogleCalendarTokenDoc{
		Email:        email,
		AccessToken:  tok.AccessToken,
		RefreshToken: refresh,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry.UTC(),
		Scope:        scope,
		ConnectedAt:  connectedAt,
		UpdatedAt:    now,
	}

	if _, err := s.ref(uid).Set(ctx, item); err != nil {
		return fmt.Errorf("guardar integración Google Calendar: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, uid string) error {
	if uid == "" {
		return fmt.Errorf("uid vacío")
	}
	_, err := s.ref(uid).Delete(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("eliminar integración Google Calendar: %w", err)
	}
	return nil
}
