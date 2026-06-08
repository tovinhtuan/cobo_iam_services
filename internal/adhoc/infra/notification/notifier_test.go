package notification_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	adhocnotif "github.com/cobo/cobo_iam_services/internal/adhoc/infra/notification"
	notifapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
)

// ---- fakes -----------------------------------------------------------------

// noopInApp records nothing and never errors; in-app delivery is out of scope
// for the Batch 2 email cutover and must keep working unchanged either way.
type noopInApp struct{}

func (noopInApp) CreateForUser(ctx context.Context, userID, companyID, kind, title, body string, resourceType, resourceID *string) error {
	return nil
}

// recordingDeliveryAdapter stands in for the legacy DeliveryAdapter (SMTP).
// It records every message and the context each Send was invoked with, so
// tests can assert (a) the legacy path keeps sending to the REAL recipient in
// every case, and (b) the request-scoped ctx flows through synchronously
// (CF-12 closure — no goroutine severs it via context.Background()).
type recordingDeliveryAdapter struct {
	mu       sync.Mutex
	messages []notifapp.DeliveryMessage
	ctxs     []context.Context
}

func (d *recordingDeliveryAdapter) Send(ctx context.Context, msg notifapp.DeliveryMessage) (notifapp.DeliveryResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messages = append(d.messages, msg)
	d.ctxs = append(d.ctxs, ctx)
	return notifapp.DeliveryResult{ProviderMessageID: "log-only-1", Provider: "log_only"}, nil
}

func (d *recordingDeliveryAdapter) snapshot() []notifapp.DeliveryMessage {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]notifapp.DeliveryMessage, len(d.messages))
	copy(out, d.messages)
	return out
}

// staticRegistry/staticRenderer stand in for the real template registry +
// renderer. The adhoc.* template keys are not registered in
// notificationregistry.NewEmbedRegistry(), so the durable pipeline (which also
// resolves+renders inside EmailNotificationService.DispatchEmail) needs a
// minimal fake that resolves any key. They also record the ctx they saw, for
// the same CF-12 propagation assertion as recordingDeliveryAdapter.
type staticRegistry struct {
	mu   sync.Mutex
	ctxs []context.Context
}

func (r *staticRegistry) Resolve(ctx context.Context, key, locale string) (notifapp.ResolvedTemplate, error) {
	r.mu.Lock()
	r.ctxs = append(r.ctxs, ctx)
	r.mu.Unlock()
	return notifapp.ResolvedTemplate{
		Key:      key,
		Locale:   locale,
		Subject:  "Subject for " + key,
		TextBody: "Body for {{.proposal_id}}",
		HTMLBody: "<p>Body for {{.proposal_id}}</p>",
	}, nil
}

type staticRenderer struct{}

func (staticRenderer) Render(t notifapp.ResolvedTemplate, vars map[string]any) (notifapp.RenderedEmail, error) {
	return notifapp.RenderedEmail{Subject: t.Subject, TextBody: t.TextBody, HTMLBody: t.HTMLBody}, nil
}

// seqIDGen mirrors notification/app's test helper — deterministic IDs.
type seqIDGen struct{ n int }

func (g *seqIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("email-notif-%03d", g.n)
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// spyMetrics is a local Metrics spy (adhocapp.Metrics is consumed, not the
// app_test package's private spyMetrics, which lives in a different package).
type spyMetrics struct {
	mu              sync.Mutex
	shadowOutcomes  []shadowOutcomeCall
	transitionCalls int
}

type shadowOutcomeCall struct {
	companyID string
	outcome   string
}

func (s *spyMetrics) RecordTransition(companyID, fromStatus, toStatus string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transitionCalls++
}

func (s *spyMetrics) RecordEmailShadowOutcome(companyID, outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shadowOutcomes = append(s.shadowOutcomes, shadowOutcomeCall{companyID: companyID, outcome: outcome})
}

func (s *spyMetrics) snapshot() []shadowOutcomeCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]shadowOutcomeCall, len(s.shadowOutcomes))
	copy(out, s.shadowOutcomes)
	return out
}

// ---- test harness -----------------------------------------------------------

const (
	testCompanyID  = "company-001"
	testProposalID = "proposal-001"
	realRecipient  = "real.recipient@example.com"
	shadowSink     = "shadow-sink@example.internal"
)

type testCtxKey struct{}

