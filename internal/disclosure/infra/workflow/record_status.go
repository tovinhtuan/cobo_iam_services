package workflow

import (
	"context"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// RecordStatusAdapter updates disclosure records when workflow tasks complete.
type RecordStatusAdapter struct {
	repo disclosureapp.Repository
}

func NewRecordStatusAdapter(repo disclosureapp.Repository) *RecordStatusAdapter {
	return &RecordStatusAdapter{repo: repo}
}

func (a *RecordStatusAdapter) MarkRecordApproved(ctx context.Context, companyID, recordID, actorUserID string) error {
	rec, err := a.repo.FindByID(ctx, companyID, recordID)
	if err != nil {
		return err
	}
	rec.Status = "Approved"
	if strings.TrimSpace(rec.PublishedDate) == "" {
		rec.PublishedDate = time.Now().UTC().Format("2006-01-02")
	}
	rec.UpdatedBy = actorUserID
	_, err = a.repo.Update(ctx, *rec)
	return err
}
