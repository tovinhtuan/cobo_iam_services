package timeline

import (
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
)

// Normalize maps one audit row to a TimelineEvent (read-only, no inference).
func Normalize(entry auditapp.Entry) TimelineEvent {
	meta := lookupAction(strings.TrimSpace(entry.Action))
	actor := Actor{
		UserID:       strings.TrimSpace(entry.ActorUserID),
		MembershipID: strings.TrimSpace(entry.ActorMembershipID),
	}
	if actor.UserID != "" {
		actor.Display = actor.UserID
	}
	ev := TimelineEvent{
		ID:           entry.EventID,
		OccurredAt:   entry.OccurredAt,
		Actor:        actor,
		Action:       entry.Action,
		Domain:       meta.Domain,
		ResourceType: strings.TrimSpace(entry.ResourceType),
		ResourceID:   strings.TrimSpace(entry.ResourceID),
		Summary:      meta.Summary,
		Source:       SourceAuditLogV1,
		Category:     meta.Category,
		MetadataSafe: SanitizeMetadata(entry.Metadata),
		ActionLink:   meta.ActionLink,
	}
	if entry.RequestID != "" {
		ev.CorrelationID = entry.RequestID
	}
	return ev
}

// NormalizeBatch maps audit rows to timeline events preserving order.
func NormalizeBatch(entries []auditapp.Entry) []TimelineEvent {
	out := make([]TimelineEvent, 0, len(entries))
	for _, e := range entries {
		out = append(out, Normalize(e))
	}
	return out
}

// FilterByDomain returns events matching domain (post-normalize).
func FilterByDomain(events []TimelineEvent, domain string) []TimelineEvent {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return events
	}
	out := make([]TimelineEvent, 0, len(events))
	for _, e := range events {
		if e.Domain == domain {
			out = append(out, e)
		}
	}
	return out
}
