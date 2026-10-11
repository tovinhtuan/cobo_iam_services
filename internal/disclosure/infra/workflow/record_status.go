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
		rec.PublishedDate = publishedDate(time.Now())
	}
	rec.UpdatedBy = actorUserID
	_, err = a.repo.Update(ctx, *rec)
	return err
}

// publishedDate is the company-local (Asia/Ho_Chi_Minh) date of the approval instant; the UTC
// date is still yesterday until 07:00 in Vietnam (DLA-01).
func publishedDate(now time.Time) string {
	return now.In(disclosureapp.CompanyLocation("Asia/Ho_Chi_Minh")).Format("2006-01-02")
}
