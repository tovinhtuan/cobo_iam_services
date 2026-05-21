package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *adminService) ListTitles(ctx context.Context, req ListTitlesRequest) ([]TitleView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.ListCompanyTitles(ctx, req.Subject.CompanyID)
}

func (s *adminService) CreateTitle(ctx context.Context, req CreateTitleRequest) (*TitleView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	titleID := s.idg.NewUUID()
	titleCode := s.idg.NewUUID() // opaque, unique-enough; not exposed in API
	return s.repo.CreateTitleRow(ctx, req.Subject.CompanyID, titleID, titleCode, name, req.SortOrder)
}

func (s *adminService) UpdateTitle(ctx context.Context, req UpdateTitleRequest) (*TitleView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TitleID = strings.TrimSpace(req.TitleID)
	if req.TitleID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title_id is required", nil)
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name cannot be empty", nil)
		}
		req.Name = &trimmed
	}
	return s.repo.PatchTitleRow(ctx, req.Subject.CompanyID, req.TitleID, req.Name, req.SortOrder, req.Status)
}

func (s *adminService) DeleteTitle(ctx context.Context, req DeleteTitleRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	req.TitleID = strings.TrimSpace(req.TitleID)
	if req.TitleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title_id is required", nil)
	}
	n, err := s.repo.CountTitleMembers(ctx, req.TitleID)
	if err != nil {
		return err
	}
	if n > 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "TITLE_HAS_MEMBERS", nil)
	}
	return s.repo.SoftDeleteTitle(ctx, req.TitleID, req.Subject.CompanyID)
}

func (s *adminService) AddTitleMember(ctx context.Context, req AddTitleMemberRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	if err := s.requireMembershipInCompany(ctx, req.MembershipID, req.Subject.CompanyID); err != nil {
		return err
	}
	// AddTitle validates title belongs to membership's company.
	return s.repo.AddTitle(ctx, req.MembershipID, req.TitleID)
}

func (s *adminService) RemoveTitleMember(ctx context.Context, req RemoveTitleMemberRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	return s.repo.RemoveTitle(ctx, req.MembershipID, req.TitleID)
}
