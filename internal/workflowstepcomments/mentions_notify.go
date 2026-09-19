package workflowstepcomments

import (
	"context"
	"log/slog"
	"strings"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

const (
	mentionNotifyTitle = "Bạn được nhắc trong trao đổi"
	mentionNotifyBody  = "Bạn được nhắc trong trao đổi của một bước công việc."

	logEventMentionNotify = "workflow.step_comment.mentioned"
)

// MentionInAppNotifier is the CreateForUser surface used after commit (optional).
type MentionInAppNotifier interface {
	CreateForUser(ctx context.Context, userID, companyID, kind, title, body string, resourceType, resourceID *string) error
}

// MembershipUserResolver maps an active company membership to its user_id.
type MembershipUserResolver interface {
	ResolveActiveUser(ctx context.Context, companyID, membershipID string) (userID string, ok bool, err error)
}

func (s *Service) WithMentionNotifier(n MentionInAppNotifier) *Service {
	s.mentionNotifier = n
	return s
}

func (s *Service) WithMembershipUserResolver(r MembershipUserResolver) *Service {
	s.membershipUsers = r
	return s
}

type mentionNotifyContext struct {
	CompanyID          string
	CommentID          string
	RecordID           string
	WorkflowInstanceID string
	StepCode           string
	AuthorUserID       string
	AuthorMembershipID string
}

// notifyCommentMentionRecipients runs AFTER successful comment+mentions commit.
// Fail-closed on recipient authz; never includes comment body; never rolls back comment.
func (s *Service) notifyCommentMentionRecipients(
	ctx context.Context,
	meta mentionNotifyContext,
	membershipIDs []string,
) {
	if !s.mentionsEnabled || s.mentionNotifier == nil || len(membershipIDs) == 0 {
		return
	}

	seenMembership := map[string]struct{}{}
	seenUser := map[string]struct{}{}

	for _, mid := range membershipIDs {
		mid = strings.TrimSpace(mid)
		if mid == "" {
			continue
		}
		if _, ok := seenMembership[mid]; ok {
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, "", mid, "duplicate_membership")
			continue
		}
		seenMembership[mid] = struct{}{}

		if mid == strings.TrimSpace(meta.AuthorMembershipID) {
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, "", mid, "self_mention")
			continue
		}

		userID, ok, err := s.resolveMentionRecipientUser(ctx, meta.CompanyID, mid)
		if err != nil {
			s.logMentionNotify("workflow_step_comment_mention_notification_failed", meta, "", mid, "resolve_user_error")
			s.log.Error(logEventMentionNotify,
				slog.String("event", logEventMentionNotify),
				slog.String("comment_id", meta.CommentID),
				slog.String("company_id", meta.CompanyID),
				slog.String("reason", "resolve_user_error"),
				slog.String("err", err.Error()),
			)
			continue
		}
		if !ok || userID == "" {
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, "", mid, "unresolvable")
			continue
		}
		if userID == strings.TrimSpace(meta.AuthorUserID) {
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, userID, mid, "self_mention")
			continue
		}
		if _, ok := seenUser[userID]; ok {
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, userID, mid, "duplicate_user")
			continue
		}
		seenUser[userID] = struct{}{}

		recipient := wff.Subject{
			UserID:       userID,
			MembershipID: mid,
			CompanyID:    meta.CompanyID,
		}
		if !s.recipientCanViewMentionResource(ctx, recipient, meta.RecordID, meta.StepCode) {
			// Fail-closed: mention does not grant access; skip rather than leak body/context.
			s.logMentionNotify("workflow_step_comment_mention_notification_skipped", meta, userID, mid, "recipient_cannot_view")
			continue
		}

		s.logMentionNotify("workflow_step_comment_mention_notification_attempted", meta, userID, mid, "")
		resType := inappapp.ResourceTypeDisclosure
		resID := meta.RecordID
		err = s.mentionNotifier.CreateForUser(
			ctx,
			userID,
			meta.CompanyID,
			inappapp.KindWorkflowStepCommentMentioned,
			mentionNotifyTitle,
			mentionNotifyBody,
			&resType,
			&resID,
		)
		if err != nil {
			s.logMentionNotify("workflow_step_comment_mention_notification_failed", meta, userID, mid, "create_failed")
			s.log.Error(logEventMentionNotify,
				slog.String("event", logEventMentionNotify),
				slog.String("comment_id", meta.CommentID),
				slog.String("company_id", meta.CompanyID),
				slog.String("recipient_user_id", userID),
				slog.String("err", err.Error()),
			)
			continue
		}
		s.logMentionNotify("workflow_step_comment_mention_notification_succeeded", meta, userID, mid, "")
	}
}

func (s *Service) resolveMentionRecipientUser(ctx context.Context, companyID, membershipID string) (string, bool, error) {
	if s.membershipUsers == nil {
		return "", false, nil
	}
	return s.membershipUsers.ResolveActiveUser(ctx, companyID, membershipID)
}

// recipientCanViewMentionResource requires deadline.view + record data scope + resolvable step.
func (s *Service) recipientCanViewMentionResource(ctx context.Context, recipient wff.Subject, recordID, stepCode string) bool {
	if err := s.deadline.AuthorizeView(ctx, recipient); err != nil {
		return false
	}
	if _, _, _, err := s.loadStep(ctx, recipient, recordID, stepCode); err != nil {
		return false
	}
	return true
}

func membershipIDsFromCanonical(canonical []CanonicalMention) []string {
	out := make([]string, 0, len(canonical))
	for _, m := range canonical {
		out = append(out, m.MembershipID)
	}
	return out
}

func membershipIDsFromMentions(rows []Mention) map[string]struct{} {
	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		id := strings.TrimSpace(r.MentionedMembershipID)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

// newMentionMembershipIDs returns memberships in newCanonical that were not in oldSet.
func newMentionMembershipIDs(newCanonical []CanonicalMention, oldSet map[string]struct{}) []string {
	if oldSet == nil {
		oldSet = map[string]struct{}{}
	}
	out := make([]string, 0)
	seen := map[string]struct{}{}
	for _, m := range newCanonical {
		id := strings.TrimSpace(m.MembershipID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, existed := oldSet[id]; existed {
			continue
		}
		out = append(out, id)
	}
	return out
}

func (s *Service) logMentionNotify(metric string, meta mentionNotifyContext, recipientUserID, membershipID, reason string) {
	attrs := []any{
		slog.String("metric", metric),
		slog.String("event", logEventMentionNotify),
		slog.String("company_id", meta.CompanyID),
		slog.String("comment_id", meta.CommentID),
	}
	if recipientUserID != "" {
		attrs = append(attrs, slog.String("recipient_user_id", recipientUserID))
	}
	if membershipID != "" {
		attrs = append(attrs, slog.String("membership_id", membershipID))
	}
	if reason != "" {
		attrs = append(attrs, slog.String("reason", reason))
	}
	s.log.Info(metric, attrs...)
}
