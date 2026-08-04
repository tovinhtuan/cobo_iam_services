package companyplan

import (
	"context"
	"strings"
	"time"
)

// Service is the shared public Reader for commercial company plans (Case C).
// No caching. Wraps an underlying Reader (MySQL or memory); does not reimplement SQL.
type Service struct {
	reader Reader
}

// NewService constructs the shared companyplan Reader façade.
func NewService(reader Reader) *Service {
	if reader == nil {
		panic("companyplan: NewService requires a non-nil Reader")
	}
	return &Service{reader: reader}
}

// GetEffectivePlan returns the covering commercial plan for companyID at time at.
// No covering record → (nil, nil). Propagates repository/database errors.
// Does not badge-filter; non-ACTIVE covering rows keep their real status.
func (s *Service) GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*CompanyPlan, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	return s.reader.GetEffectivePlan(ctx, companyID, at)
}

// GetEffectivePlans batch-resolves plans keyed by company_id.
// Deduplicates IDs; empty input returns an empty map without calling the repository.
// Companies without a covering record are omitted (no fake plan). Same selection rule as GetEffectivePlan.
func (s *Service) GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*CompanyPlan, error) {
	ids := uniqueNonEmpty(companyIDs)
	out := make(map[string]*CompanyPlan, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	got, err := s.reader.GetEffectivePlans(ctx, ids, at)
	if err != nil {
		return nil, err
	}
	if got == nil {
		return out, nil
	}
	return got, nil
}

// Ensure Service satisfies Reader.
var _ Reader = (*Service)(nil)
