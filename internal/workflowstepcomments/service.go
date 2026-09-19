package workflowstepcomments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	"github.com/google/uuid"
)

// Service implements step discussion comments (LIST/CREATE/EDIT/DELETE).
type Service struct {
	deadline         wff.CommentDeadlineContext
	repo             Repository
	idem             idempotency.Store
	names            DisplayNameResolver
	audit            Auditor
	candidates       MentionCandidateStore
	mentionsRepo     MentionsRepository
	memberships      MembershipActiveLookup
	mentionDisplays  MentionDisplayResolver
	mentionNotifier  MentionInAppNotifier
	membershipUsers  MembershipUserResolver
	db               *sql.DB
	mentionsEnabled  bool
	now              func() time.Time
	log              *slog.Logger
}

func NewService(deadline wff.CommentDeadlineContext, repo Repository, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{deadline: deadline, repo: repo, now: time.Now, log: log}
}

func (s *Service) WithIdempotency(store idempotency.Store) *Service {
	s.idem = store
	return s
}

func (s *Service) WithDisplayNames(r DisplayNameResolver) *Service {
	s.names = r
	return s
}

func (s *Service) WithAudit(a Auditor) *Service {
	s.audit = a
	return s
}

func (s *Service) WithNow(now func() time.Time) *Service {
	s.now = now
	return s
}

func (s *Service) WithMentionsEnabled(enabled bool) *Service {
	s.mentionsEnabled = enabled
	return s
}

func (s *Service) WithMentionCandidates(store MentionCandidateStore) *Service {
	s.candidates = store
	return s
}

func (s *Service) WithMentionsRepository(repo MentionsRepository) *Service {
	s.mentionsRepo = repo
	return s
}

func (s *Service) WithMembershipLookup(lookup MembershipActiveLookup) *Service {
	s.memberships = lookup
	return s
}

func (s *Service) WithMentionDisplays(r MentionDisplayResolver) *Service {
	s.mentionDisplays = r
	return s
}

func (s *Service) WithDB(db *sql.DB) *Service {
	s.db = db
	return s
}

