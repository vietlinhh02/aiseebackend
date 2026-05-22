package care

import (
	"context"
	"fmt"

	"github.com/team-dang/aiseebackend/internal/emergency"
)

var _ emergency.SOSDomainHook = (*EmergencySOSHook)(nil)

// EmergencySOSHook implements emergency.SOSDomainHook using the care Store.
type EmergencySOSHook struct {
	Store Store
}

func NewEmergencySOSHook(store Store) emergency.SOSDomainHook {
	return &EmergencySOSHook{Store: store}
}

func (h *EmergencySOSHook) OnSOSAlert(
	ctx context.Context,
	userID, roomName string,
	lat, lng float64,
	familyNotified int,
) error {
	incidentID, err := h.Store.CreateIncidentSOS(ctx, userID, roomName, lat, lng, familyNotified)
	if err != nil {
		return err
	}
	summary := fmt.Sprintf("Family notified: %d", familyNotified)
	return h.Store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    userID,
		ActorRole: "user",
		EventType: "sos.triggered",
		Severity:  "critical",
		Title:     "SOS alert sent",
		Summary:   &summary,
		Metadata: map[string]any{
			"incident_id": incidentID,
			"room_name":   roomName,
			"lat":         lat,
			"lng":         lng,
		},
	})
}

func (h *EmergencySOSHook) OnSOSFamilyEscalation(
	ctx context.Context,
	userID, roomName, contactName, status string,
	remaining []string,
) error {
	incidentID, ok, err := h.Store.GetLatestOpenIncidentID(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	payload := map[string]any{
		"room_name":    roomName,
		"contact_name": contactName,
		"status":       status,
		"remaining":    remaining,
	}
	if err := h.Store.AppendIncidentEvent(ctx, incidentID, "sos.escalated_to_family", nil, payload); err != nil {
		return err
	}
	title := "SOS escalated to family"
	body := contactName
	return h.Store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    userID,
		ActorRole: "system",
		EventType: "sos.escalated_to_family",
		Severity:  "warning",
		Title:     title,
		Summary:   &body,
		Metadata: map[string]any{
			"incident_id": incidentID,
			"room_name":   roomName,
		},
	})
}
