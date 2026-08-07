package disclosure

import (
	"context"
	"sort"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// WorkflowSeederAdapter clones GetEffectiveWorkflow into proposal step inputs.
// Does not mutate the template. Assignee is always left empty on seed.
type WorkflowSeederAdapter struct {
	svc disclosureapp.Service
}

func NewWorkflowSeederAdapter(svc disclosureapp.Service) *WorkflowSeederAdapter {
	return &WorkflowSeederAdapter{svc: svc}
}

func (a *WorkflowSeederAdapter) SeedFromDisclosureType(ctx context.Context, companyID, typeID string) ([]adhocapp.ProposalWorkflowStepInput, error) {
	if a == nil || a.svc == nil {
		return nil, nil
	}
	resp, err := a.svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID},
		TypeID:  typeID,
	})
	if err != nil {
		return nil, err
	}
	steps := append([]disclosureapp.WorkflowStepDTO(nil), resp.Data.Workflow...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].DisplayOrder != steps[j].DisplayOrder {
			return steps[i].DisplayOrder < steps[j].DisplayOrder
		}
		return steps[i].StepID < steps[j].StepID
	})
	out := make([]adhocapp.ProposalWorkflowStepInput, 0, len(steps))
	for i, st := range steps {
		name := strings.TrimSpace(st.Stage)
		if name == "" {
			name = strings.TrimSpace(st.Description)
		}
		if name == "" {
			name = strings.TrimSpace(st.StepID)
		}
		out = append(out, adhocapp.ProposalWorkflowStepInput{
			SourceStepID:   strings.TrimSpace(st.StepID),
			Order:          i + 1,
			Name:           name,
			ProcessingDays: st.ProcessingDays,
			DepartmentID:   strings.TrimSpace(st.DepartmentID),
			// Assignee always null on seed — product: handler chosen per proposal.
		})
	}
	return out, nil
}