// ListComments returns alive comments + BE capabilities.
func (s *Service) ListComments(ctx context.Context, sub wff.Subject, recordID, stepCode string, page, pageSize int) (*ListResponse, error) {
	if err := s.deadline.AuthorizeView(ctx, sub); err != nil {
		return nil, err
	}
	wf, states, stepStatus, err := s.loadStep(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	owner := ownerFrom(wf, stepCode)
	items, total, err := s.repo.ListAliveByStep(ctx, owner, page, pageSize)
	if err != nil {
		return nil, err
	}
	canCreate := s.computeCanCreate(ctx, sub, wf.RecordStatus, stepStatus)
	canMention := s.computeCanMention()
	capsByComment := make([]CommentCapabilities, len(items))
	for i, c := range items {
		capsByComment[i] = s.computeCommentCaps(ctx, sub, c, wf.RecordStatus)
	}
	names := s.resolveNames(ctx, items)
	out := make([]CommentDTO, 0, len(items))
	for i, c := range items {
		dto := toCommentDTO(c, names, capsByComment[i], nil)
		if s.mentionsEnabled {
			dto.Mentions = s.loadMentionDTOs(ctx, c.CompanyID, c.ID)
		}
		out = append(out, dto)
	}
	_, isCompleted, _ := wff.EvaluateStepAuthority(wf, states, stepCode, s.now())
	return &ListResponse{
		RecordID:           wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           stepCode,
		Completed:          isCompleted,
		Capabilities:       ListCapabilities{CanCreate: canCreate, CanMention: canMention},
		Page:               page,
		PageSize:           pageSize,
		Total:              total,
		Comments:           out,
	}, nil
}

// ListMentionCandidates returns active company members matching q for @mention.
func (s *Service) ListMentionCandidates(ctx context.Context, sub wff.Subject, recordID, stepCode, q string, page, pageSize int) (*MentionCandidatesResponse, error) {
	if !s.mentionsEnabled {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil)
	}
	if err := s.deadline.AuthorizeView(ctx, sub); err != nil {
		return nil, err
	}
	if _, _, _, err := s.loadStep(ctx, sub, recordID, stepCode); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	qLen := RuneLen(q)
	if qLen < CandidateQMinRunes {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "q must be at least 2 characters", nil)
	}
	if qLen > CandidateQMaxRunes {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "q must be at most 100 characters", nil)
	}
	if page < 1 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "page must be >= 1", nil)
	}
	if pageSize < 1 {
		pageSize = CandidatePageDefault
	}
	if pageSize > CandidatePageMax {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "page_size must be <= 50", nil)
	}
	if s.candidates == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil)
	}
	items, total, err := s.candidates.SearchActiveMembers(ctx, sub.CompanyID, q, page, pageSize)
	if err != nil {
		return nil, err
	}
	out := make([]MentionCandidateDTO, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.DisplayName)
		if name == "" {
			name = it.MembershipID
		}
		out = append(out, MentionCandidateDTO{
			MembershipID: it.MembershipID,
			DisplayName:  name,
		})
	}
	return &MentionCandidatesResponse{
		Items:    out,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// CreateComment inserts a comment with actor-isolated Idempotency-Key.
// mentions may be nil/empty for V1 clients.
func (s *Service) CreateComment(ctx context.Context, sub wff.Subject, recordID, stepCode, clientKey, body string, mentions []MentionInput) (*MutateResponse, int, error) {
	clientKey = strings.TrimSpace(clientKey)
	if clientKey == "" {
		return nil, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "Idempotency-Key header required", nil)
	}
	if err := s.deadline.AuthorizeComment(ctx, sub); err != nil {
		return nil, 0, err
	}
	wf, _, stepStatus, err := s.loadStep(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, 0, err
	}
	if !isRecordMutateAllowed(wf.RecordStatus) {
		return nil, 0, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is locked for comments", nil)
	}
	if !isStepCreateAllowed(stepStatus) {
		return nil, 0, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "step does not allow new comments", nil)
	}
	body, err = validateBody(body)
	if err != nil {
		return nil, 0, err
	}

	if MentionsNonEmpty(mentions) && !s.mentionsEnabled {
		return nil, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil)
	}

	var canonical []CanonicalMention
	canonicalJSON := "[]"
	if MentionsNonEmpty(mentions) {
		canonical, canonicalJSON, err = ValidateAndCanonicalizeMentions(ctx, sub.CompanyID, body, mentions, s.memberships)
		if err != nil {
			return nil, 0, err
		}
	}

	storageKey := buildIdempotencyStorageKey(sub.CompanyID, sub.MembershipID, wf.RecordID, stepCode, clientKey)
	reqHash := HashCreateRequest(body, canonicalJSON)

	var idemRes idempotency.Result
	if s.idem != nil {
		idemRes, err = s.idem.TryReserve(ctx, idempotency.Params{
			CompanyID:   sub.CompanyID,
			Scope:       IdempotencyScope,
			Key:         storageKey,
			RequestHash: reqHash,
		})
		if err != nil {
			return nil, 0, err
		}
		if idemRes.Replay {
			var replay MutateResponse
			if err := json.Unmarshal(idemRes.ReplayBody, &replay); err != nil {
				return nil, 0, fmt.Errorf("decode idempotent replay: %w", err)
			}
			status := idemRes.ReplayHTTPStatus
			if status == 0 {
				status = http.StatusCreated
			}
			return &replay, status, nil
		}
		if idemRes.Conflict {
			return nil, 0, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "idempotency conflict or request in progress", nil)
		}
	}

	now := s.now().UTC()
	comment := Comment{
		ID:                 uuid.NewString(),
		CompanyID:          ownerFrom(wf, stepCode).CompanyID,
		DisclosureRecordID: wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           strings.TrimSpace(stepCode),
		AuthorUserID:       sub.UserID,
		AuthorMembershipID: sub.MembershipID,
		Body:               body,
		CreatedAt:          now,
	}
	if err := s.persistCommentCreate(ctx, comment, canonical, now); err != nil {
		if s.idem != nil && idemRes.ReservationID != "" {
			_ = s.idem.Abandon(ctx, idemRes.ReservationID)
		}
		return nil, 0, err
	}

	names := s.resolveNames(ctx, []Comment{comment})
	var mentionDTOs []MentionDTO
	if s.mentionsEnabled {
		mentionDTOs = s.loadMentionDTOs(ctx, comment.CompanyID, comment.ID)
	}
	dto := toCommentDTO(comment, names, CommentCapabilities{CanEdit: true, CanDelete: true}, mentionDTOs)
	resp := &MutateResponse{Comment: dto}

	if s.idem != nil && idemRes.ReservationID != "" {
		bodyJSON, mErr := json.Marshal(resp)
		if mErr != nil {
			return nil, 0, mErr
		}
		env, mErr := json.Marshal(idempotency.Envelope{HTTPStatus: http.StatusCreated, Body: bodyJSON})
		if mErr != nil {
			return nil, 0, mErr
		}
		if cErr := s.idem.Complete(ctx, idemRes.ReservationID, env); cErr != nil {
			// Comment exists; reservation may stay in_progress. Do not claim exactly-once.
			s.log.Error("workflow step comment idempotency complete failed",
				slog.String("reservation_id", idemRes.ReservationID),
				slog.String("comment_id", comment.ID),
				slog.String("err", cErr.Error()),
			)
		}
	}

	metaExtra := map[string]any{"body_length": len(comment.Body)}
	if len(canonical) > 0 {
		ids := make([]string, len(canonical))
		for i, m := range canonical {
			ids[i] = m.MembershipID
		}
		metaExtra["mention_membership_ids"] = ids
	}
	s.appendCommentAudit(ctx, AuditActionCreate, comment.ID, sub, commentAuditMeta{
		CompanyID:          comment.CompanyID,
		DisclosureRecordID: comment.DisclosureRecordID,
		WorkflowInstanceID: comment.WorkflowInstanceID,
		StepCode:           comment.StepCode,
	}, metaExtra)

	// AFTER_COMMIT_BEST_EFFORT: notify only after persist succeeded; never on replay.
	s.notifyCommentMentionRecipients(ctx, mentionNotifyContext{
		CompanyID:          comment.CompanyID,
		CommentID:          comment.ID,
		RecordID:           comment.DisclosureRecordID,
		WorkflowInstanceID: comment.WorkflowInstanceID,
		StepCode:           comment.StepCode,
		AuthorUserID:       comment.AuthorUserID,
		AuthorMembershipID: comment.AuthorMembershipID,
	}, membershipIDsFromCanonical(canonical))

	return resp, http.StatusCreated, nil
}