// harness bundles every collaborator + fake the notifier needs, so each test
// only has to set the three cutover flags and (optionally) pre-seed the repo.
type harness struct {
	delivery *recordingDeliveryAdapter
	registry *staticRegistry
	repo     *inmemory.EmailNotificationRepository
	notifSvc *notifapp.EmailNotificationService
	metrics  *spyMetrics
}

func newHarness() *harness {
	repo := inmemory.NewEmailNotificationRepository()
	registry := &staticRegistry{}
	notifSvc := notifapp.NewEmailNotificationService(
		repo, registry, staticRenderer{}, &seqIDGen{},
		fixedClock(time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)),
	)
	return &harness{
		delivery: &recordingDeliveryAdapter{},
		registry: registry,
		repo:     repo,
		notifSvc: notifSvc,
		metrics:  &spyMetrics{},
	}
}

// build constructs the notifier under test, wired to the harness's real
// in-memory durable pipeline (h.notifSvc).
func (h *harness) build(shadowMode, outboxEnabled bool, shadowRecipient string) adhocapp.ProposalNotifier {
	return adhocnotif.New(
		noopInApp{}, h.delivery, h.registry, staticRenderer{}, h.notifSvc,
		"https://portal.example.vn", shadowMode, outboxEnabled, shadowRecipient,
		h.metrics, nil,
	)
}

// buildWithNilNotificationService simulates "durable pipeline unavailable"
// (nil *EmailNotificationService — exactly how httpserver wires the notifier
// when pool/outboxSQL are absent, per server.go's adhocEmailNotificationService
// guard), for TC-Shadow-05.
func (h *harness) buildWithNilNotificationService(shadowMode, outboxEnabled bool, shadowRecipient string) adhocapp.ProposalNotifier {
	return adhocnotif.New(
		noopInApp{}, h.delivery, h.registry, staticRenderer{}, nil,
		"https://portal.example.vn", shadowMode, outboxEnabled, shadowRecipient,
		h.metrics, nil,
	)
}

func testProposal() adhocapp.ProposalDTO {
	return adhocapp.ProposalDTO{
		ProposalID: testProposalID,
		CompanyID:  testCompanyID,
		ChangeNote: "Điều chỉnh ngày công bố",
		RecordID:   "record-001",
	}
}

func testCreator() adhocapp.MemberInfo {
	return adhocapp.MemberInfo{
		UserID:       "user-001",
		MembershipID: "membership-001",
		Email:        realRecipient,
		FullName:     "Nguyễn Văn A",
	}
}

// ---- TC-Shadow-01: Case B happy path — dual pipeline, no duplicate to user --

func TestShadowMode_DualSendsLegacyRealDurableShadow(t *testing.T) {
	h := newHarness()
	notifier := h.build(true, false, shadowSink)

	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	legacy := h.delivery.snapshot()
	if len(legacy) != 1 {
		t.Fatalf("legacy sends = %d, want 1", len(legacy))
	}
	if legacy[0].To != realRecipient {
		t.Fatalf("legacy recipient = %q, want real recipient %q (Case B: legacy MUST keep real recipient)", legacy[0].To, realRecipient)
	}

	rows := h.repo.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("durable rows = %d, want 1", len(rows))
	}
	if rows[0].RecipientEmail != shadowSink {
		t.Fatalf("durable recipient = %q, want shadow sink %q (Case B: durable MUST NOT reach the real user)", rows[0].RecipientEmail, shadowSink)
	}
	if rows[0].SourceAggregateID != testProposalID {
		t.Fatalf("durable SourceAggregateID = %q, want %q (rewrite must not touch audit fields)", rows[0].SourceAggregateID, testProposalID)
	}

	wantIdemKey := fmt.Sprintf("adhoc.proposal_approved.%s.%s", testProposalID, testCreator().MembershipID)
	if rows[0].IdempotencyKey != wantIdemKey {
		t.Fatalf("idempotency key = %q, want %q (rewrite must not touch idempotency key)", rows[0].IdempotencyKey, wantIdemKey)
	}

	outcomes := h.metrics.snapshot()
	if len(outcomes) != 1 || outcomes[0].outcome != "match" {
		t.Fatalf("shadow outcomes = %+v, want exactly one \"match\"", outcomes)
	}
	if outcomes[0].companyID != testCompanyID {
		t.Fatalf("shadow outcome companyID = %q, want %q", outcomes[0].companyID, testCompanyID)
	}
}

