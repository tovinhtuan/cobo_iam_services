package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// GetObjectDependencies returns reverse dependency lookup (ADR-043, Sprint 4 Batch 3).
func (s *adminService) GetObjectDependencies(ctx context.Context, req GetObjectDependenciesRequest) (*dependency.Result, error) {
	if err := s.authorizeConfigurationHealth(ctx, req.Subject); err != nil {
		return nil, err
	}
	companyID := strings.TrimSpace(req.Subject.CompanyID)
	if companyID == "" {
		return nil, perrNewBadRequest("company context required")
	}
	objectType := strings.TrimSpace(strings.ToLower(req.ObjectType))
	objectID := strings.TrimSpace(req.ObjectID)
	if objectID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "object_id is required", nil)
	}
	if objectType != dependency.ObjectTypeDepartment && objectType != dependency.ObjectTypeRole {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "object_type must be department or role", nil)
	}

	sampleLimit := req.SampleLimit
	if sampleLimit <= 0 {
		sampleLimit = dependency.DefaultSampleLimit
	}
	if sampleLimit > dependency.MaxSampleLimit {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "sample_limit must be between 1 and 20", nil)
	}
	includeCounts := req.IncludeCounts
	if !req.IncludeCountsSet {
		includeCounts = true
	}

	provider := dependency.Provider{
		Loader: conflict.SnapshotLoader{
			Reader:                   s.conflictReader,
			CompanyTierLookup:        s.companyTierLookup,
			SubscriptionTierEnforced: s.subscriptionTierEnforcementEnabled,
		},
		Reader: s.dependencyReader,
	}
	out, err := provider.Resolve(ctx, dependency.Query{
		CompanyID:     companyID,
		ObjectType:    objectType,
		ObjectID:      objectID,
		SampleLimit:   sampleLimit,
		IncludeCounts: includeCounts,
		EvaluatedAt:   time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, dependency.ErrInvalidObjectType) {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
		}
		if errors.Is(err, dependency.ErrObjectNotFound) {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "object not found", nil)
		}
		return nil, err
	}
	return out, nil
}

// ParseObjectDependenciesQuery parses optional query params for dependency API.
func ParseObjectDependenciesQuery(sampleLimitRaw string, includeCountsRaw string) (sampleLimit int, includeCounts bool, err error) {
	includeCounts = true
	if includeCountsRaw != "" {
		v, parseErr := strconv.ParseBool(strings.TrimSpace(includeCountsRaw))
		if parseErr != nil {
			return 0, false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "include_counts must be true or false", nil)
		}
		includeCounts = v
	}
	if sampleLimitRaw == "" {
		return dependency.DefaultSampleLimit, includeCounts, nil
	}
	n, parseErr := strconv.Atoi(strings.TrimSpace(sampleLimitRaw))
	if parseErr != nil {
		return 0, false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "sample_limit must be an integer", nil)
	}
	return n, includeCounts, nil
}