// UpdateComment patches body when author≤24h+deadline.comment or rbac.manage+deadline.view.
// When mentions feature is ON, mentions omitted/[] clears mention rows.
func (s *Service) UpdateComment(ctx context.Context, sub wff.Subject, recordID, stepCode, commentID, body string, mentions []MentionInput) (*MutateResponse, error) {
	wf, _, _, err := s.loadStep(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	if isRecordTerminalFrozen(wf.RecordStatus) || !isRecordMutateAllowed(wf.RecordStatus) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is locked for comments", nil)
	}
	body, err = validateBody(body)
	if err != nil {
		return nil, err
	}
	if MentionsNonEmpty(mentions) && !s.mentionsEnabled {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil)
	}
	owner := ownerFrom(wf, stepCode)
	existing, err := s.repo.GetByIDInContext(ctx, owner, commentID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if existing == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	}
	if !existing.IsAlive() {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	}
	if err := s.authorizeMutateComment(ctx, sub, *existing); err != nil {
		return nil, err
	}

	var canonical []CanonicalMention
	if s.mentionsEnabled {
		canonical, _, err = ValidateAndCanonicalizeMentions(ctx, sub.CompanyID, body, mentions, s.memberships)
		if err != nil {
			return nil, err
		}
	}

	oldMentionSet := map[string]struct{}{}
	if s.mentionsEnabled && s.mentionsRepo != nil {
		oldRows, lerr := s.mentionsRepo.ListByComment(ctx, existing.CompanyID, existing.ID)
		if lerr != nil {
			return nil, lerr
		}
		oldMentionSet = membershipIDsFromMentions(oldRows)
	}

	now := s.now().UTC()
	if err := s.persistCommentUpdate(ctx, owner, commentID, body, now, canonical, s.mentionsEnabled); err != nil {
		return nil, mapRepoErr(err)
	}
	existing.Body = body
	existing.UpdatedAt = &now
	names := s.resolveNames(ctx, []Comment{*existing})
	caps := s.computeCommentCaps(ctx, sub, *existing, wf.RecordStatus)
	var mentionDTOs []MentionDTO
	if s.mentionsEnabled {
		mentionDTOs = s.loadMentionDTOs(ctx, existing.CompanyID, existing.ID)
	}
	resp := &MutateResponse{Comment: toCommentDTO(*existing, names, caps, mentionDTOs)}
	metaExtra := map[string]any{"body_length": len(body)}
	if s.mentionsEnabled {
		ids := make([]string, len(canonical))
		for i, m := range canonical {
			ids[i] = m.MembershipID
		}
		metaExtra["mention_membership_ids"] = ids
	}
	s.appendCommentAudit(ctx, AuditActionEdit, existing.ID, sub, commentAuditMeta{
		CompanyID:          existing.CompanyID,
		DisclosureRecordID: existing.DisclosureRecordID,
		WorkflowInstanceID: existing.WorkflowInstanceID,
		StepCode:           existing.StepCode,
	}, metaExtra)

	// AFTER_COMMIT_BEST_EFFORT: notify only newly added memberships.
	if s.mentionsEnabled {
		s.notifyCommentMentionRecipients(ctx, mentionNotifyContext{
			CompanyID:          existing.CompanyID,
			CommentID:          existing.ID,
			RecordID:           existing.DisclosureRecordID,
			WorkflowInstanceID: existing.WorkflowInstanceID,
			StepCode:           existing.StepCode,
			AuthorUserID:       existing.AuthorUserID,
			AuthorMembershipID: existing.AuthorMembershipID,
		}, newMentionMembershipIDs(canonical, oldMentionSet))
	}

	return resp, nil
}

