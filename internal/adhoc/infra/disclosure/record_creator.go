// Package disclosure adapts disclosureapp.Service to adhocapp.RecordCreator.
package disclosure

import (
	"context"
	"fmt"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// RecordCreatorAdapter wraps disclosureapp.Service and implements adhocapp.RecordCreator.
type RecordCreatorAdapter struct {
	svc disclosureapp.Service
}

func NewRecordCreatorAdapter(svc disclosureapp.Service) adhocapp.RecordCreator {
	return &RecordCreatorAdapter{svc: svc}
}

func (a *RecordCreatorAdapter) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string) (string, error) {
	sub := disclosureapp.Subject{
		UserID:       createdByMembershipID,
		MembershipID: createdByMembershipID,
		CompanyID:    companyID,
	}
	rec, err := a.svc.CreateRecord(ctx, disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{
			TypeID:  typeID,
			Title:   title,
			Content: title, // minimal content; can be enriched later
		},
	})
	if err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}
	if _, err := a.svc.SubmitRecord(ctx, disclosureapp.SubmitRecordRequest{
		Subject:  sub,
		RecordID: rec.RecordID,
	}); err != nil {
		return "", fmt.Errorf("submit record: %w", err)
	}
	return rec.RecordID, nil
}
