package app

import "github.com/cobo/cobo_iam_services/internal/companyaccess/dashboard"

type GetOperationalDashboardRequest struct {
	Subject AdminSubject
}

// OperationalDashboardView is the API response alias.
type OperationalDashboardView = dashboard.Result
