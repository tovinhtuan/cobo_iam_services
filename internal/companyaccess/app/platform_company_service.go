package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/companystatus"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	companyHistoryDefaultLimit = 20
	companyHistoryMaxLimit     = 50
)

var hcmLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return loc
	}
	return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
}()

func (s *adminService) auditContext(sub AdminSubject, requestID string) CompanyAuditContext {
	return CompanyAuditContext{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		RequestID:         strings.TrimSpace(requestID),
		OccurredAt:        time.Now().UTC(),
	}
}

func (s *adminService) CreateCompany(ctx context.Context, req CreateCompanyRequest) (*CreateCompanyResult, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.CompanyName)
	fields := map[string]*string{
		"company_name": &name, "tax_code": &req.TaxCode, "registration_number": &req.RegistrationNumber,
		"address": &req.Address, "phone": &req.Phone, "contact_email": &req.ContactEmail,
		"representative_name": &req.RepresentativeName,
	}
	if errs := validateCompanyFields(fields); len(errs) > 0 {
		return nil, companyFieldErrors(errs)
	}
	vStatus, err := companystatus.NormalizeVerificationStatus(companystatus.VerificationVerified)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
	}
	boot := CreateCompanyBootstrap{
		VerificationStatus: vStatus,
		TaxCode:            strings.TrimSpace(req.TaxCode),
		RegistrationNumber: strings.TrimSpace(req.RegistrationNumber),
		Address:            strings.TrimSpace(req.Address),
		Phone:              strings.TrimSpace(req.Phone),
		ContactEmail:       strings.TrimSpace(req.ContactEmail),
		RepresentativeName: strings.TrimSpace(req.RepresentativeName),
	}
	after := map[string]any{
		"company_name": name, "verification_status": vStatus, "tax_code": boot.TaxCode,
		"registration_number": boot.RegistrationNumber, "address": boot.Address,
		"phone":               maskCompanyField("phone", boot.Phone),
		"contact_email":       maskCompanyField("contact_email", boot.ContactEmail),
		"representative_name": maskCompanyField("representative_name", boot.RepresentativeName),
	}
	companyID, companyCode, err := s.repo.CreateStandaloneCompanyAudited(ctx, name, boot, s.auditContext(req.Subject, req.RequestID),
		func(companyID, companyCode string) CompanyAuditRecord {
			after["company_code"] = companyCode
			return CompanyAuditRecord{
				EventID:  s.idg.NewUUID(),
				Action:   CompanyAuditCreate,
				Metadata: map[string]any{"after": after, "source": "cms"},
			}
		})
	if err != nil {
		return nil, err
	}
	return &CreateCompanyResult{CompanyID: companyID, CompanyCode: companyCode, CompanyName: name}, nil
}

func (s *adminService) UpdatePlatformCompany(ctx context.Context, req UpdatePlatformCompanyRequest) error {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return err
	}
	if req.VerificationStatus != nil {
		norm, err := companystatus.NormalizeVerificationStatus(*req.VerificationStatus)
		if err != nil {
			return companyFieldErrors(map[string]string{"verification_status": "Trạng thái xác minh phải là Đã xác minh hoặc Chưa xác minh."})
		}
		req.VerificationStatus = &norm
	}
	if errs := validateCompanyFields(map[string]*string{
		"company_name": req.CompanyName, "tax_code": req.TaxCode, "registration_number": req.RegistrationNumber,
		"address": req.Address, "phone": req.Phone, "contact_email": req.ContactEmail,
		"representative_name": req.RepresentativeName,
	}); len(errs) > 0 {
		return companyFieldErrors(errs)
	}
	companyID := strings.TrimSpace(req.CompanyID)
	expected := strings.TrimSpace(req.ExpectedRevision)
	return s.repo.MutateCompanyPlatform(ctx, companyID, s.auditContext(req.Subject, req.RequestID),
		func(snap CompanySnapshot) (*CompanyMutation, error) {
			if expected != "" {
				if current := CompanyRevision(snap); current != expected {
					return nil, staleCompanyRevision(current)
				}
			}
			changes := companyChanges(snap, req)
			if len(changes) == 0 && !hasNonAuditedUpdate(req) {
				return nil, nil // nothing changed: no write, no audit
			}
			update := changedOnly(snap, req)
			mutation := &CompanyMutation{Update: &update}
			if len(changes) > 0 {
				mutation.Audit = append(mutation.Audit, CompanyAuditRecord{
					EventID: s.idg.NewUUID(), Action: CompanyAuditUpdate,
					Metadata: map[string]any{"changes": changes, "source": "cms"},
				})
			}
			if update.VerificationStatus != nil {
				mutation.Audit = append(mutation.Audit, CompanyAuditRecord{
					EventID: s.idg.NewUUID(), Action: CompanyAuditVerificationChange,
					Metadata: map[string]any{
						"before": map[string]any{"verification_status": snap.VerificationStatus},
						"after":  map[string]any{"verification_status": *update.VerificationStatus},
						"source": "cms",
					},
				})
			}
			return mutation, nil
		})
}