// DeleteComment soft-deletes a comment.
func (s *Service) DeleteComment(ctx context.Context, sub wff.Subject, recordID, stepCode, commentID string) error {
	wf, _, _, err := s.loadStep(ctx, sub, recordID, stepCode)
	if err != nil {
		return err
	}
	if isRecordTerminalFrozen(wf.RecordStatus) || !isRecordMutateAllowed(wf.RecordStatus) {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is locked for comments", nil)
	}
	owner := ownerFrom(wf, stepCode)
	existing, err := s.repo.GetByIDInContext(ctx, owner, commentID)
	if err != nil {
		return mapRepoErr(err)
	}
	if existing == nil {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	}
	if !existing.IsAlive() {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "comment already deleted", nil)
	}
	if err := s.authorizeMutateComment(ctx, sub, *existing); err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.repo.SoftDelete(ctx, owner, commentID, sub.MembershipID, now); err != nil {
		return mapRepoErr(err)
	}
	s.appendCommentAudit(ctx, AuditActionDelete, existing.ID, sub, commentAuditMeta{
		CompanyID:          existing.CompanyID,
		DisclosureRecordID: existing.DisclosureRecordID,
		WorkflowInstanceID: existing.WorkflowInstanceID,
		StepCode:           existing.StepCode,
	}, nil)
	return nil
}

func (s *Service) loadStep(ctx context.Context, sub wff.Subject, recordID, stepCode string) (wff.WorkflowContext, map[string]wff.StepState, string, error) {
	wf, err := s.deadline.LoadWorkflowForRecord(ctx, sub, recordID)
	if err != nil {
		return wff.WorkflowContext{}, nil, "", err
	}
	stepCode = strings.TrimSpace(stepCode)
	states, err := s.deadline.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return wff.WorkflowContext{}, nil, "", err
	}
	status, err := wff.ClassifyWorkflowStepStatus(wf, states, stepCode, s.now())
	if err != nil {
		return wff.WorkflowContext{}, nil, "", err
	}
	return wf, states, status, nil
}

func (s *Service) computeCanCreate(ctx context.Context, sub wff.Subject, recordStatus, stepStatus string) bool {
	if !isRecordMutateAllowed(recordStatus) {
		return false
	}
	if !isStepCreateAllowed(stepStatus) {
		return false
	}
	if err := s.deadline.AuthorizeComment(ctx, sub); err != nil {
		return false
	}
	return true
}

// computeCanMention is BE-owned: flag ON after successful view+scope+step load.
func (s *Service) computeCanMention() bool {
	return s.mentionsEnabled && s.candidates != nil
}

