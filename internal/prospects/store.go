package prospects

import (
	"context"
	"fmt"
	"math"
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

func validateClockTime(field, value string) error {
	if _, err := time.Parse("15:04", value); err != nil {
		return fmt.Errorf("%s inválida (usa HH:MM)", field)
	}
	return nil
}

func parseAppointmentDateTime(date, clock string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", date+" "+clock, time.Local)
}

func (s *Store) ensureAppointmentSlotFree(
	ctx context.Context,
	uid, placeID, date, clock string,
	intervalMinutes int,
) error {
	intervalMinutes = model.NormalizeAppointmentInterval(intervalMinutes)
	newAt, err := parseAppointmentDateTime(date, clock)
	if err != nil {
		return fmt.Errorf("fecha/hora de cita inválida")
	}

	items, err := s.List(ctx, uid)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.PlaceID == placeID {
			continue
		}

		check := func(otherDate, otherTime string) error {
			if otherDate == "" || otherTime == "" {
				return nil
			}
			existingAt, err := parseAppointmentDateTime(otherDate, otherTime)
			if err != nil {
				return nil
			}
			diff := int(math.Abs(newAt.Sub(existingAt).Minutes()))
			if diff < intervalMinutes {
				name := strings.TrimSpace(item.Name)
				if name == "" {
					name = item.PlaceID
				}
				return fmt.Errorf(
					"la cita se cruza con otra visita agendada el %s a las %s con \"%s\" (intervalo mínimo: %d min)",
					otherDate,
					otherTime,
					name,
					intervalMinutes,
				)
			}
			return nil
		}

		if err := check(item.CallDate, item.CallTime); err != nil {
			return err
		}
		if err := check(item.VisitDate, item.VisitTime); err != nil {
			return err
		}
	}
	return nil
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

func (s *Store) Upsert(
	ctx context.Context,
	identity model.VisitorIdentity,
	placeID string,
	req model.UpsertProspectRequest,
	appointmentIntervalMinutes int,
) (model.Prospect, error) {
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

	outcome := strings.TrimSpace(req.ContactOutcome)
	notes := strings.TrimSpace(req.ContactNotes)
	if contactStatus == model.ContactStatusNotContacted {
		outcome = ""
		notes = ""
	} else {
		if outcome == "" {
			outcome = existing.ContactOutcome
		}
		if notes == "" {
			notes = existing.ContactNotes
		}
	}
	if !model.ValidContactOutcome(outcome) {
		return model.Prospect{}, fmt.Errorf("contact_outcome inválido")
	}

	visitDate := existing.VisitDate
	visitTime := existing.VisitTime
	reqVisitDate := strings.TrimSpace(req.VisitDate)
	reqVisitTime := strings.TrimSpace(req.VisitTime)
	if req.ClearVisitDate {
		visitDate = ""
		visitTime = ""
	} else {
		if reqVisitDate != "" {
			visitDate = reqVisitDate
		}
		if reqVisitTime != "" {
			visitTime = reqVisitTime
		}
	}
	if visitDate != "" {
		if _, err := time.Parse("2006-01-02", visitDate); err != nil {
			return model.Prospect{}, fmt.Errorf("visit_date inválida (usa YYYY-MM-DD)")
		}
	}
	if visitTime != "" {
		if err := validateClockTime("visit_time", visitTime); err != nil {
			return model.Prospect{}, err
		}
	}
	schedulingVisit := !req.ClearVisitDate && (reqVisitDate != "" || reqVisitTime != "")
	if schedulingVisit {
		if visitDate == "" {
			return model.Prospect{}, fmt.Errorf("visit_date es obligatoria al agendar una visita")
		}
		if visitTime == "" {
			return model.Prospect{}, fmt.Errorf("visit_time es obligatoria al agendar una visita")
		}
	}
	if visitDate == "" {
		visitTime = ""
	}

	callDate := existing.CallDate
	callTime := existing.CallTime
	reqCallDate := strings.TrimSpace(req.CallDate)
	reqCallTime := strings.TrimSpace(req.CallTime)
	if req.ClearCallDate {
		callDate = ""
		callTime = ""
	} else {
		if reqCallDate != "" {
			callDate = reqCallDate
		}
		if reqCallTime != "" {
			callTime = reqCallTime
		}
	}
	if callDate != "" {
		if _, err := time.Parse("2006-01-02", callDate); err != nil {
			return model.Prospect{}, fmt.Errorf("call_date inválida (usa YYYY-MM-DD)")
		}
	}
	if callTime != "" {
		if err := validateClockTime("call_time", callTime); err != nil {
			return model.Prospect{}, err
		}
	}
	schedulingCall := !req.ClearCallDate && (reqCallDate != "" || reqCallTime != "")
	if schedulingCall {
		if callDate == "" {
			return model.Prospect{}, fmt.Errorf("call_date es obligatoria al agendar una llamada")
		}
		if callTime == "" {
			return model.Prospect{}, fmt.Errorf("call_time es obligatoria al agendar una llamada")
		}
	}
	if callDate == "" {
		callTime = ""
	}
	if callDate != "" && callTime != "" {
		if err := s.ensureAppointmentSlotFree(
			ctx,
			identity.UID,
			placeID,
			callDate,
			callTime,
			appointmentIntervalMinutes,
		); err != nil {
			return model.Prospect{}, err
		}
	}
	if visitDate != "" && visitTime != "" {
		if err := s.ensureAppointmentSlotFree(
			ctx,
			identity.UID,
			placeID,
			visitDate,
			visitTime,
			appointmentIntervalMinutes,
		); err != nil {
			return model.Prospect{}, err
		}
	}

	item := model.Prospect{
		PlaceID:            placeID,
		Name:               strings.TrimSpace(req.Name),
		Address:            strings.TrimSpace(req.Address),
		Phone:              strings.TrimSpace(req.Phone),
		Rating:             req.Rating,
		UserRatingCount:    req.UserRatingCount,
		GoogleMapsURI:      strings.TrimSpace(req.GoogleMapsURI),
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		OpenNow:            req.OpenNow,
		ContactStatus:      contactStatus,
		ContactOutcome:     outcome,
		ContactNotes:       notes,
		VisitDate:          visitDate,
		VisitTime:          visitTime,
		CallDate:           callDate,
		CallTime:           callTime,
		CallGoogleEventID:  existing.CallGoogleEventID,
		VisitGoogleEventID: existing.VisitGoogleEventID,
		CreatedAt:          createdAt,
		UpdatedAt:          now,
	}
	if callDate == "" {
		item.CallGoogleEventID = ""
	}
	if visitDate == "" {
		item.VisitGoogleEventID = ""
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
	// Set sin merge: campos omitempty vacíos se eliminan del documento.
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
		// Si ya no hay visita agendada, eliminar el espejo global de la cita.
		batch.Delete(globalRef)
	}

	if _, err := batch.Commit(ctx); err != nil {
		return model.Prospect{}, fmt.Errorf("guardar prospecto: %w", err)
	}
	return item, nil
}

func (s *Store) Get(ctx context.Context, uid, placeID string) (*model.Prospect, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return nil, fmt.Errorf("place_id vacío")
	}
	if uid == "" {
		return nil, fmt.Errorf("uid vacío")
	}
	doc, err := s.col(uid).Doc(sanitizePlaceID(placeID)).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("leer prospecto: %w", err)
	}
	var item model.Prospect
	if err := doc.DataTo(&item); err != nil {
		return nil, fmt.Errorf("parsear prospecto: %w", err)
	}
	return &item, nil
}

