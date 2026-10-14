package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// StepContentUnavailableReason is details.reason on the 503 returned by submit when the
// template step content cannot be read (infrastructure failure). The proposal stays draft.
const StepContentUnavailableReason = "step_content_unavailable"

// ProposalStepDocument is one document requirement inherited from the template step.
// TemplateFileName is display-only and dropped when there is no TemplateFileID.
type ProposalStepDocument struct {
	DocID            string `json:"doc_id"`
	Name             string `json:"name"`
	Required         bool   `json:"required"`
	TemplateFileID   string `json:"template_file_id,omitempty"`
	TemplateFileName string `json:"template_file_name,omitempty"`
}

// ProposalStepReminderConfig mirrors the template step reminder_config JSON
// (disclosure WorkflowStepReminderConfig) so adhoc/app does not import disclosure/app.
type ProposalStepReminderConfig struct {
	Enabled    bool   `json:"enabled"`
	Mode       string `json:"mode,omitempty"`
	DaysBefore []int  `json:"days_before,omitempty"`
}

// ProposalStepInheritedContent is the template step content frozen into a proposal step at
// submit (server-authoritative; never accepted from the client). Present (possibly empty) only
// for steps whose source_step_id matched the company's effective workflow at submit time.
type ProposalStepInheritedContent struct {
	Description       string                 `json:"description,omitempty"`
	DescriptionFormat string                 `json:"description_format,omitempty"`
	Instructions      string                 `json:"instructions,omitempty"`
	Documents         []ProposalStepDocument `json:"documents,omitempty"`
	// ReminderConfig nil means the runtime default reminder rule.
	ReminderConfig *ProposalStepReminderConfig `json:"reminder_config,omitempty"`
}

// ProposalContentCapture records when and from which effective workflow the step content was captured.
type ProposalContentCapture struct {
	CapturedAt     time.Time `json:"captured_at"`
	WorkflowSource string    `json:"workflow_source,omitempty"`
	VersionNo      int       `json:"version_no,omitempty"`
	// MissingSourceStepIDs lists source_step_id values no longer present in the template.
	MissingSourceStepIDs []string `json:"missing_source_step_ids,omitempty"`
}

// StepContentCatalog is the company's effective workflow step content keyed by template step id.
type StepContentCatalog struct {
	WorkflowSource string
	VersionNo      int
	BySourceStepID map[string]ProposalStepInheritedContent
}

// StepContentResolver reads the effective workflow of the proposal's own company
// (sub.CompanyID) for the disclosure type. 400/404/409/422 mean "no template content"
// (submit continues without capture); any other error, including 401/403, is unavailable (503).
type StepContentResolver interface {
	ResolveStepContent(ctx context.Context, sub Subject, typeID string) (*StepContentCatalog, error)
}

// AttachStepContentCapture enables submit-time capture of template step content
// (ADHOC_PROPOSAL_INHERIT_STEP_CONTENT_ENABLED). A nil resolver keeps capture disabled.
func AttachStepContentCapture(svc Service, resolver StepContentResolver) Service {
	if s, ok := svc.(*service); ok {
		s.stepContent = resolver
	}
	return svc
}

// stepContentResolveTimeout bounds the template read so submit latency stays bounded when the DB is slow;
// a timeout maps to the retryable 503.
var stepContentResolveTimeout = 5 * time.Second

// captureStepContent fills inherited content on snap (already normalized for submit, not shared
// with the persisted draft). Draft content is always stripped first so only the server sets it.
// Returns a log summary when content was captured (nil when capture is off or skipped).
func (s *service) captureStepContent(ctx context.Context, sub Subject, proposalID, typeID string, snap *ProposalWorkflowSnapshot) (*stepContentCaptureSummary, error) {
	if snap == nil {
		return nil, nil
	}
	stripInheritedContent(snap)
	if s.stepContent == nil {
		return nil, nil
	}
	capturedAt := time.Now().UTC()
	if !hasSourceSteps(snap) {
		// Only self-added steps: nothing to read from the template.
		ApplyStepContentCatalog(snap, &StepContentCatalog{}, capturedAt)
		return summarizeStepContentCapture(snap), nil
	}
	resolveCtx, cancel := context.WithTimeout(ctx, stepContentResolveTimeout)
	defer cancel()
	catalog, err := s.stepContent.ResolveStepContent(resolveCtx, sub, typeID)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && isNoTemplateContentStatus(he.HTTPStatus) {
			slog.Info("adhoc.step_content_skipped",
				slog.String("proposal_id", proposalID),
				slog.String("type_id", typeID),
				slog.Int("status", he.HTTPStatus),
				slog.String("code", string(he.Code)))
			return nil, nil
		}
		slog.Warn("adhoc.step_content_unavailable",
			slog.String("proposal_id", proposalID),
			slog.String("type_id", typeID),
			slog.Any("error", err))
		// Vietnamese text: clients that do not map details.reason show the message as is.
		he := perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable,
			"Chưa lấy được nội dung bước từ mẫu, vui lòng thử gửi lại.", err)
		he.Details = map[string]any{"reason": StepContentUnavailableReason}
		return nil, he
	}
	ApplyStepContentCatalog(snap, catalog, capturedAt)
	return summarizeStepContentCapture(snap), nil
}

