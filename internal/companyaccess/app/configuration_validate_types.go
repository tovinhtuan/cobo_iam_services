package app

import "github.com/cobo/cobo_iam_services/internal/companyaccess/validation"

type ValidateConfigurationRequest struct {
	Subject AdminSubject
	Suites  []string
}

// ConfigurationValidateView is the API response for POST /api/v1/admin/configuration/validate.
type ConfigurationValidateView = validation.Result