func (s *Store) PatchGoogleEventIDs(
	ctx context.Context,
	uid, placeID, callEventID, visitEventID string,
) error {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" || uid == "" {
		return fmt.Errorf("uid/place_id vacío")
	}
	ref := s.col(uid).Doc(sanitizePlaceID(placeID))
	updates := []firestore.Update{
		{Path: "updated_at", Value: time.Now().UTC()},
	}
	if strings.TrimSpace(callEventID) == "" {
		updates = append(updates, firestore.Update{Path: "call_google_event_id", Value: firestore.Delete})
	} else {
		updates = append(updates, firestore.Update{Path: "call_google_event_id", Value: callEventID})
	}
	if strings.TrimSpace(visitEventID) == "" {
		updates = append(updates, firestore.Update{Path: "visit_google_event_id", Value: firestore.Delete})
	} else {
		updates = append(updates, firestore.Update{Path: "visit_google_event_id", Value: visitEventID})
	}
	_, err := ref.Update(ctx, updates)
	if err != nil {
		return fmt.Errorf("guardar IDs de Calendar: %w", err)
	}
	return nil
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

func (s *Store) UpdateContactStatus(ctx context.Context, uid, placeID, contactStatus, outcome, notes string) error {
	placeID = strings.TrimSpace(placeID)
	contactStatus = strings.TrimSpace(contactStatus)
	outcome = strings.TrimSpace(outcome)
	notes = strings.TrimSpace(notes)
	if placeID == "" {
		return fmt.Errorf("place_id vacío")
	}
	if !model.ValidContactStatus(contactStatus) {
		return fmt.Errorf("contact_status inválido")
	}
	if contactStatus == model.ContactStatusNotContacted {
		outcome = ""
		notes = ""
	}
	if !model.ValidContactOutcome(outcome) {
		return fmt.Errorf("contact_outcome inválido")
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
		{Path: "contact_outcome", Value: outcome},
		{Path: "contact_notes", Value: notes},
		{Path: "updated_at", Value: time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("actualizar estado: %w", err)
	}
	return nil
}
