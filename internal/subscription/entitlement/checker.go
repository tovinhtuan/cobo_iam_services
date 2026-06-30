package entitlement

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Feature keys for subscription entitlement checks (Batch 5).
const (
	FeatureNotificationRulesMutation  = "notification_rules.mutation"
	FeatureNotificationRulesRuntime   = "notification_rules.runtime"
	FeatureNotificationRulesSimulation = "notification_rules.simulation"
)

// Tier constants (IAM user_subscription_tiers values).
const (
	TierFree       = "Free"
	TierPremium    = "Premium"
	TierEnterprise = "Enterprise"
)

// Reason codes (stable for logs/API).
const (
	ReasonAllowed                      = ""
	ReasonSubscriptionTierDenied       = "SUBSCRIPTION_TIER_DENIED"
	ReasonSubscriptionTierUnknown      = "SUBSCRIPTION_TIER_UNKNOWN"
	ReasonSubscriptionFeatureNotConfig = "SUBSCRIPTION_FEATURE_NOT_CONFIGURED"
)

// TierResolver returns subscription tier for a user. Empty means unknown.
type TierResolver func(ctx context.Context, userID string) string

// CompanyTierResolver returns effective subscription tier for a company at runtime dispatch.
// Empty means unknown — premium schedules are skipped when enforcement is ON.
type CompanyTierResolver func(ctx context.Context, companyID string) string

// Checker evaluates subscription entitlements server-side.
type Checker struct {
	Enabled              bool
	ResolveUserTier      TierResolver
	ResolveCompanyTier   CompanyTierResolver
}

// Result is the outcome of an entitlement check.
type Result struct {
	Allowed      bool
	Tier         string
	RequiredTier string
	ReasonCode   string
	Feature      string
}

// Check evaluates whether the actor may use a feature.
func (c Checker) Check(ctx context.Context, userID, feature string) Result {
	res := Result{Allowed: true, Feature: feature}
	if !c.Enabled {
		return res
	}
	tier := c.normalizeTier(c.ResolveUserTier(ctx, userID))
	res.Tier = tier
	required := requiredTierForFeature(feature)
	if required == "" {
		res.Allowed = true
		res.ReasonCode = ReasonSubscriptionFeatureNotConfig
		return res
	}
	res.RequiredTier = required
	if tierRank(tier) == 0 {
		res.Allowed = false
		res.ReasonCode = ReasonSubscriptionTierUnknown
		return res
	}
	if tierRank(tier) < tierRank(required) {
		res.Allowed = false
		res.ReasonCode = ReasonSubscriptionTierDenied
		return res
	}
	return res
}

// CheckRuntimePremium evaluates tier against premium-only matched schedules or premium channels.
func (c Checker) CheckRuntimePremium(ctx context.Context, companyID string, matchedPremiumSchedule bool, premiumChannelsEnabled bool) Result {
	res := Result{Allowed: true, Feature: FeatureNotificationRulesRuntime}
	if !c.Enabled {
		return res
	}
	if !matchedPremiumSchedule && !premiumChannelsEnabled {
		return res
	}
	tier := c.normalizeTier(c.ResolveCompanyTier(ctx, companyID))
	res.Tier = tier
	res.RequiredTier = TierPremium
	if tierRank(tier) == 0 {
		res.Allowed = false
		res.ReasonCode = ReasonSubscriptionTierUnknown
		return res
	}
	if tierRank(tier) < tierRank(TierPremium) {
		res.Allowed = false
		res.ReasonCode = ReasonSubscriptionTierDenied
		return res
	}
	return res
}

// ValidateAlertChannelPrefsMutation returns an HTTP 402 error when prefs violate tier limits.
func (c Checker) ValidateAlertChannelPrefsMutation(ctx context.Context, userID string, prefs map[string]any) error {
	if !c.Enabled || prefs == nil {
		return nil
	}
	tier := c.normalizeTier(c.ResolveUserTier(ctx, userID))
	if hasPremiumViolation(prefs) {
		if tierRank(tier) == 0 {
			return newTierRequiredError(FeatureNotificationRulesMutation, tier, TierPremium, ReasonSubscriptionTierUnknown)
		}
		if tierRank(tier) < tierRank(TierPremium) {
			return newTierRequiredError(FeatureNotificationRulesMutation, tier, TierPremium, ReasonSubscriptionTierDenied)
		}
	}
	return nil
}

func (c Checker) normalizeTier(tier string) string {
	t := strings.TrimSpace(tier)
	if t == "" {
		return ""
	}
	switch strings.ToLower(t) {
	case "free":
		return TierFree
	case "premium":
		return TierPremium
	case "enterprise":
		return TierEnterprise
	default:
		return t
	}
}

func requiredTierForFeature(feature string) string {
	switch strings.TrimSpace(feature) {
	case FeatureNotificationRulesMutation, FeatureNotificationRulesRuntime, FeatureNotificationRulesSimulation:
		// Simulation is allowed for all tiers (read-only); premium content surfaced via warnings.
		if feature == FeatureNotificationRulesSimulation {
			return ""
		}
		return TierPremium
	default:
		return ""
	}
}

func tierRank(tier string) int {
	switch strings.TrimSpace(tier) {
	case TierEnterprise:
		return 3
	case TierPremium:
		return 2
	case TierFree:
		return 1
	default:
		return 0
	}
}

// HasPremiumViolation reports whether prefs enable premium-only schedules or premium channels.
func hasPremiumViolation(prefs map[string]any) bool {
	if schedules, ok := prefs["schedules"].([]any); ok {
		for _, item := range schedules {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			enabled, _ := m["enabled"].(bool)
			premium, _ := m["premium_only"].(bool)
			if enabled && premium {
				return true
			}
		}
	}
	if channels, ok := prefs["channels"].(map[string]any); ok {
		for _, key := range []string{"zalo", "sms"} {
			raw, ok := channels[key].(map[string]any)
			if !ok {
				continue
			}
			if enabled, _ := raw["enabled"].(bool); enabled {
				return true
			}
		}
	}
	return false
}

// MatchedPremiumSchedule reports whether any matched schedule is premium-only.
func MatchedPremiumSchedule(schedules []ScheduleMatch) bool {
	for _, s := range schedules {
		if s.PremiumOnly {
			return true
		}
	}
	return false
}

// ScheduleMatch mirrors reminder evaluator schedule match (decoupled type for entitlement).
type ScheduleMatch struct {
	PremiumOnly bool
}

func newTierRequiredError(feature, currentTier, requiredTier, reasonCode string) error {
	displayCurrent := currentTier
	if strings.TrimSpace(displayCurrent) == "" {
		displayCurrent = TierFree
	}
	he := perr.NewHTTPError(http.StatusPaymentRequired, perr.CodeSubscriptionTierRequired,
		"subscription tier upgrade required", nil)
	he.Details = map[string]any{
		"error":          "subscription_tier_required",
		"feature":        feature,
		"current_tier":   displayCurrent,
		"required_tier":  requiredTier,
		"reason_code":    reasonCode,
	}
	return he
}

// DisplayTier returns a safe tier label for logs/API (never leaks secrets).
func DisplayTier(tier string) string {
	t := strings.TrimSpace(tier)
	if t == "" {
		return TierFree
	}
	return t
}
