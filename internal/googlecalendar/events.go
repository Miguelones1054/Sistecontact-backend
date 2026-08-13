package googlecalendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const DefaultTimeZone = "America/Bogota"

// AppointmentKind identifica el tipo de cita.
type AppointmentKind string

const (
	KindCall  AppointmentKind = "call"
	KindVisit AppointmentKind = "visit"
)

// AppointmentEvent datos para crear/actualizar un evento de agenda.
type AppointmentEvent struct {
	Kind            AppointmentKind
	BusinessName    string
	Address         string
	Phone           string
	Date            string // YYYY-MM-DD
	Time            string // HH:MM
	DurationMinutes int
	TimeZone        string
	ExistingEventID string
}

// Client crea/actualiza/borra eventos en el Calendar del usuario.
type Client struct {
	oauth *OAuth
	store *Store
}

func NewClient(oauth *OAuth, store *Store) *Client {
	if oauth == nil || !oauth.Configured() || store == nil {
		return nil
	}
	return &Client{oauth: oauth, store: store}
}

func (c *Client) Enabled() bool {
	return c != nil && c.oauth != nil && c.oauth.Configured() && c.store != nil
}

func (c *Client) calendarService(ctx context.Context, uid string) (*calendar.Service, error) {
	doc, err := c.store.Get(ctx, uid)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.RefreshToken == "" {
		return nil, nil
	}

	tok := &oauth2.Token{
		AccessToken:  doc.AccessToken,
		RefreshToken: doc.RefreshToken,
		TokenType:    doc.TokenType,
		Expiry:       doc.Expiry,
	}
	ts := c.oauth.Config.TokenSource(ctx, tok)
	fresh, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("token Google Calendar: %w", err)
	}
	if fresh.AccessToken != doc.AccessToken || (!fresh.Expiry.IsZero() && !fresh.Expiry.Equal(doc.Expiry)) {
		_ = c.store.Save(ctx, uid, doc.Email, fresh, doc.Scope)
	}

	svc, err := calendar.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("cliente Google Calendar: %w", err)
	}
	return svc, nil
}

// UpsertEvent crea o actualiza un evento. Devuelve el event ID.
// Si el usuario no tiene Calendar conectado, retorna ("", nil).
func (c *Client) UpsertEvent(ctx context.Context, uid string, appt AppointmentEvent) (string, error) {
	if !c.Enabled() {
		return "", nil
	}
	svc, err := c.calendarService(ctx, uid)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", nil
	}

	ev, err := buildEvent(appt)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(appt.ExistingEventID) != "" {
		updated, err := svc.Events.Patch("primary", appt.ExistingEventID, ev).Context(ctx).Do()
		if err == nil {
			return updated.Id, nil
		}
		// Si el evento ya no existe, crear uno nuevo.
	}

	created, err := svc.Events.Insert("primary", ev).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("crear evento Calendar: %w", err)
	}
	return created.Id, nil
}

// DeleteEvent elimina un evento si existe. Ignora "not found".
func (c *Client) DeleteEvent(ctx context.Context, uid, eventID string) error {
	if !c.Enabled() {
		return nil
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil
	}
	svc, err := c.calendarService(ctx, uid)
	if err != nil {
		return err
	}
	if svc == nil {
		return nil
	}
	err = svc.Events.Delete("primary", eventID).Context(ctx).Do()
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("eliminar evento Calendar: %w", err)
	}
	return nil
}

func buildEvent(appt AppointmentEvent) (*calendar.Event, error) {
	date := strings.TrimSpace(appt.Date)
	clock := strings.TrimSpace(appt.Time)
	if date == "" || clock == "" {
		return nil, fmt.Errorf("fecha y hora son obligatorias para el evento")
	}
	tz := strings.TrimSpace(appt.TimeZone)
	if tz == "" {
		tz = DefaultTimeZone
	}
	dur := appt.DurationMinutes
	if dur <= 0 {
		dur = 60
	}

	start, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, locationOrLocal(tz))
	if err != nil {
		return nil, fmt.Errorf("fecha/hora de evento inválida")
	}
	end := start.Add(time.Duration(dur) * time.Minute)

	kindLabel := "Visita"
	if appt.Kind == KindCall {
		kindLabel = "Llamada"
	}
	name := strings.TrimSpace(appt.BusinessName)
	if name == "" {
		name = "Comercio"
	}

	descParts := []string{
		fmt.Sprintf("%s agendada desde SisteContact.", kindLabel),
	}
	if addr := strings.TrimSpace(appt.Address); addr != "" {
		descParts = append(descParts, "Dirección: "+addr)
	}
	if phone := strings.TrimSpace(appt.Phone); phone != "" {
		descParts = append(descParts, "Teléfono: "+phone)
	}

	ev := &calendar.Event{
		Summary:     fmt.Sprintf("%s — %s", kindLabel, name),
		Description: strings.Join(descParts, "\n"),
		Location:    strings.TrimSpace(appt.Address),
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: tz,
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: tz,
		},
		Reminders: &calendar.EventReminders{
			UseDefault: false,
			Overrides: []*calendar.EventReminder{
				{Method: "popup", Minutes: 30},
				{Method: "email", Minutes: 30},
			},
			ForceSendFields: []string{"UseDefault"},
		},
	}
	return ev, nil
}

func locationOrLocal(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Local
	}
	return loc
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found") || strings.Contains(msg, "notFound")
}
