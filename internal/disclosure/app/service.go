package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo Repository
	auth authapp.Service
	idg  idgen.Generator
}

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator) Service {
	return &service{repo: repo, auth: auth, idg: idg}
}

func (s *service) CreateRecord(ctx context.Context, req CreateRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.Payload.Title) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title is required", nil)
	}
	if strings.TrimSpace(req.Payload.Content) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content is required", nil)
	}
	plannedDate, err := normalizeDate(req.Payload.PlannedDate)
	if err != nil {
		return nil, err
	}
	departmentID := strings.TrimSpace(req.Payload.DepartmentID)
	if departmentID == "" {
		departmentID = "general"
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.create", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   "",
		Attributes: map[string]any{
			"department_id":       departmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      "draft",
		},
	}); err != nil {
		return nil, err
	}
	rec := RecordDTO{
		RecordID:      s.idg.NewUUID(),
		CompanyID:     req.Subject.CompanyID,
		TypeID:        strings.TrimSpace(req.Payload.TypeID),
		DepartmentID:  departmentID,
		Title:         strings.TrimSpace(req.Payload.Title),
		Summary:       strings.TrimSpace(req.Payload.Summary),
		Content:       strings.TrimSpace(req.Payload.Content),
		PlannedDate:   plannedDate,
		Status:        "Draft",
		Attachments:   sanitizeAttachments(req.Payload.Attachments),
		EvidenceLink:  strings.TrimSpace(req.Payload.EvidenceLink),
		CreatedBy:     req.Subject.UserID,
		UpdatedBy:     req.Subject.UserID,
	}
	return s.repo.Create(ctx, rec)
}

func (s *service) UpdateRecord(ctx context.Context, req UpdateRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	plannedDate, err := normalizeDate(req.Payload.PlannedDate)
	if err != nil {
		return nil, err
	}
	departmentID := strings.TrimSpace(req.Payload.DepartmentID)
	if departmentID == "" {
		departmentID = cur.DepartmentID
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.update", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	cur.TypeID = strings.TrimSpace(req.Payload.TypeID)
	cur.DepartmentID = departmentID
	cur.Title = strings.TrimSpace(req.Payload.Title)
	cur.Summary = strings.TrimSpace(req.Payload.Summary)
	cur.Content = strings.TrimSpace(req.Payload.Content)
	cur.PlannedDate = plannedDate
	cur.Attachments = sanitizeAttachments(req.Payload.Attachments)
	cur.EvidenceLink = strings.TrimSpace(req.Payload.EvidenceLink)
	cur.UpdatedBy = req.Subject.UserID
	return s.repo.Update(ctx, *cur)
}

func (s *service) SubmitRecord(ctx context.Context, req SubmitRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.submit", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	cur.Status = "Published"
	cur.PublishedDate = time.Now().UTC().Format("2006-01-02")
	cur.UpdatedBy = req.Subject.UserID
	return s.repo.Update(ctx, *cur)
}

func (s *service) ConfirmRecord(ctx context.Context, req ConfirmRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.approve", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	if strings.ToLower(cur.Status) != "published" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is not in published state", nil)
	}
	cur.Status = "Completed"
	cur.UpdatedBy = req.Subject.UserID
	return s.repo.Update(ctx, *cur)
}

func (s *service) ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error) {
	if err := s.authorize(ctx, req.Subject, "disclosure.view", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   "",
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	items, err := s.repo.List(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}
	return &ListRecordsResponse{Items: items}, nil
}

func (s *service) GetRecord(ctx context.Context, req GetRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.view", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	return cur, nil
}

func normalizeDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "planned_date must be YYYY-MM-DD", nil)
	}
	return raw, nil
}

func sanitizeAttachments(items []AttachmentDTO) []AttachmentDTO {
	if len(items) == 0 {
		return []AttachmentDTO{}
	}
	out := make([]AttachmentDTO, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		out = append(out, AttachmentDTO{
			ID:         strings.TrimSpace(it.ID),
			Name:       name,
			Type:       strings.TrimSpace(it.Type),
			UploadedAt: strings.TrimSpace(it.UploadedAt),
		})
	}
	return out
}

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Action: action, Resource: resource})
	if err != nil {
		return fmt.Errorf("authorize disclosure action: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}
