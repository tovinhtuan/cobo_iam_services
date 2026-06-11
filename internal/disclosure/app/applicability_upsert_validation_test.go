package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestUpsertTypeVersion_RejectsEmptyApplicabilityClasses(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)

	req := baseUpsertRequest()
	req.ApplicabilityRules = &applicability.TemplateApplicabilityRules{
		ApplicableCompanyClasses: []applicability.CompanyClass{},
		ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
		DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
			applicability.StructureHasSubsidiaries:     {Days: 30},
			applicability.StructureHasSubordinateUnits: {Days: 30},
			applicability.StructureSimpleStructure:     {Days: 20},
		},
	}
	_, err := svc.UpsertTypeVersion(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
}
