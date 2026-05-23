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

func defaultDeadlineRuleCatalog() []DeadlineRuleCatalogDTO {
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

func (s *service) loadDeadlineRuleCatalog(ctx context.Context) []DeadlineRuleCatalogDTO {
	reader, ok := s.repo.(deadlineRuleCatalogReader)
	if !ok {
		return defaultDeadlineRuleCatalog()
	}
	items, err := reader.ListActiveDeadlineRuleCatalog(ctx)
	if err != nil || len(items) == 0 {
		return defaultDeadlineRuleCatalog()
	}
	return items
}