// ---- TC-Shadow-02: empty shadow recipient — durable dispatch is skipped -----

func TestShadowMode_EmptyShadowRecipientSkipsDurableDispatch(t *testing.T) {
	h := newHarness()
	notifier := h.build(true, false, "")

	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	legacy := h.delivery.snapshot()
	if len(legacy) != 1 || legacy[0].To != realRecipient {
		t.Fatalf("legacy sends = %+v, want exactly one send to %q", legacy, realRecipient)
	}
	if rows := h.repo.Snapshot(); len(rows) != 0 {
		t.Fatalf("durable rows = %d, want 0 (no shadow recipient configured -> no durable dispatch)", len(rows))
	}
	if outcomes := h.metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("shadow outcomes = %+v, want none", outcomes)
	}
}

// ---- TC-Shadow-03: durable record never carries the real recipient ---------

func TestShadowMode_DurableRecordNeverAddressedToRealUser(t *testing.T) {
	h := newHarness()
	notifier := h.build(true, false, shadowSink)

	creator := testCreator()
	notifier.NotifyCreatorRejected(context.Background(), testProposal(), creator)

	for _, row := range h.repo.Snapshot() {
		if row.RecipientEmail == creator.Email {
			t.Fatalf("durable row addressed to real user %q — Shadow Mode must never duplicate-send to real recipients: %+v", creator.Email, row)
		}
		if row.RecipientEmail != shadowSink {
			t.Fatalf("durable row recipient = %q, want shadow sink %q", row.RecipientEmail, shadowSink)
		}
	}
}

// ---- TC-Shadow-04: idempotency-key collision is reported as "mismatch" -----

func TestShadowMode_IdempotencyCollisionReportsMismatch(t *testing.T) {
	h := newHarness()

	// Pre-seed the repo with a record under the EXACT idempotency key this
	// dispatch will compute, but for a DIFFERENT proposal — i.e. a genuine
	// collision (the in-app manifestation of duplicate idempotency_key /
	// COUNT(*) > 1, observable here because DispatchEmail short-circuits to
	// the existing row instead of inserting a new one).
	collidingKey := fmt.Sprintf("adhoc.proposal_approved.%s.%s", testProposalID, testCreator().MembershipID)
	seedCtx := context.Background()
	_, err := h.notifSvc.DispatchEmail(seedCtx, notifapp.DispatchEmailRequest{
		To:                  shadowSink,
		TemplateKey:         "adhoc.proposal_approved",
		Locale:              "vi",
		Variables:           map[string]any{"proposal_id": "OTHER-PROPOSAL-999"},
		IdempotencyKey:      collidingKey,
		TriggeredByUserID:   "system",
		SourceEventType:     "adhoc.proposal_approved",
		SourceAggregateType: "ad_hoc_proposal",
		SourceAggregateID:   "OTHER-PROPOSAL-999", // <- different from testProposalID
		CompanyID:           testCompanyID,
	})
	if err != nil {
		t.Fatalf("seed dispatch error = %v", err)
	}

	notifier := h.build(true, false, shadowSink)
	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	outcomes := h.metrics.snapshot()
	if len(outcomes) != 1 || outcomes[0].outcome != "mismatch" {
		t.Fatalf("shadow outcomes = %+v, want exactly one \"mismatch\" (colliding idempotency key resolved to a different proposal's record)", outcomes)
	}

	// The collision must not have produced a second row (DB unique constraint
	// in production / inmemory FindByIdempotencyKey short-circuit in test).
	if rows := h.repo.Snapshot(); len(rows) != 1 {
		t.Fatalf("durable rows = %d, want 1 (collision must short-circuit, not insert a duplicate)", len(rows))
	}
}

// ---- TC-Shadow-05: durable pipeline unavailable — shadow comparison is a no-op

func TestShadowMode_NilNotificationServiceSkipsComparisonWithoutBreakingLegacy(t *testing.T) {
	h := newHarness()
	notifier := h.buildWithNilNotificationService(true, false, shadowSink)

	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	legacy := h.delivery.snapshot()
	if len(legacy) != 1 || legacy[0].To != realRecipient {
		t.Fatalf("legacy sends = %+v, want exactly one send to %q even when durable pipeline is unavailable", legacy, realRecipient)
	}
	if outcomes := h.metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("shadow outcomes = %+v, want none (dispatchDurable returns nil,nil when notificationService == nil)", outcomes)
	}
}

