package care

// Event types aligned with care-dashboard-feature-roadmap §10.
var knownCareEventTypes = map[string]struct{}{
	"medication.created": {}, "medication.updated": {}, "medication.deleted": {},
	"medication.taken": {}, "medication.skipped": {}, "medication.refill.low": {},
	"health.symptom.logged": {}, "health.metric.logged": {}, "health.pattern.detected": {},
	"ai.session.started": {}, "ai.session.ended": {}, "ai.question.asked": {},
	"ai.camera.activated": {}, "ai.camera.deactivated": {},
	"call.voice.started": {}, "call.voice.ended": {}, "call.video.started": {},
	"call.video.ended": {}, "call.missed": {},
	"sos.triggered": {}, "sos.family_notified": {}, "sos.escalated_to_family": {}, "sos.family_responded": {},
	"sos.volunteer_escalated": {}, "sos.resolved": {},
	"location.updated": {}, "location.stale": {},
	"task.created": {}, "task.completed": {}, "task.overdue": {},
	"brief.daily.created": {}, "report.weekly.created": {}, "report.pdf.exported": {},
	"alert.created": {}, "alert.acknowledged": {}, "alert.resolved": {}, "alert.dismissed": {},
	"volunteer.matched": {}, "volunteer.rated": {}, "volunteer.trusted": {}, "volunteer.blocked": {},
}

func IsKnownCareEventType(eventType string) bool {
	_, ok := knownCareEventTypes[eventType]
	return ok
}
