package emergency

import "context"

// SOSDomainHook persists incidents / care_events after SOS flows.
// Implemented in the care package and wired from cmd/server.
type SOSDomainHook interface {
	OnSOSAlert(
		ctx context.Context,
		userID, roomName string,
		lat, lng float64,
		familyNotified int,
	) error
	OnSOSFamilyEscalation(
		ctx context.Context,
		userID, roomName, contactName, status string,
		remaining []string,
	) error
}

type noopSOSHook struct{}

func (noopSOSHook) OnSOSAlert(
	context.Context, string, string, float64, float64, int,
) error {
	return nil
}

func (noopSOSHook) OnSOSFamilyEscalation(
	context.Context, string, string, string, string, []string,
) error {
	return nil
}
