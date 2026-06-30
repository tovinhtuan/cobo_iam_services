package entitlement

import (
	"context"
	"database/sql"
	"strings"
)

// NewMySQLCompanyTierResolver returns the highest subscription tier among active company members.
func NewMySQLCompanyTierResolver(db *sql.DB) CompanyTierResolver {
	if db == nil {
		return nil
	}
	return func(ctx context.Context, companyID string) string {
		companyID = strings.TrimSpace(companyID)
		if companyID == "" {
			return ""
		}
		rows, err := db.QueryContext(ctx, `
			SELECT ust.subscription_tier
			FROM memberships m
			INNER JOIN user_subscription_tiers ust ON ust.user_id = m.user_id
				AND (ust.effective_to IS NULL OR ust.effective_to > UTC_TIMESTAMP())
			WHERE m.company_id = ? AND m.status = 'active'
		`, companyID)
		if err != nil {
			return ""
		}
		defer rows.Close()
		bestRank := 0
		bestTier := ""
		for rows.Next() {
			var tier string
			if err := rows.Scan(&tier); err != nil {
				continue
			}
			r := tierRank(normalizeTierLabel(tier))
			if r > bestRank {
				bestRank = r
				bestTier = normalizeTierLabel(tier)
			}
		}
		return bestTier
	}
}

func normalizeTierLabel(tier string) string {
	c := Checker{}
	return c.normalizeTier(tier)
}