type stepContentCaptureSummary struct {
	steps, inheritedSteps, documents, missingSourceSteps int
	workflowSource                                       string
	versionNo                                            int
}

func summarizeStepContentCapture(snap *ProposalWorkflowSnapshot) *stepContentCaptureSummary {
	out := &stepContentCaptureSummary{steps: len(snap.Steps)}
	for _, st := range snap.Steps {
		if st.Inherited != nil {
			out.inheritedSteps++
			out.documents += len(st.Inherited.Documents)
		}
	}
	if snap.ContentCapture != nil {
		out.missingSourceSteps = len(snap.ContentCapture.MissingSourceStepIDs)
		out.workflowSource = snap.ContentCapture.WorkflowSource
		out.versionNo = snap.ContentCapture.VersionNo
	}
	return out
}

// logStepContentCaptured runs after the submit UPDATE so a lost race / replay is visible (applied=false).
func logStepContentCaptured(proposalID, typeID string, sum *stepContentCaptureSummary, applied bool) {
	if sum == nil {
		return
	}
	slog.Info("adhoc.step_content_captured",
		slog.String("proposal_id", proposalID),
		slog.String("type_id", typeID),
		slog.Bool("applied", applied),
		slog.Int("steps", sum.steps),
		slog.Int("inherited_steps", sum.inheritedSteps),
		slog.Int("documents", sum.documents),
		slog.Int("missing_source_steps", sum.missingSourceSteps),
		slog.String("workflow_source", sum.workflowSource),
		slog.Int("version_no", sum.versionNo))
}

// ApplyStepContentCatalog copies catalog content onto every step whose source_step_id matches and
// records the capture metadata. Steps without source_step_id stay without inherited content;
// unmatched source ids are listed in MissingSourceStepIDs (no error).
func ApplyStepContentCatalog(snap *ProposalWorkflowSnapshot, catalog *StepContentCatalog, capturedAt time.Time) {
	if snap == nil {
		return
	}
	if catalog == nil {
		catalog = &StepContentCatalog{}
	}
	capture := &ProposalContentCapture{
		CapturedAt:     capturedAt.UTC(),
		WorkflowSource: strings.TrimSpace(catalog.WorkflowSource),
		VersionNo:      catalog.VersionNo,
	}
	seenMissing := map[string]struct{}{}
	for i := range snap.Steps {
		snap.Steps[i].Inherited = nil
		sourceID := strings.TrimSpace(snap.Steps[i].SourceStepID)
		if sourceID == "" {
			continue
		}
		content, ok := catalog.BySourceStepID[sourceID]
		if !ok {
			if _, dup := seenMissing[sourceID]; !dup {
				seenMissing[sourceID] = struct{}{}
				capture.MissingSourceStepIDs = append(capture.MissingSourceStepIDs, sourceID)
			}
			continue
		}
		snap.Steps[i].Inherited = CloneProposalStepInheritedContent(&content)
	}
	snap.ContentCapture = capture
}

// CloneProposalStepInheritedContent returns a deep copy, or nil.
func CloneProposalStepInheritedContent(in *ProposalStepInheritedContent) *ProposalStepInheritedContent {
	if in == nil {
		return nil
	}
	out := *in
	if in.Documents != nil {
		out.Documents = append([]ProposalStepDocument(nil), in.Documents...)
	}
	if in.ReminderConfig != nil {
		cfg := *in.ReminderConfig
		if in.ReminderConfig.DaysBefore != nil {
			cfg.DaysBefore = append([]int(nil), in.ReminderConfig.DaysBefore...)
		}
		out.ReminderConfig = &cfg
	}
	return &out
}

// isNoTemplateContentStatus: the type/workflow does not exist or is not usable. Auth failures are not
// in the list — the submitter already holds ad_hoc_alert.propose, so a 401/403 here is an anomaly
// and must not silently drop the content (and with it the required-document gate).
func isNoTemplateContentStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
		return true
	}
	return false
}

// WithoutInheritedStepContent returns a copy without per-step inherited content and proposer
// additions (list projection: keeps content_capture, drops the potentially large step bodies).
func WithoutInheritedStepContent(snap *ProposalWorkflowSnapshot) *ProposalWorkflowSnapshot {
	if snap == nil {
		return nil
	}
	out := *snap
	out.Steps = append([]ProposalWorkflowStep(nil), snap.Steps...)
	for i := range out.Steps {
		out.Steps[i].Inherited = nil
		out.Steps[i].Additions = nil
	}
	return &out
}

func stripInheritedContent(snap *ProposalWorkflowSnapshot) {
	snap.ContentCapture = nil
	for i := range snap.Steps {
		snap.Steps[i].Inherited = nil
	}
}

func hasSourceSteps(snap *ProposalWorkflowSnapshot) bool {
	for _, st := range snap.Steps {
		if strings.TrimSpace(st.SourceStepID) != "" {
			return true
		}
	}
	return false
}