// ---- TC-Rollback-01: default flags == byte-identical legacy-only behavior --

func TestRollback_DefaultFlagsAreLegacyOnly(t *testing.T) {
	h := newHarness()
	notifier := h.build(false, false, "")

	notifier.NotifyFocalsForReview(context.Background(), testProposal(), []adhocapp.MemberInfo{testCreator()})

	legacy := h.delivery.snapshot()
	if len(legacy) != 1 || legacy[0].To != realRecipient {
		t.Fatalf("legacy sends = %+v, want exactly one send to %q (Case A unchanged)", legacy, realRecipient)
	}
	if rows := h.repo.Snapshot(); len(rows) != 0 {
		t.Fatalf("durable rows = %d, want 0 — durable pipeline must never run when both flags are off", len(rows))
	}
	if outcomes := h.metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("shadow outcomes = %+v, want none — comparison only runs in the shadow branch", outcomes)
	}
}

// ---- TC-Rollback-02: outboxEnabled wins — durable-only, no dual-send -------

func TestRollback_OutboxEnabledWinsOverShadowMode(t *testing.T) {
	h := newHarness()
	// Both flags on (Case D): "OutboxEnabled wins" — durable only, real
	// recipient, legacy NOT invoked, no shadow comparison metric.
	notifier := h.build(true, true, shadowSink)

	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	if legacy := h.delivery.snapshot(); len(legacy) != 0 {
		t.Fatalf("legacy sends = %+v, want none — outboxEnabled must suppress the legacy path entirely", legacy)
	}
	rows := h.repo.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("durable rows = %d, want 1", len(rows))
	}
	if rows[0].RecipientEmail != realRecipient {
		t.Fatalf("durable recipient = %q, want REAL recipient %q (Case D: durable is the sole authoritative sender)", rows[0].RecipientEmail, realRecipient)
	}
	if outcomes := h.metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("shadow outcomes = %+v, want none — comparison metric only fires from the shadow branch, never Case C/D", outcomes)
	}
}

// ---- Regression: Case C (outbox on, shadow off) behaves like Case D --------

func TestRegression_OutboxOnlyDurableRealRecipientNoShadowMetric(t *testing.T) {
	h := newHarness()
	notifier := h.build(false, true, "")

	notifier.NotifyControllerForReview(context.Background(), testProposal(), testCreator())

	if legacy := h.delivery.snapshot(); len(legacy) != 0 {
		t.Fatalf("legacy sends = %+v, want none in Case C", legacy)
	}
	rows := h.repo.Snapshot()
	if len(rows) != 1 || rows[0].RecipientEmail != realRecipient {
		t.Fatalf("durable rows = %+v, want exactly one row addressed to the real recipient %q", rows, realRecipient)
	}
	if outcomes := h.metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("shadow outcomes = %+v, want none in Case C", outcomes)
	}
}

// ---- CF-12 closure: request-scoped ctx propagates synchronously ------------
//
// Prior to Batch 2, dispatchNotificationAsync spawned a goroutine seeded with
// context.Background(), severing the request-scoped context (deadlines,
// trace IDs, cancellation). This test proves the live ctx — carrying a
// caller-supplied value — reaches both the legacy delivery adapter and the
// durable pipeline's template registry synchronously, in the same call.
// A goroutine handed context.Background() could never observe this value.
func TestCF12_RequestScopedContextPropagatesSynchronously(t *testing.T) {
	h := newHarness()
	notifier := h.build(true, false, shadowSink)

	ctx := context.WithValue(context.Background(), testCtxKey{}, "trace-cf12-001")
	notifier.NotifyCreatorApproved(ctx, testProposal(), testCreator())

	// Legacy path (sendLegacyEmail -> DeliveryAdapter.Send) must see the value.
	legacyCtxs := h.delivery.ctxs
	if len(legacyCtxs) != 1 {
		t.Fatalf("legacy adapter invocations = %d, want 1", len(legacyCtxs))
	}
	if v, _ := legacyCtxs[0].Value(testCtxKey{}).(string); v != "trace-cf12-001" {
		t.Fatalf("legacy adapter ctx value = %q, want %q — request-scoped ctx must reach the legacy send synchronously (CF-12)", v, "trace-cf12-001")
	}

	// Durable path (dispatchDurable -> DispatchEmail -> TemplateRegistry.Resolve)
	// must ALSO see the value — proving sendEmail/dispatchDurable never wrap the
	// call in a goroutine seeded with context.Background().
	h.registry.mu.Lock()
	regCtxs := append([]context.Context(nil), h.registry.ctxs...)
	h.registry.mu.Unlock()
	if len(regCtxs) == 0 {
		t.Fatalf("template registry was never invoked by the durable pipeline")
	}
	for i, c := range regCtxs {
		if v, _ := c.Value(testCtxKey{}).(string); v != "trace-cf12-001" {
			t.Fatalf("durable pipeline registry ctx[%d] value = %q, want %q — request-scoped ctx must reach the durable dispatch synchronously (CF-12)", i, v, "trace-cf12-001")
		}
	}

	if ctx.Err() != nil {
		t.Fatalf("unexpected ctx error: %v", ctx.Err())
	}
}