func (s *Service) computeCommentCaps(ctx context.Context, sub wff.Subject, c Comment, recordStatus string) CommentCapabilities {
	if isRecordTerminalFrozen(recordStatus) || !isRecordMutateAllowed(recordStatus) {
		return CommentCapabilities{}
	}
	if err := s.authorizeMutateComment(ctx, sub, c); err != nil {
		return CommentCapabilities{}
	}
	return CommentCapabilities{CanEdit: true, CanDelete: true}
}

func (s *Service) authorizeMutateComment(ctx context.Context, sub wff.Subject, c Comment) error {
	now := s.now()
	isAuthor := c.AuthorMembershipID == sub.MembershipID && c.AuthorUserID == sub.UserID
	withinWindow := now.Sub(c.CreatedAt) <= EditWindow
	if isAuthor && withinWindow {
		if err := s.deadline.AuthorizeComment(ctx, sub); err == nil {
			return nil
		}
	}
	hasManage, err := s.deadline.HasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if hasManage {
		if err := s.deadline.AuthorizeView(ctx, sub); err == nil {
			return nil
		}
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "not allowed to mutate this comment", nil)
}

func (s *Service) resolveNames(ctx context.Context, items []Comment) map[string]string {
	out := map[string]string{}
	if s.names == nil || len(items) == 0 {
		for _, c := range items {
			out[c.AuthorUserID] = c.AuthorUserID
		}
		return out
	}
	ids := make([]string, 0, len(items))
	for _, c := range items {
		ids = append(ids, c.AuthorUserID)
	}
	resolved, err := s.names.ResolveDisplayNames(ctx, ids)
	if err != nil {
		s.log.Warn("resolve comment author display names failed", slog.String("err", err.Error()))
	}
	for _, c := range items {
		if name, ok := resolved[c.AuthorUserID]; ok && strings.TrimSpace(name) != "" {
			out[c.AuthorUserID] = name
		} else {
			out[c.AuthorUserID] = c.AuthorUserID
		}
	}
	return out
}

func ownerFrom(wf wff.WorkflowContext, stepCode string) OwnerContext {
	return OwnerContext{
		CompanyID:          wf.CompanyID,
		DisclosureRecordID: wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           strings.TrimSpace(stepCode),
	}
}

func validateBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "body is required", nil)
	}
	if len([]rune(body)) > MaxBodyLength {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "body exceeds 4000 characters", nil)
	}
	return body, nil
}

