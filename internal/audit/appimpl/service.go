package appimpl

import (
	"context"
	"fmt"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/platform/clock"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type Service struct {
	repo  auditapp.Repository
	clock clock.Clock
	idgen idgen.Generator
}

func NewService(repo auditapp.Repository, clk clock.Clock, id idgen.Generator) auditapp.Service {
	return &Service{repo: repo, clock: clk, idgen: id}
}

func (s *Service) AppendAuditLog(ctx context.Context, req auditapp.AppendAuditLogRequest) error {
	if req.EventID == "" {
		req.EventID = s.idgen.NewUUID()
	}
	if req.OccurredAt == "" {
		req.OccurredAt = s.clock.Now().Format(time.RFC3339)
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	if err := s.repo.Append(ctx, req); err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}