// ---- CF-12 closure: sources contain no detached-context goroutine spawns ---
//
// Static companion to the propagation test above: asserts the literal pattern
// CF-12 closed (a `go` statement launching notifier dispatch seeded with
// context.Background()) is gone from the adhoc service layer. This mirrors
// what a reviewer would grep for and pins the regression at the source level,
// not just observable behavior.
func TestCF12_NoDetachedGoroutineDispatchInServiceSource(t *testing.T) {
	src := readSourceFile(t, "../../app/service.go")
	if idx := indexOf(src, "context.Background()"); idx >= 0 {
		t.Fatalf("internal/adhoc/app/service.go still calls context.Background() at byte offset %d — CF-12 requires threading the request-scoped ctx", idx)
	}
	if idx := indexOf(src, "go dispatchNotificationAsync"); idx >= 0 {
		t.Fatalf("internal/adhoc/app/service.go still spawns dispatchNotificationAsync via goroutine at byte offset %d", idx)
	}
	if idx := indexOf(src, "dispatchNotificationAsync"); idx >= 0 {
		t.Fatalf("internal/adhoc/app/service.go still references dispatchNotificationAsync at byte offset %d — CF-12 requires its removal", idx)
	}
}

// ---- R-1: outbox enabled, notification service nil — fallback to legacy ------

// TestR1_OutboxEnabledNilServiceFallsBackToLegacy verifies that when
// outboxEnabled=true but notificationService=nil (pool/outboxSQL unavailable at
// startup), sendEmail falls back to the legacy DeliveryAdapter.Send path rather
// than silently dropping the email (R-1 fix).
func TestR1_OutboxEnabledNilServiceFallsBackToLegacy(t *testing.T) {
	h := newHarness()
	notifier := h.buildWithNilNotificationService(false, true, "")

	notifier.NotifyCreatorApproved(context.Background(), testProposal(), testCreator())

	legacy := h.delivery.snapshot()
	if len(legacy) != 1 {
		t.Fatalf("legacy delivery.Send call count = %d, want 1 (R-1: nil notificationService must fall back to legacy, not drop)", len(legacy))
	}
	if legacy[0].To != realRecipient {
		t.Fatalf("legacy recipient = %q, want %q (R-1: fallback must deliver to the real recipient)", legacy[0].To, realRecipient)
	}

	rows := h.repo.Snapshot()
	if len(rows) != 0 {
		t.Fatalf("durable rows = %d, want 0 (R-1: nil service must not attempt durable dispatch)", len(rows))
	}
}

// TestR1_ConstructorWarnsWhenOutboxEnabledNilService verifies that New() emits a
// Warn-level log when outboxEnabled=true and notificationService=nil, so operators
// are alerted at startup rather than discovering the misconfiguration on the first
// proposal action (R-1 fix).
func TestR1_ConstructorWarnsWhenOutboxEnabledNilService(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	adhocnotif.New(
		noopInApp{}, &recordingDeliveryAdapter{}, &staticRegistry{}, staticRenderer{}, nil,
		"https://portal.example.vn", false, true, "",
		&spyMetrics{}, log,
	)

	got := buf.String()
	want := "outbox enabled but notification service is nil"
	if !strings.Contains(got, want) {
		t.Fatalf("expected constructor warning containing %q when outboxEnabled=true and notificationService=nil; got: %q", want, got)
	}
}

func readSourceFile(t *testing.T, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(b)
}

func indexOf(haystack, needle string) int {
	return strings.Index(haystack, needle)
}