func buildIdempotencyStorageKey(companyID, membershipID, recordID, stepCode, clientKey string) string {
	raw := strings.Join([]string{
		IdempotencyScope,
		strings.TrimSpace(companyID),
		strings.TrimSpace(membershipID),
		strings.TrimSpace(recordID),
		strings.TrimSpace(stepCode),
		strings.TrimSpace(clientKey),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Service) persistCommentCreate(ctx context.Context, comment Comment, canonical []CanonicalMention, now time.Time) error {
	rows := canonicalToMentionRows(comment.CompanyID, comment.ID, canonical, now)
	return s.runWriteTx(ctx, func(tx DBTX) error {
		if err := s.repo.InsertInTx(ctx, tx, comment); err != nil {
			return err
		}
		if s.mentionsRepo == nil {
			if len(rows) > 0 {
				return fmt.Errorf("mentions repository not configured")
			}
			return nil
		}
		return s.mentionsRepo.ReplaceForComment(ctx, tx, comment.CompanyID, comment.ID, rows)
	})
}

func (s *Service) persistCommentUpdate(ctx context.Context, owner OwnerContext, commentID, body string, now time.Time, canonical []CanonicalMention, replaceMentions bool) error {
	rows := canonicalToMentionRows(owner.CompanyID, commentID, canonical, now)
	return s.runWriteTx(ctx, func(tx DBTX) error {
		if err := s.repo.UpdateBodyInTx(ctx, tx, owner, commentID, body, now); err != nil {
			return err
		}
		if !replaceMentions || s.mentionsRepo == nil {
			return nil
		}
		return s.mentionsRepo.ReplaceForComment(ctx, tx, owner.CompanyID, commentID, rows)
	})
}

func (s *Service) runWriteTx(ctx context.Context, fn func(tx DBTX) error) error {
	if s.db != nil {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin comment+mentions tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := fn(tx); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit comment+mentions tx: %w", err)
		}
		return nil
	}

	// Memory path: snapshot comments + mentions; restore both on failure.
	var commentSnap map[string]Comment
	var mentionSnap map[string][]Mention
	if mem, ok := s.repo.(*MemoryRepository); ok {
		commentSnap = mem.SnapshotComments()
	}
	if memM, ok := s.mentionsRepo.(*MemoryMentionsRepository); ok {
		mentionSnap = memM.SnapshotMentions()
	}
	if err := fn(nil); err != nil {
		if mem, ok := s.repo.(*MemoryRepository); ok && commentSnap != nil {
			mem.RestoreComments(commentSnap)
		}
		if memM, ok := s.mentionsRepo.(*MemoryMentionsRepository); ok && mentionSnap != nil {
			memM.RestoreMentions(mentionSnap)
		}
		return err
	}
	return nil
}

func canonicalToMentionRows(companyID, commentID string, canonical []CanonicalMention, now time.Time) []Mention {
	out := make([]Mention, 0, len(canonical))
	for _, c := range canonical {
		out = append(out, Mention{
			ID:                    uuid.NewString(),
			CompanyID:             companyID,
			CommentID:             commentID,
			MentionedMembershipID: c.MembershipID,
			StartOffset:           c.Start,
			EndOffset:             c.End,
			CreatedAt:             now,
		})
	}
	return out
}

func (s *Service) loadMentionDTOs(ctx context.Context, companyID, commentID string) []MentionDTO {
	if s.mentionsRepo == nil {
		return []MentionDTO{}
	}
	rows, err := s.mentionsRepo.ListByComment(ctx, companyID, commentID)
	if err != nil {
		s.log.Warn("list comment mentions failed", slog.String("comment_id", commentID), slog.String("err", err.Error()))
		return []MentionDTO{}
	}
	if len(rows) == 0 {
		return []MentionDTO{}
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.MentionedMembershipID
	}
	displays := map[string]MentionDisplayInfo{}
	if s.mentionDisplays != nil {
		resolved, rerr := s.mentionDisplays.ResolveMentionDisplays(ctx, companyID, ids)
		if rerr != nil {
			s.log.Warn("resolve mention displays failed", slog.String("err", rerr.Error()))
		} else {
			displays = resolved
		}
	}
	out := make([]MentionDTO, 0, len(rows))
	for _, r := range rows {
		name := InactiveMentionDisplayName
		if info, ok := displays[r.MentionedMembershipID]; ok {
			if info.Active && strings.TrimSpace(info.DisplayName) != "" {
				name = info.DisplayName
			} else {
				name = InactiveMentionDisplayName
			}
		}
		out = append(out, MentionDTO{
			MembershipID: r.MentionedMembershipID,
			DisplayName:  name,
			Start:        r.StartOffset,
			End:          r.EndOffset,
		})
	}
	return out
}

func toCommentDTO(c Comment, names map[string]string, caps CommentCapabilities, mentions []MentionDTO) CommentDTO {
	display := names[c.AuthorUserID]
	if display == "" {
		display = c.AuthorUserID
	}
	var updated *string
	if c.UpdatedAt != nil {
		s := c.UpdatedAt.UTC().Format(time.RFC3339)
		updated = &s
	}
	dto := CommentDTO{
		CommentID: c.ID,
		Body:      c.Body,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: updated,
		Author: AuthorDTO{
			UserID:       c.AuthorUserID,
			MembershipID: c.AuthorMembershipID,
			DisplayName:  display,
		},
		Capabilities: caps,
	}
	if mentions != nil {
		dto.Mentions = mentions
	}
	return dto
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	var pathErr errPathMismatch
	if errors.As(err, &pathErr) || errors.Is(err, errPathMismatch{}) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	}
	var nf errNotFound
	if errors.As(err, &nf) || errors.Is(err, errNotFound{}) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	}
	var ad errAlreadyDeleted
	if errors.As(err, &ad) || errors.Is(err, errAlreadyDeleted{}) {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "comment already deleted", nil)
	}
	// pointer/value sentinel compare for typed empties
	switch err.(type) {
	case errPathMismatch:
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	case errNotFound:
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "comment not found", nil)
	case errAlreadyDeleted:
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "comment already deleted", nil)
	}
	return err
}
