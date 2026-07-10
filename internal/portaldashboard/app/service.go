package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
)

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type CompanyReader interface {
	GetCompanyBrief(ctx context.Context, companyID string) (name, code string, err error)
}

type Service interface {
	GetOverview(ctx context.Context, sub Subject, rangeInput domain.ParseRangeInput) (*domain.OverviewResponse, error)
}

type service struct {
	auth      authapp.Service
	deadlines deadlinealertsapp.Service
	adHoc     adhocapp.Service // nil when ad-hoc feature disabled
	inApp     inappapp.Service
	company   CompanyReader // optional
}

func NewService(
	auth authapp.Service,
	deadlines deadlinealertsapp.Service,
	adHoc adhocapp.Service,
	inApp inappapp.Service,
	company CompanyReader,
) Service {
	return &service{auth: auth, deadlines: deadlines, adHoc: adHoc, inApp: inApp, company: company}
}

type GetOverviewRequest struct {
	Subject Subject
	Range   domain.ParseRangeInput
}

func (s *service) GetOverview(ctx context.Context, sub Subject, rangeInput domain.ParseRangeInput) (*domain.OverviewResponse, error) {
	if strings.TrimSpace(sub.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	if err := s.authorizeDashboard(ctx, sub); err != nil {
		return nil, err
	}

	dr, err := domain.ParseRange(rangeInput)
	if err != nil {
		return nil, err
	}

	company := domain.CompanyBrief{ID: sub.CompanyID}
	if s.company != nil {
		name, code, err := s.company.GetCompanyBrief(ctx, sub.CompanyID)
		if err == nil {
			company.Name = name
			company.Code = code
		}
	}

	deadlineSub := deadlinealertsapp.Subject{
		UserID:       sub.UserID,
		MembershipID: sub.MembershipID,
		CompanyID:    sub.CompanyID,
	}

	deadlines, err := s.fetchDeadlines(ctx, deadlineSub, dr)
	if err != nil {
		return nil, err
	}

	adHoc := s.fetchAdHoc(ctx, adhocapp.Subject{
		UserID:       sub.UserID,
		MembershipID: sub.MembershipID,
		CompanyID:    sub.CompanyID,
	})

	inApp := s.fetchInApp(ctx, sub.UserID, sub.CompanyID)

	resp := buildOverview(company, dr, deadlines, adHoc, inApp)
	return &resp, nil
}

func (s *service) authorizeDashboard(ctx context.Context, sub Subject) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Action: "dashboard.view",
		Resource: authapp.ResourceRef{
			Type: "dashboard",
		},
	})
	if err != nil {
		return fmt.Errorf("authorize dashboard.view: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "dashboard.view permission required", nil)
	}
	return nil
}

func (s *service) fetchDeadlines(ctx context.Context, sub deadlinealertsapp.Subject, dr domain.DateRange) (deadlineFetch, error) {
	out := deadlineFetch{}
	list := func(status string) (*deadlinealertsapp.ListDeadlineAlertsResponse, error) {
		return s.deadlines.ListDeadlineAlerts(ctx, deadlinealertsapp.ListDeadlineAlertsRequest{
			Subject:   sub,
			Status:    status,
			StartDate: dr.From,
			EndDate:   dr.To,
			Page:      1,
			PageSize:  100,
		})
	}

	overdueResp, err := list("OVERDUE")
	if err != nil {
		return out, err
	}
	dueSoonResp, err := list("DUE_SOON")
	if err != nil {
		return out, err
	}
	pendingResp, err := list("PENDING_CONFIRM")
	if err != nil {
		return out, err
	}
	upcomingResp, err := list("UPCOMING")
	if err != nil {
		return out, err
	}

	start7, end7 := domain.Next7DaysWindow(dr)
	dueIn7 := 0
	for _, st := range []string{"OVERDUE", "DUE_SOON", "UPCOMING", "PENDING_CONFIRM"} {
		r, err := s.deadlines.ListDeadlineAlerts(ctx, deadlinealertsapp.ListDeadlineAlertsRequest{
			Subject:   sub,
			Status:    st,
			StartDate: start7,
			EndDate:   end7,
			Page:      1,
			PageSize:  1,
		})
		if err != nil {
			return out, err
		}
		dueIn7 += r.Total
	}

	out.overdue = overdueResp.Items
	out.dueSoon = dueSoonResp.Items
	out.pendingConfirm = pendingResp.Items
	out.upcoming = upcomingResp.Items
	out.overdueTotal = overdueResp.Total
	out.dueIn7Days = dueIn7
	return out, nil
}

func (s *service) fetchAdHoc(ctx context.Context, sub adhocapp.Subject) adHocFetch {
	out := adHocFetch{}
	if s.adHoc == nil {
		out.skipped = true
		out.warn = "ad_hoc feature not enabled"
		return out
	}
	list := func(status string) ([]adhocapp.ProposalDTO, int, error) {
		resp, err := s.adHoc.ListProposals(ctx, adhocapp.ListProposalsRequest{
			Subject:      sub,
			StatusFilter: []string{status},
			Page:         1,
			PageSize:     100,
		})
		if err != nil {
			return nil, 0, err
		}
		return resp.Items, resp.Total, nil
	}

	focal, focalTotal, err := list("pending_focal_approval")
	if err != nil {
		if isForbidden(err) {
			out.skipped = true
			out.warn = "ad_hoc proposals skipped: permission denied"
			return out
		}
		out.warn = "ad_hoc pending focal fetch failed"
		return out
	}
	admin, adminTotal, err := list("pending_admin_approval")
	if err != nil && !isForbidden(err) {
		out.warn = "ad_hoc pending admin fetch failed"
	}
	rejected, rejectedTotal, err := list("rejected")
	if err != nil && !isForbidden(err) {
		out.warn = "ad_hoc rejected fetch failed"
	}

	out.pendingFocal = focal
	out.pendingAdmin = admin
	out.rejected = rejected
	out.pendingTotal = focalTotal + adminTotal
	out.rejectedTotal = rejectedTotal
	return out
}

func (s *service) fetchInApp(ctx context.Context, userID, companyID string) inAppFetch {
	out := inAppFetch{}
	if s.inApp == nil {
		out.warn = "in-app notifications unavailable"
		return out
	}
	items, err := s.inApp.List(ctx, userID, companyID)
	if err != nil {
		out.err = err
		out.warn = "in-app notifications fetch failed"
		return out
	}
	out.items = items
	return out
}

func isForbidden(err error) bool {
	if he, ok := perr.AsHTTPError(err); ok {
		return he.HTTPStatus == http.StatusForbidden
	}
	return false
}

// adminCompanyReader adapts companyaccess AdminRepository for dashboard company brief.
type AdminCompanyReader struct {
	GetName func(ctx context.Context, companyID string) (string, error)
	GetCode func(ctx context.Context, companyID string) (string, error)
}

func (r AdminCompanyReader) GetCompanyBrief(ctx context.Context, companyID string) (string, string, error) {
	var name, code string
	var err error
	if r.GetName != nil {
		name, err = r.GetName(ctx, companyID)
		if err != nil {
			return "", "", err
		}
	}
	if r.GetCode != nil {
		code, _ = r.GetCode(ctx, companyID)
	}
	return name, code, nil
}
