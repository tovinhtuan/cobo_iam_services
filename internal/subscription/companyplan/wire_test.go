package companyplan

import "testing"

func TestToPlanDTO_NilAndFull(t *testing.T) {
	if ToPlanDTO(nil) != nil {
		t.Fatal("nil → nil")
	}
	p := &CompanyPlan{Code: PlanCodePremium, Status: PlanStatusTrial}
	dto := ToPlanDTO(p)
	if dto == nil || dto.Code != "PREMIUM" || dto.DisplayName != "Premium" || dto.Status != "TRIAL" || dto.Source != "COMPANY_SUBSCRIPTION" {
		t.Fatalf("dto=%+v", dto)
	}
}
