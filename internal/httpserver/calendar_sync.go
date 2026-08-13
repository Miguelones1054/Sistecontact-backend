package httpserver

import (
	"context"
	"log/slog"

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
	if h.gcalClient == nil || !h.gcalClient.Enabled() {
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

	callID := after.CallGoogleEventID
	if callID == "" {
		callID = beforeCallID
	}
	visitID := after.VisitGoogleEventID
	if visitID == "" {
		visitID = beforeVisitID
	}

	// Llamada
	if after.CallDate == "" || after.CallTime == "" {
		if beforeCallID != "" {
			if err := h.gcalClient.DeleteEvent(ctx, uid, beforeCallID); err != nil {
				slog.Warn("calendar delete call", "uid", uid, "err", err)
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
			callID = beforeCallID
		} else if id != "" {
			callID = id
		}
	}

	// Visita
	if after.VisitDate == "" || after.VisitTime == "" {
		if beforeVisitID != "" {
			if err := h.gcalClient.DeleteEvent(ctx, uid, beforeVisitID); err != nil {
				slog.Warn("calendar delete visit", "uid", uid, "err", err)
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
			visitID = beforeVisitID
		} else if id != "" {
			visitID = id
		}
	}

	after.CallGoogleEventID = callID
	after.VisitGoogleEventID = visitID

	changed := false
	if before == nil {
		changed = callID != "" || visitID != ""
	} else {
		changed = callID != before.CallGoogleEventID || visitID != before.VisitGoogleEventID
	}
	if changed {
		if err := h.prospects.PatchGoogleEventIDs(ctx, uid, after.PlaceID, callID, visitID); err != nil {
			slog.Warn("calendar patch ids", "uid", uid, "place", after.PlaceID, "err", err)
		}
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
