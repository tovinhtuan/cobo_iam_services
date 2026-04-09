package inmemory

import (
	"context"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type MembershipQueryService struct {
	ByUser map[string][]caapp.MembershipView
	Roles  map[string][]string
	Deps   map[string][]caapp.DepartmentView
	Titles map[string][]string
}

func NewMembershipQueryService() *MembershipQueryService {
	// Bootstrap fixture dataset for P0.4 flow testing.
	return &MembershipQueryService{
		ByUser: map[string][]caapp.MembershipView{
			"u_123": {
				{MembershipID: "m_001", UserID: "u_123", CompanyID: "c_001", CompanyName: "Company X", Status: "active"},
				{MembershipID: "m_002", UserID: "u_123", CompanyID: "c_002", CompanyName: "Company Y", Status: "active"},
			},
			"u_single": {
				{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo Company", Status: "active"},
			},
		},
		Roles: map[string][]string{
			"m_001": {"department_staff", "disclosure_approver"},
		},
		Deps: map[string][]caapp.DepartmentView{
			"m_001": {
				{DepartmentID: "d_legal", DepartmentName: "Legal"},
				{DepartmentID: "d_ir", DepartmentName: "IR"},
			},
		},
		Titles: map[string][]string{
			"m_001": {"Dau moi CBTT"},
		},
	}
}

func (s *MembershipQueryService) GetMembershipsByUser(_ context.Context, userID string) ([]caapp.MembershipView, error) {
	items := s.ByUser[userID]
	out := make([]caapp.MembershipView, len(items))
	copy(out, items)
	return out, nil
}

func (s *MembershipQueryService) GetActiveMembership(_ context.Context, userID, companyID string) (*caapp.MembershipView, error) {
	for _, m := range s.ByUser[userID] {
		if m.CompanyID == companyID && strings.EqualFold(m.Status, "active") {
			cp := m
			return &cp, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeMembershipNotFound, "membership not found", nil)
}

func (s *MembershipQueryService) GetMembershipRoles(_ context.Context, membershipID string) ([]string, error) {
	out := append([]string(nil), s.Roles[membershipID]...)
	return out, nil
}

func (s *MembershipQueryService) GetMembershipDepartments(_ context.Context, membershipID string) ([]caapp.DepartmentView, error) {
	items := s.Deps[membershipID]
	out := make([]caapp.DepartmentView, len(items))
	copy(out, items)
	return out, nil
}

func (s *MembershipQueryService) GetMembershipTitles(_ context.Context, membershipID string) ([]string, error) {
	out := append([]string(nil), s.Titles[membershipID]...)
	return out, nil
}
