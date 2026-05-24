package app

import (
	"context"
)

type DeadlineRuleCatalogDTO struct {
	Code      string `json:"code"`
	LabelVI   string `json:"label_vi"`
	Pattern   string `json:"pattern"`
	InputType string `json:"input_type"`
}

type deadlineRuleCatalogReader interface {
	ListActiveDeadlineRuleCatalog(ctx context.Context) ([]DeadlineRuleCatalogDTO, error)
}

// DefaultDeadlineRuleCatalog returns the two built-in deadline rules used as
// a seed/fallback when the deadline_rule_catalog table is empty.
func DefaultDeadlineRuleCatalog() []DeadlineRuleCatalogDTO {
	return []DeadlineRuleCatalogDTO{
		{
			Code:      "T+N",
			LabelVI:   "Trong vòng N ngày kể từ ngày sự kiện",
			Pattern:   `^T\+\d+$`,
			InputType: "number",
		},
		{
			Code:      "dd/mm",
			LabelVI:   "Ngày dd/mm hàng năm",
			Pattern:   `^\d{2}/\d{2}$`,
			InputType: "date_dm",
		},
	}
}

// defaultDeadlineRuleCatalog is retained for internal callers.
func defaultDeadlineRuleCatalog() []DeadlineRuleCatalogDTO {
	return DefaultDeadlineRuleCatalog()
}

func (s *service) loadDeadlineRuleCatalog(ctx context.Context) []DeadlineRuleCatalogDTO {
	items, err := s.repo.ListActiveDeadlineRuleCatalog(ctx)
	if err != nil || len(items) == 0 {
		return defaultDeadlineRuleCatalog()
	}
	return items
}
