package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type service struct {
	resolver Resolver
	checker  Checker
	repo     Repository
}

func NewService(resolver Resolver, checker Checker, repo Repository) Service {
	return &service{resolver: resolver, checker: checker, repo: repo}
}

func (s *service) Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeDecision, error) {
	if strings.TrimSpace(req.Subject.MembershipID) == "" || strings.TrimSpace(req.Subject.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "membership_id and company_id are required", nil)
	}
	eff, err := s.resolver.Resolve(ctx, req.Subject.MembershipID, req.Subject.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("resolve effective access: %w", err)
	}
	policy, err := s.repo.GetActionPolicy(ctx, req.Subject.CompanyID, req.Action)
	if err != nil {
		return nil, fmt.Errorf("get action policy: %w", err)
	}
	decision, err := s.checker.Check(ctx, req, eff, policy)
	if err != nil {
		return nil, fmt.Errorf("check authorization: %w", err)
	}
	return decision, nil
}

func (s *service) AuthorizeBatch(ctx context.Context, req AuthorizeBatchRequest) (*AuthorizeBatchResponse, error) {
	if strings.TrimSpace(req.Subject.MembershipID) == "" || strings.TrimSpace(req.Subject.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "membership_id and company_id are required", nil)
	}
	eff, err := s.resolver.Resolve(ctx, req.Subject.MembershipID, req.Subject.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("resolve effective access: %w", err)
	}
	out := &AuthorizeBatchResponse{Results: make([]AuthorizeDecision, 0, len(req.Checks))}
	for _, c := range req.Checks {
		policy, err := s.repo.GetActionPolicy(ctx, req.Subject.CompanyID, c.Action)
		if err != nil {
			return nil, fmt.Errorf("get action policy (batch): %w", err)
		}
		d, err := s.checker.Check(ctx, AuthorizeRequest{Subject: req.Subject, Action: c.Action, Resource: c.Resource}, eff, policy)
		if err != nil {
			return nil, fmt.Errorf("batch check: %w", err)
		}
		out.Results = append(out.Results, *d)
	}
	return out, nil
}

func (s *service) GetEffectiveAccess(ctx context.Context, membershipID, companyID string) (*EffectiveAccessSummary, error) {
	if strings.TrimSpace(membershipID) == "" || strings.TrimSpace(companyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "membership_id and company_id are required", nil)
	}
	return s.resolver.Resolve(ctx, membershipID, companyID)
}
