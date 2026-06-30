package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
)

// selfServiceCompanyQuotaLimit returns max self-provisioned companies; 0 means unlimited (Enterprise).
func selfServiceCompanyQuotaLimit(tier string) int {
	switch strings.TrimSpace(tier) {
	case "Premium":
		return 3
	case "Enterprise":
		return 0
	default:
		return 1
	}
}

func (s *adminService) subscriptionTier(ctx context.Context, userID string) string {
	if s.tierLookup == nil {
		return ""
	}
	return s.tierLookup(ctx, userID)
}

func (s *adminService) entitlementChecker() entitlement.Checker {
	return entitlement.Checker{
		Enabled:         s.subscriptionTierEnforcementEnabled,
		ResolveUserTier: s.tierLookup,
	}
}

func displaySubscriptionTier(tier string) string {
	if strings.TrimSpace(tier) == "" {
		return "Free"
	}
	return strings.TrimSpace(tier)
}

func logCreateSelfServiceSuccess(ctx context.Context, userID string, boot *BootstrapSelfServiceResult, limit int, tier string) {
	current := boot.SelfProvisionedCount
	slog.InfoContext(ctx, "company self-service provision",
		slog.String("user_id", userID),
		slog.String("company_id", boot.CompanyID),
		slog.String("membership_id", boot.MembershipID),
		slog.String("idempotency_scope", "company.create"),
		slog.String("provisioning_source", ProvisioningSourceSelfServiceCreate),
		slog.Int("quota_limit", limit),
		slog.Int("quota_current", current),
		slog.String("quota_tier", displaySubscriptionTier(tier)),
	)
}