func (s *adminService) SetPlatformCompanyStatus(ctx context.Context, req SetPlatformCompanyStatusRequest) (*PlatformCompanyStatusChange, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	target, err := companystatus.NormalizeOperationalStatus(req.Status)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
	}
	action := CompanyActionForStatus(target)
	reasonCode := strings.ToUpper(strings.TrimSpace(req.ReasonCode))
	reasonNote := strings.TrimSpace(req.ReasonNote)
	fieldErrs := map[string]string{}
	if !companyStatusReasonCodes[action][reasonCode] {
		fieldErrs["reason_code"] = "Chọn lý do hợp lệ cho thao tác này."
	}
	if reasonCode == CompanyReasonOther && reasonNote == "" {
		fieldErrs["reason_note"] = "Nhập ghi chú khi chọn lý do Khác."
	}
	if len([]rune(reasonNote)) > companyReasonNoteMaxRunes {
		fieldErrs["reason_note"] = "Ghi chú tối đa 500 ký tự."
	}
	if action == CompanyActionDeactivate && strings.TrimSpace(req.ConfirmCompanyCode) == "" {
		fieldErrs["confirm_company_code"] = "Nhập mã công ty để xác nhận vô hiệu hoá."
	}
	if len(fieldErrs) > 0 {
		return nil, companyFieldErrors(fieldErrs)
	}
	companyID := strings.TrimSpace(req.CompanyID)
	if target != companystatus.StatusActive { // inactive or suspended restrict the company's members
		if err := s.assertNotPlatformHostCompany(ctx, req.Subject, companyID); err != nil {
			return nil, err
		}
	}
	recipients, err := s.repo.ListCompanyPrimaryAdminEmails(ctx, companyID)
	if err != nil {
		return nil, err
	}

	expected := strings.TrimSpace(req.ExpectedRevision)
	result := &PlatformCompanyStatusChange{To: target}
	var notice *companyStatusNotice
	err = s.repo.MutateCompanyPlatform(ctx, companyID, s.auditContext(req.Subject, req.RequestID),
		func(snap CompanySnapshot) (*CompanyMutation, error) {
			result.From = snap.Status
			if action == CompanyActionDeactivate && !strings.EqualFold(strings.TrimSpace(req.ConfirmCompanyCode), strings.TrimSpace(snap.CompanyCode)) {
				return nil, companyFieldErrors(map[string]string{"confirm_company_code": "Mã công ty không khớp."})
			}
			if expected != "" {
				if current := CompanyRevision(snap); current != expected {
					return nil, staleCompanyRevision(current)
				}
			}
			if snap.Status == target {
				return nil, nil // already there: no-op (double click), no audit, no email
			}
			if snap.Status == companystatus.StatusInactive && target == companystatus.StatusSuspended {
				return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidStateTransition,
					"an inactive company must be activated before it can be suspended", nil)
			}
			to := recipients
			if len(to) == 0 && strings.TrimSpace(snap.ContactEmail) != "" {
				to = []string{strings.TrimSpace(snap.ContactEmail)}
			}
			eventID := s.idg.NewUUID()
			result.Changed = true
			result.AuditEventID = eventID
			result.NotifiedCount = len(to)
			notice = &companyStatusNotice{snapshot: snap, target: target, reasonCode: reasonCode, eventID: eventID, recipients: to}
			return &CompanyMutation{
				Status: target,
				Audit: []CompanyAuditRecord{{
					EventID: eventID,
					Action:  CompanyAuditStatusChange,
					Metadata: map[string]any{
						"before":         map[string]any{"status": snap.Status},
						"after":          map[string]any{"status": target},
						"reason_code":    reasonCode,
						"reason_note":    truncateRunes(reasonNote, companyReasonNoteMaxRunes),
						"notified_count": len(to),
						"source":         "cms",
					},
				}},
			}, nil
		})
	if err != nil {
		return nil, err
	}
	if notice != nil {
		// Detached: the status is committed, a client disconnect must not drop the notification.
		result.NotifiedCount = s.sendCompanyStatusEmails(context.WithoutCancel(ctx), req.Subject, *notice)
	}
	return result, nil
}

