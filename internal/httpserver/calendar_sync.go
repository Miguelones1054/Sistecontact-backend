package httpserver

import (
	"context"
	"log/slog"
	"strings"

	"github.com/sistecontact/api/internal/googlecalendar"
	"github.com/sistecontact/api/internal/model"
)

func (h *Handler) syncProspectGoogleCalendar(
	ctx context.Context,
	uid string,
	before *model.Prospect,
	after model.Prospect,
	durationMinutes int,
) model.Prospect {
	after.CalendarSyncStatus = ""
	after.CalendarSyncError = ""

	if h.gcalClient == nil || !h.gcalClient.Enabled() {
		return after
	}

	scheduleTouched := false
	if before == nil {
		scheduleTouched = after.CallDate != "" || after.VisitDate != ""
	} else {
		scheduleTouched =
			after.CallDate != before.CallDate ||
				after.CallTime != before.CallTime ||
				after.VisitDate != before.VisitDate ||
				after.VisitTime != before.VisitTime ||
				(after.CallDate == "" && before.CallGoogleEventID != "") ||
				(after.VisitDate == "" && before.VisitGoogleEventID != "")
	}
	if !scheduleTouched {
		return after
	}

	var beforeCallID, beforeVisitID string
	var beforeCallDate, beforeCallTime, beforeVisitDate, beforeVisitTime string
	if before != nil {
		beforeCallID = before.CallGoogleEventID
		beforeVisitID = before.VisitGoogleEventID
		beforeCallDate = before.CallDate
		beforeCallTime = before.CallTime
		beforeVisitDate = before.VisitDate
		beforeVisitTime = before.VisitTime
	}

	callID := beforeCallID
	visitID := beforeVisitID

	var syncErrs []string
	syncedAny := false
	disconnected := false

	// Llamada
	if after.CallDate == "" || after.CallTime == "" {
		if beforeCallID != "" {
			if err := h.gcalClient.DeleteEvent(ctx, uid, beforeCallID); err != nil {
				slog.Warn("calendar delete call", "uid", uid, "err", err)
				syncErrs = append(syncErrs, err.Error())
			} else {
				syncedAny = true
			}
		}
		callID = ""
	} else if after.CallDate != beforeCallDate ||
		after.CallTime != beforeCallTime ||
		beforeCallID == "" {
		id, err := h.gcalClient.UpsertEvent(ctx, uid, googlecalendar.AppointmentEvent{
			Kind:            googlecalendar.KindCall,
			BusinessName:    after.Name,
			Address:         after.Address,
			Phone:           after.Phone,
			Date:            after.CallDate,
			Time:            after.CallTime,
			DurationMinutes: durationMinutes,
			TimeZone:        h.calendarTZ,
			ExistingEventID: beforeCallID,
		})
		if err != nil {
			slog.Warn("calendar upsert call", "uid", uid, "place", after.PlaceID, "err", err)
			syncErrs = append(syncErrs, err.Error())
		} else if id == "" {
			disconnected = true
		} else {
			callID = id
			syncedAny = true
			slog.Info("calendar call synced", "uid", uid, "place", after.PlaceID, "event_id", id)
		}
	}

	// Visita
	if after.VisitDate == "" || after.VisitTime == "" {
		if beforeVisitID != "" {
			if err := h.gcalClient.DeleteEvent(ctx, uid, beforeVisitID); err != nil {
				slog.Warn("calendar delete visit", "uid", uid, "err", err)
				syncErrs = append(syncErrs, err.Error())
			} else {
				syncedAny = true
			}
		}
		visitID = ""
	} else if after.VisitDate != beforeVisitDate ||
		after.VisitTime != beforeVisitTime ||
		beforeVisitID == "" {
		id, err := h.gcalClient.UpsertEvent(ctx, uid, googlecalendar.AppointmentEvent{
			Kind:            googlecalendar.KindVisit,
			BusinessName:    after.Name,
			Address:         after.Address,
			Phone:           after.Phone,
			Date:            after.VisitDate,
			Time:            after.VisitTime,
			DurationMinutes: durationMinutes,
			TimeZone:        h.calendarTZ,
			ExistingEventID: beforeVisitID,
		})
		if err != nil {
			slog.Warn("calendar upsert visit", "uid", uid, "place", after.PlaceID, "err", err)
			syncErrs = append(syncErrs, err.Error())
		} else if id == "" {
			disconnected = true
		} else {
			visitID = id
			syncedAny = true
			slog.Info("calendar visit synced", "uid", uid, "place", after.PlaceID, "event_id", id)
		}
	}

	after.CallGoogleEventID = callID
	after.VisitGoogleEventID = visitID

	if callID != beforeCallID || visitID != beforeVisitID {
		if err := h.prospects.PatchGoogleEventIDs(ctx, uid, after.PlaceID, callID, visitID); err != nil {
			slog.Warn("calendar patch ids", "uid", uid, "place", after.PlaceID, "err", err)
			syncErrs = append(syncErrs, err.Error())
		}
	}

	switch {
	case len(syncErrs) > 0:
		after.CalendarSyncStatus = "error"
		after.CalendarSyncError = strings.Join(syncErrs, " | ")
	case disconnected:
		after.CalendarSyncStatus = "skipped"
		after.CalendarSyncError = "Google Calendar no está conectado en Mi perfil"
	case syncedAny:
		after.CalendarSyncStatus = "synced"
	}

	return after
}

func (h *Handler) deleteProspectGoogleCalendar(ctx context.Context, uid string, item *model.Prospect) {
	if h.gcalClient == nil || !h.gcalClient.Enabled() || item == nil {
		return
	}
	if item.CallGoogleEventID != "" {
		if err := h.gcalClient.DeleteEvent(ctx, uid, item.CallGoogleEventID); err != nil {
			slog.Warn("calendar delete call on prospect remove", "uid", uid, "err", err)
		}
	}
	if item.VisitGoogleEventID != "" {
		if err := h.gcalClient.DeleteEvent(ctx, uid, item.VisitGoogleEventID); err != nil {
			slog.Warn("calendar delete visit on prospect remove", "uid", uid, "err", err)
		}
	}
}
