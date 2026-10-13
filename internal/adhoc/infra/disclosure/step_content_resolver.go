package disclosure

import (
	"context"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type effectiveWorkflowReader interface {
	GetEffectiveWorkflow(ctx context.Context, req disclosureapp.GetEffectiveWorkflowRequest) (*disclosureapp.GetEffectiveWorkflowResponse, error)
}

// StepContentResolverAdapter reads step content (description, documents, reminder_config) from the
// effective workflow of the submitting user's company. Used once at proposal submit; runtime
// (approve, alerts) only reads the frozen proposal snapshot.
type StepContentResolverAdapter struct {
	svc effectiveWorkflowReader
}

func NewStepContentResolverAdapter(svc disclosureapp.Service) *StepContentResolverAdapter {
	return &StepContentResolverAdapter{svc: svc}
}

func (a *StepContentResolverAdapter) ResolveStepContent(ctx context.Context, sub adhocapp.Subject, typeID string) (*adhocapp.StepContentCatalog, error) {
	if a == nil || a.svc == nil {
		return &adhocapp.StepContentCatalog{}, nil
	}
	resp, err := a.svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject:     disclosureapp.Subject{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID},
		TypeID:      typeID,
		RuntimeOnly: true, // never freeze an unpublished CMS draft
	})
	if err != nil {
		return nil, err
	}
	catalog := &adhocapp.StepContentCatalog{
		WorkflowSource: strings.TrimSpace(resp.Data.Source),
		VersionNo:      resp.Data.VersionNo,
		BySourceStepID: make(map[string]adhocapp.ProposalStepInheritedContent, len(resp.Data.Workflow)),
	}
	for _, st := range resp.Data.Workflow {
		id := strings.TrimSpace(st.StepID)
		if id == "" {
			continue
		}
		if _, dup := catalog.BySourceStepID[id]; dup {
			continue
		}
		catalog.BySourceStepID[id] = stepInheritedContent(st)
	}
	return catalog, nil
}

func stepInheritedContent(st disclosureapp.WorkflowStepDTO) adhocapp.ProposalStepInheritedContent {
	out := adhocapp.ProposalStepInheritedContent{
		Description:       strings.TrimSpace(st.Description),
		DescriptionFormat: strings.TrimSpace(st.DescriptionFormat),
		Instructions:      strings.TrimSpace(st.Instructions),
	}
	if out.Description == "" {
		out.DescriptionFormat = ""
	}
	for _, d := range st.Documents {
		doc := adhocapp.ProposalStepDocument{
			DocID:            strings.TrimSpace(d.DocID),
			Name:             strings.TrimSpace(d.Name),
			Required:         d.Required,
			TemplateFileID:   strings.TrimSpace(d.TemplateFileID),
			TemplateFileName: strings.TrimSpace(d.TemplateFileName),
		}
		if doc.TemplateFileID == "" {
			doc.TemplateFileName = ""
		}
		if doc.DocID == "" && doc.Name == "" {
			continue
		}
		out.Documents = append(out.Documents, doc)
	}
	if cfg := st.ReminderConfig; cfg != nil {
		out.ReminderConfig = &adhocapp.ProposalStepReminderConfig{
			Enabled:    cfg.Enabled,
			Mode:       strings.TrimSpace(cfg.Mode),
			DaysBefore: append([]int(nil), cfg.DaysBefore...),
		}
	}
	return out
}
