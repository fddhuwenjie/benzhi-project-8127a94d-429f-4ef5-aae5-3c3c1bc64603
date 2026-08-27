package evidence

import (
	"time"

	"oralhistory/internal/domain"
)

func NewAuditEvent(caseID, eventType, actorID, requestID, requestDigest string, revision int64, payload any, now time.Time) domain.AuditEvent {
	return domain.AuditEvent{
		CaseID:        caseID,
		Type:          eventType,
		ActorID:       actorID,
		Revision:      revision,
		RequestID:     requestID,
		RequestSHA256: requestDigest,
		Payload:       payload,
		OccurredAt:    now.UTC(),
	}
}

type TimelineItem struct {
	Sequence    int64     `json:"sequence"`
	Type        string    `json:"type"`
	ActorID     string    `json:"actor_id"`
	Revision    int64     `json:"revision"`
	OccurredAt  time.Time `json:"occurred_at"`
	EventSHA256 string    `json:"event_sha256"`
}

func BuildTimeline(events []domain.AuditEvent) []TimelineItem {
	result := make([]TimelineItem, 0, len(events))
	for _, event := range events {
		result = append(result, TimelineItem{
			Sequence:    event.Sequence,
			Type:        event.Type,
			ActorID:     event.ActorID,
			Revision:    event.Revision,
			OccurredAt:  event.OccurredAt,
			EventSHA256: event.EventSHA256,
		})
	}
	return result
}
