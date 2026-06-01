package app

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

type svc struct {
	repo          Repository
	userIDQuerier UserIDQuerier
	log           *slog.Logger
}

// NewService creates an InAppNotification service.
// userIDQuerier may be nil — if so, reminder-sourced notifications are skipped.
func NewService(repo Repository, userIDQuerier UserIDQuerier, log *slog.Logger) Service {
	if log == nil {
		log = slog.Default()
	}
	return &svc{repo: repo, userIDQuerier: userIDQuerier, log: log}
}

// CreateForReminder creates one notification record per resolved user_id.
// It resolves user_ids from RecipientEmails via UserIDQuerier.
// Errors are logged but never propagate — caller (reminder worker) is fire-and-forget.
func (s *svc) CreateForReminder(ctx context.Context, req ReminderInAppRequest) error {
	if s.userIDQuerier == nil || len(req.RecipientEmails) == 0 {
		return nil
	}
	userIDs, err := s.userIDQuerier.UserIDsByEmails(ctx, req.CompanyID, req.RecipientEmails)
	if err != nil {
		s.log.Warn("inappnotification: resolve user IDs for reminder failed",
			slog.String("company_id", req.CompanyID), slog.String("err", err.Error()))
		return nil
	}
	var rt *string
	var ri *string
	if req.ResourceType != "" {
		rt = ptr(req.ResourceType)
	}
	if req.ResourceID != "" {
		ri = ptr(req.ResourceID)
	}
	for _, uid := range userIDs {
		if uid == "" {
			continue
		}
		n := InAppNotification{
			ID:           newID(),
			UserID:       uid,
			CompanyID:    req.CompanyID,
			Kind:         req.Kind,
			Title:        req.Title,
			Body:         req.Body,
			ResourceType: rt,
			ResourceID:   ri,
			IsRead:       false,
			CreatedAt:    time.Now().UTC(),
		}
		if createErr := s.repo.Create(ctx, n); createErr != nil {
			s.log.Warn("inappnotification: create failed",
				slog.String("user_id", uid), slog.String("err", createErr.Error()))
		}
	}
	return nil
}

// CreateForUser creates a single in-app notification for a known user_id.
func (s *svc) CreateForUser(ctx context.Context, userID, companyID, kind, title, body string, resourceType, resourceID *string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(kind) == "" {
		return nil
	}
	n := InAppNotification{
		ID:           newID(),
		UserID:       strings.TrimSpace(userID),
		CompanyID:    strings.TrimSpace(companyID),
		Kind:         kind,
		Title:        title,
		Body:         body,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IsRead:       false,
		CreatedAt:    time.Now().UTC(),
	}
	return s.repo.Create(ctx, n)
}

func (s *svc) List(ctx context.Context, userID, companyID string) ([]InAppNotification, error) {
	return s.repo.ListByUser(ctx, userID, companyID, listLimit)
}

func (s *svc) UnreadCount(ctx context.Context, userID, companyID string) (int, error) {
	return s.repo.UnreadCount(ctx, userID, companyID)
}

func (s *svc) MarkRead(ctx context.Context, userID, notifID string) error {
	return s.repo.MarkRead(ctx, userID, notifID)
}

func (s *svc) MarkAllRead(ctx context.Context, userID, companyID string) error {
	return s.repo.MarkAllRead(ctx, userID, companyID)
}

func newID() string {
	return "uin_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func ptr(s string) *string { return &s }