type companyStatusNotice struct {
	snapshot   CompanySnapshot
	target     string
	reasonCode string
	eventID    string
	recipients []string
}

// sendCompanyStatusEmails runs after commit: a failure is logged, the status change stands.
// Returns how many emails were queued.
func (s *adminService) sendCompanyStatusEmails(ctx context.Context, sub AdminSubject, n companyStatusNotice) int {
	if s.companyStatusEmailer == nil {
		return 0
	}
	vars := map[string]any{
		"company_name":     n.snapshot.CompanyName,
		"company_code":     n.snapshot.CompanyCode,
		"new_status_label": companyStatusLabelsVI[n.target],
		"reason_label":     companyReasonLabelsVI[n.reasonCode],
		"effective_at":     time.Now().In(hcmLocation).Format("15:04 02/01/2006"),
		"is_suspended":     n.target == companystatus.StatusSuspended,
		"is_inactive":      n.target == companystatus.StatusInactive,
		"is_active":        n.target == companystatus.StatusActive,
	}
	sent := 0
	for _, to := range n.recipients {
		err := s.companyStatusEmailer(ctx, CompanyStatusEmail{
			To:             to,
			CompanyID:      n.snapshot.CompanyID,
			ActorUserID:    sub.UserID,
			IdempotencyKey: "company_status:" + n.snapshot.CompanyID + ":" + n.eventID + ":" + strings.ToLower(to),
			Variables:      vars,
		})
		if err != nil {
			slog.WarnContext(ctx, "company status email not queued", "company_id", n.snapshot.CompanyID, "audit_event_id", n.eventID, "error", err)
			continue
		}
		sent++
	}
	return sent
}

func (s *adminService) ListPlatformCompanyHistory(ctx context.Context, req ListPlatformCompanyHistoryRequest) (*PlatformCompanyHistory, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	if s.auditRepo == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "audit repository not configured", nil)
	}
	companyID := strings.TrimSpace(req.CompanyID)
	if companyID == "" {
		return nil, perrNewBadRequest("company_id is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = companyHistoryDefaultLimit
	}
	if limit > companyHistoryMaxLimit {
		limit = companyHistoryMaxLimit
	}
	cursorAt, cursorEventID, err := decodeCompanyHistoryCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	entries, err := s.auditRepo.ListFiltered(ctx, auditapp.ListFilter{
		CompanyID:       companyID,
		Action:          CompanyAuditActionPrefix,
		ActionPrefix:    true,
		Cursor:          cursorAt,
		CursorEventID:   cursorEventID,
		KeysetByEventID: true,
		Limit:           limit + 1, // one extra row tells whether another page exists
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	names := map[string]string{}
	out := &PlatformCompanyHistory{Items: make([]PlatformCompanyHistoryItem, 0, len(entries)), Limit: limit}
	for _, e := range entries {
		if strings.TrimSpace(e.CompanyID) != companyID {
			continue
		}
		actorID := strings.TrimSpace(e.ActorUserID)
		if _, ok := names[actorID]; !ok && actorID != "" {
			loginID, email, fullName, _, lookupErr := s.repo.GetUserProfile(ctx, actorID)
			if lookupErr == nil {
				names[actorID] = firstNonBlank(fullName, email, loginID)
			} else {
				names[actorID] = ""
			}
		}
		meta := e.Metadata
		item := PlatformCompanyHistoryItem{
			EventID:    e.EventID,
			OccurredAt: e.OccurredAt,
			Action:     e.Action,
			Actor:      PlatformCompanyHistoryActor{UserID: actorID, DisplayName: names[actorID]},
			Changes:    []any{},
		}
		if m, ok := meta["before"].(map[string]any); ok {
			item.Before = m
		}
		if m, ok := meta["after"].(map[string]any); ok {
			item.After = m
		}
		if v, ok := meta["reason_code"].(string); ok {
			item.ReasonCode = v
		}
		if v, ok := meta["reason_note"].(string); ok {
			item.ReasonNote = v
		}
		switch v := meta["changes"].(type) {
		case []any: // decoded from metadata_json
			item.Changes = v
		case []map[string]any: // in-memory audit repository keeps the original value
			for _, c := range v {
				item.Changes = append(item.Changes, c)
			}
		}
		if e.Action == "cms.company_plan.activate" {
			item.Metadata = meta
		}
		out.Items = append(out.Items, item)
	}
	if hasMore {
		last := entries[len(entries)-1]
		out.NextCursor = encodeCompanyHistoryCursor(last.OccurredAt, last.EventID)
	}
	return out, nil
}

func firstNonBlank(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
