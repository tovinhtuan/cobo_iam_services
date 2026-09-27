package inmemory

import (
	"context"
	"sort"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"net/http"
)

func (r *Repository) CreateImportAttempt(_ context.Context, row *disclosureapp.TemplateImportAttempt) error {
	if row == nil || strings.TrimSpace(row.ID) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "import attempt id is required", nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *row
	r.importAttempts[row.ID] = &cp
	return nil
}

func (r *Repository) GetImportAttemptByID(_ context.Context, id string) (*disclosureapp.TemplateImportAttempt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	row := r.importAttempts[id]
	if row == nil {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (r *Repository) ClaimImportAttempt(_ context.Context, id, companyID, actorUserID string, rowVersion int64, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.importAttempts[id]
	if row == nil || row.CompanyID != companyID || row.ActorUserID != actorUserID || row.RowVersion != rowVersion || row.CreatedAt.Before(cutoff) {
		return 0, nil
	}
	if row.Status != disclosureapp.ImportAttemptStatusValidated && row.Status != disclosureapp.ImportAttemptStatusMappingRequired {
		return 0, nil
	}
	row.Status = disclosureapp.ImportAttemptStatusConfirming
	row.RowVersion++
	row.UpdatedAt = time.Now().UTC()
	return row.RowVersion, nil
}

func (r *Repository) MarkImportAttemptConfirmed(_ context.Context, id string, rowVersion int64, createdTypeID, targetTypeID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.importAttempts[id]
	if row == nil || row.RowVersion != rowVersion || row.Status != disclosureapp.ImportAttemptStatusConfirming {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt version mismatch", nil)
	}
	row.Status = disclosureapp.ImportAttemptStatusConfirmed
	row.CreatedTypeID = createdTypeID
	row.TargetTypeID = targetTypeID
	row.ConfirmedAt = at.UTC()
	row.UpdatedAt = at.UTC()
	return nil
}

func (r *Repository) MarkImportAttemptFailed(_ context.Context, id string, rowVersion int64, errorCode, targetTypeID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.importAttempts[id]
	if row == nil || row.RowVersion != rowVersion || row.Status != disclosureapp.ImportAttemptStatusConfirming {
		return nil
	}
	if row.Status == disclosureapp.ImportAttemptStatusConfirmed {
		return nil
	}
	row.Status = disclosureapp.ImportAttemptStatusConfirmFailed
	row.ConfirmErrorCode = errorCode
	row.TargetTypeID = targetTypeID
	row.ConfirmedAt = at.UTC()
	row.UpdatedAt = at.UTC()
	return nil
}

func (r *Repository) ListImportAttempts(_ context.Context, q disclosureapp.ListTemplateImportHistoryRequest) ([]disclosureapp.TemplateImportAttempt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := time.Now().UTC().Add(-time.Duration(disclosureapp.TemplateImportRetentionDays) * 24 * time.Hour)
	cursorTime, cursorID, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	out := make([]disclosureapp.TemplateImportAttempt, 0)
	for _, row := range r.importAttempts {
		if row.CompanyID != q.Subject.CompanyID || row.CreatedAt.Before(cutoff) {
			continue
		}
		if q.Status != "" && row.Status != q.Status {
			continue
		}
		if q.Filename != "" && !strings.Contains(strings.ToLower(row.Filename), strings.ToLower(q.Filename)) {
			continue
		}
		if q.FileSHA256 != "" && !strings.EqualFold(row.FileSHA256, q.FileSHA256) {
			continue
		}
		if !q.From.IsZero() && row.CreatedAt.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && row.CreatedAt.After(q.To) {
			continue
		}
		if !cursorTime.IsZero() {
			if row.CreatedAt.After(cursorTime) || (row.CreatedAt.Equal(cursorTime) && row.ID >= cursorID) {
				continue
			}
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit+1]
	}
	return out, nil
}

func decodeCursor(cursor string) (time.Time, string, error) {
	return disclosureapp.DecodeImportHistoryCursor(cursor)
}
