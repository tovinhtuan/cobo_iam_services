package companyplan

// PlanDTO is the additive commercial plan wire shape for Portal responses.
// Only these four fields are exposed — never invoice, amount, payment method,
// billing account, contract detail, or entitlement internals.
type PlanDTO struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	Source      string `json:"source"`
}

// ToPlanDTO maps a domain CompanyPlan to the shared wire DTO.
// nil domain plan → nil DTO (JSON null). Does not badge-filter ACTIVE/PREMIUM.
func ToPlanDTO(p *CompanyPlan) *PlanDTO {
	if p == nil {
		return nil
	}
	return &PlanDTO{
		Code:        string(p.Code),
		DisplayName: p.Code.DisplayName(),
		Status:      string(p.Status),
		Source:      string(p.WireSource()),
	}
}
