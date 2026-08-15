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

func (s *Store) loginTicketRef(id string) *firestore.DocumentRef {
	return s.db.Collection("oauth_login_tickets").Doc(id)
}

func (s *Store) SaveLoginTicket(ctx context.Context, id, customToken, uid string, ttl time.Duration) error {
	if id == "" || customToken == "" {
		return fmt.Errorf("ticket vacío")
	}
	now := time.Now().UTC()
	data := map[string]any{
		"custom_token": customToken,
		"uid":          uid,
		"created_at":   now,
		"expires_at":   now.Add(ttl),
	}
	if _, err := s.loginTicketRef(id).Set(ctx, data); err != nil {
		return fmt.Errorf("guardar ticket de login Google: %w", err)
	}
	return nil
}

func (s *Store) ConsumeLoginTicket(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("ticket vacío")
	}
	ref := s.loginTicketRef(id)
	doc, err := ref.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", fmt.Errorf("ticket inválido o expirado")
		}
		return "", fmt.Errorf("leer ticket de login Google: %w", err)
	}

	var item struct {
		CustomToken string    `firestore:"custom_token"`
		ExpiresAt   time.Time `firestore:"expires_at"`
	}
	if err := doc.DataTo(&item); err != nil || item.CustomToken == "" {
		return "", fmt.Errorf("ticket inválido")
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		_, _ = ref.Delete(ctx)
		return "", fmt.Errorf("ticket inválido o expirado")
	}
	if _, err := ref.Delete(ctx); err != nil {
		return "", fmt.Errorf("consumir ticket de login Google: %w", err)
	}
	return item.CustomToken, nil
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
