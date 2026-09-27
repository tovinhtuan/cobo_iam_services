package workflowdept

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestRetryScheduleMatchesContract(t *testing.T) {
	want := []time.Duration{time.Minute, 3 * time.Minute, 6 * time.Minute, 10 * time.Minute}
	for i, delay := range want {
		got, permanent := RetryAfterFailure(i + 1)
		if permanent || got != delay {
			t.Fatalf("attempt %d delay=%s permanent=%v", i+1, got, permanent)
		}
	}
	if _, permanent := RetryAfterFailure(5); !permanent {
		t.Fatal("attempt 5 must be permanent")
	}
}

func TestClaimAllowsPendingAndRetryableOnly(t *testing.T) {
	for _, status := range []string{SendPending, SendRetryableFailed} {
		next, owned := ClaimToSending(status)
		if !owned || next != SendSending {
			t.Fatalf("%s claim %s %v", status, next, owned)
		}
	}
	for _, status := range []string{SendSending, SendSent, SendUnknown, SendPermanentFailed} {
		if _, owned := ClaimToSending(status); owned {
			t.Fatalf("%s must not be claimed", status)
		}
	}
	if MayCallSMTP(SendSent, "") || MayCallSMTP(SendUnknown, "") || MayCallSMTP(SendPermanentFailed, "") {
		t.Fatal("terminal states must not send")
	}
	if MayCallSMTP(SendSending, "provider-1") {
		t.Fatal("provider id blocks smtp")
	}
}

func TestProviderOutcomes(t *testing.T) {
	if AfterProviderClassified(true, false, false, false) != SendSent {
		t.Fatal("accepted id")
	}
	if AfterProviderClassified(false, false, true, false) != SendUnknown {
		t.Fatal("uncertain")
	}
	if AfterProvider(false, false, true) != SendUnknown {
		t.Fatal("legacy uncertain helper")
	}
	if AfterProviderClassified(false, false, false, false) != SendRetryableFailed {
		t.Fatal("transient")
	}
	if AfterProviderClassified(false, false, false, true) != SendPermanentFailed {
		t.Fatal("permanent smtp")
	}
}

func TestRecipientKeyFailClosedAndNoLogOfKey(t *testing.T) {
	t.Setenv(EnvRecipientKey, "")
	t.Setenv(EnvRecipientKeyVersion, "")
	if _, err := LoadRecipientKey(); err != ErrRecipientKeyMissing {
		t.Fatalf("missing key err=%v", err)
	}
	t.Setenv(EnvRecipientKey, "aaaa")
	t.Setenv(EnvRecipientKeyVersion, "1")
	if _, err := LoadRecipientKey(); err != ErrRecipientKeyMissing {
		t.Fatal("short key must fail closed")
	}
}

func TestFlagOffDoesNotTouchStoreOrSender(t *testing.T) {
	t.Setenv(EnvBindingEnabled, "")
	t.Setenv(EnvEmailBindingEnabled, "")
	if EmailBindingEnabled() {
		t.Fatal("flags must default off")
	}
	store := &panicStore{}
	sender := &countingSender{}
	if err := RunBindingTick(context.Background(), BindingDeps{Store: store, Sender: sender, SMTPHost: "smtp.example"}); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 {
		t.Fatal("sender called while flags off")
	}
}

func TestConcurrentClaimOneOwner(t *testing.T) {
	store := newMemStore(&DeliveryResolution{ResolutionID: "r1", SendStatus: SendPending, CompanyID: "c", OccurrenceID: "o"})
	var wg sync.WaitGroup
	owners := make(chan ClaimOutcome, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		lease := string(rune('a' + i))
		go func() {
			defer wg.Done()
			got, err := store.ClaimDueResolution(context.Background(), lease, time.Now().UTC(), time.Now().UTC().Add(BindingSMTPLease))
			if err != nil {
				t.Errorf("claim %v", err)
				return
			}
			owners <- got.Outcome
		}()
	}
	wg.Wait()
	close(owners)
	claimed := 0
	for outcome := range owners {
		if outcome == ClaimClaimed {
			claimed++
		}
		if outcome != ClaimClaimed && outcome != ClaimNotFound && outcome != ClaimAlreadyOwned {
			t.Fatalf("unexpected %s", outcome)
		}
	}
	if claimed != 1 {
		t.Fatalf("claimed=%d", claimed)
	}
}

func TestBindingPathDoesNotCallSenderOnSafetyStops(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	blob, err := EncryptRecipientEmails(key, 1, "c1", "occ", "res", []byte("a@co.test"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBindingEnabled, "true")
	t.Setenv(EnvEmailBindingEnabled, "true")
	t.Setenv(EnvRecipientKey, "")
	t.Setenv(EnvRecipientKeyVersion, "")

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	sender := &countingSender{}

	t.Run("missing key", func(t *testing.T) {
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, SendStatus: SendPending,
		})
		if err := RunBindingTick(context.Background(), BindingDeps{Store: store, Sender: sender, Now: func() time.Time { return now }, Log: logger}); err != nil {
			t.Fatal(err)
		}
		row := store.must("res")
		if row.SendStatus != SendPermanentFailed || row.ProviderID != "" || row.LastErrorCode != ErrCodeRecipientKeyMissing {
			t.Fatalf("%+v", row)
		}
		if sender.calls != 0 {
			t.Fatal("smtp called without key")
		}
	})

	t.Run("empty smtp", func(t *testing.T) {
		loaded := RecipientKey{Key: key, Version: 1}
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, KeyVersion: 1, SendStatus: SendPending,
		})
		if err := RunBindingTick(context.Background(), BindingDeps{
			Store: store, Sender: sender, Key: &loaded, SMTPHost: "  ", Now: func() time.Time { return now }, Log: logger,
		}); err != nil {
			t.Fatal(err)
		}
		row := store.must("res")
		if row.SendStatus != SendPermanentFailed || row.ProviderID != "" || row.LastErrorCode != ErrCodeSMTPHostEmpty {
			t.Fatalf("%+v", row)
		}
		if sender.calls != 0 {
			t.Fatal("legacy or binding sender called")
		}
	})

	t.Run("decrypt failure", func(t *testing.T) {
		loaded := RecipientKey{Key: bytes.Repeat([]byte{9}, 32), Version: 1}
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, KeyVersion: 1, SendStatus: SendPending,
		})
		if err := RunBindingTick(context.Background(), BindingDeps{
			Store: store, Sender: sender, Key: &loaded, SMTPHost: "smtp.example", Now: func() time.Time { return now }, Log: logger,
		}); err != nil {
			t.Fatal(err)
		}
		if store.must("res").LastErrorCode != ErrCodeRecipientDecryptFail || sender.calls != 0 {
			t.Fatalf("status %s calls %d", store.must("res").SendStatus, sender.calls)
		}
	})

	if bytes.Contains(logs.Bytes(), []byte("@")) || bytes.Contains(logs.Bytes(), key) {
		t.Fatalf("log leaked recipient or key: %s", logs.String())
	}
}

func TestBindingProviderClassification(t *testing.T) {
	key := bytes.Repeat([]byte{4}, 32)
	blob, err := EncryptRecipientEmails(key, 2, "c1", "occ", "res", []byte("a@co.test"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBindingEnabled, "true")
	t.Setenv(EnvEmailBindingEnabled, "true")
	loaded := RecipientKey{Key: key, Version: 2}
	now := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		sender *countingSender
		want   string
		code   string
	}{
		{name: "accepted", sender: &countingSender{id: "smtp-1"}, want: SendSent},
		{name: "uncertain", sender: &countingSender{uncertain: true, err: errTimeout("timeout")}, want: SendUnknown, code: ErrCodeSMTPUncertain},
		{name: "transient", sender: &countingSender{err: errTimeout("421 busy")}, want: SendRetryableFailed, code: ErrCodeSMTPTransient},
		{name: "permanent", sender: &countingSender{permanent: true, err: errTimeout("550 mailbox")}, want: SendPermanentFailed, code: ErrCodeSMTPPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemStore(&DeliveryResolution{
				ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, KeyVersion: 2, SendStatus: SendRetryableFailed,
			})
			if err := RunBindingTick(context.Background(), BindingDeps{
				Store: store, Sender: tc.sender, Key: &loaded, SMTPHost: "smtp.example", Now: func() time.Time { return now },
				Log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
			}); err != nil {
				t.Fatal(err)
			}
			row := store.must("res")
			if row.SendStatus != tc.want {
				t.Fatalf("status %s", row.SendStatus)
			}
			if tc.want == SendSent && row.ProviderID != "smtp-1" {
				t.Fatalf("provider %q", row.ProviderID)
			}
			if tc.want == SendRetryableFailed {
				if row.AttemptCount != 1 || row.NextRetryAt == nil || !row.NextRetryAt.Equal(now.Add(time.Minute)) {
					t.Fatalf("retry %+v", row)
				}
			}
			if tc.code != "" && row.LastErrorCode != tc.code {
				t.Fatalf("code %s", row.LastErrorCode)
			}
			if tc.sender.calls != 1 {
				t.Fatalf("calls %d", tc.sender.calls)
			}
		})
	}
}

func TestAttemptFiveBecomesPermanent(t *testing.T) {
	key := bytes.Repeat([]byte{5}, 32)
	blob, err := EncryptRecipientEmails(key, 1, "c1", "occ", "res", []byte("a@co.test"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBindingEnabled, "true")
	t.Setenv(EnvEmailBindingEnabled, "true")
	loaded := RecipientKey{Key: key, Version: 1}
	store := newMemStore(&DeliveryResolution{
		ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, KeyVersion: 1,
		SendStatus: SendRetryableFailed, AttemptCount: 4,
	})
	sender := &countingSender{err: errTimeout("421 busy")}
	now := time.Now().UTC()
	if err := RunBindingTick(context.Background(), BindingDeps{
		Store: store, Sender: sender, Key: &loaded, SMTPHost: "smtp.example", Now: func() time.Time { return now },
		Log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}); err != nil {
		t.Fatal(err)
	}
	row := store.must("res")
	if row.SendStatus != SendPermanentFailed || row.ProviderID != "" {
		t.Fatalf("%+v", row)
	}
}

func TestReaperMovesStaleSendingToUnknown(t *testing.T) {
	past := time.Now().UTC().Add(-time.Minute)
	store := newMemStore(&DeliveryResolution{
		ResolutionID: "res", SendStatus: SendSending, LeaseID: "old", LeaseUntil: &past,
	})
	n, err := store.ReapStaleSending(context.Background(), time.Now().UTC())
	if err != nil || n != 1 {
		t.Fatalf("reaped %d err %v", n, err)
	}
	if store.must("res").SendStatus != SendUnknown {
		t.Fatal(store.must("res").SendStatus)
	}
	if _, owned := ClaimToSending(store.must("res").SendStatus); owned {
		t.Fatal("unknown was reclaimed")
	}
}

func TestIllegalClaimAndMarkTransitions(t *testing.T) {
	now := time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	t.Run("future retry is not claimed", func(t *testing.T) {
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", SendStatus: SendRetryableFailed, NextRetryAt: &future,
		})
		got, err := store.ClaimDueResolution(context.Background(), "lease-a", now, now.Add(BindingSMTPLease))
		if err != nil || got.Outcome != ClaimNotFound {
			t.Fatalf("%+v err %v", got, err)
		}
		if store.must("res").SendStatus != SendRetryableFailed {
			t.Fatal(store.must("res").SendStatus)
		}
	})

	t.Run("due retry can be claimed once", func(t *testing.T) {
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", SendStatus: SendRetryableFailed, NextRetryAt: &past, LeaseUntil: &past,
		})
		first, err := store.ClaimDueResolution(context.Background(), "lease-a", now, now.Add(BindingSMTPLease))
		if err != nil || first.Outcome != ClaimClaimed || store.must("res").AttemptCount != 0 {
			t.Fatalf("first %+v count %d err %v", first, store.must("res").AttemptCount, err)
		}
		second, err := store.ClaimDueResolution(context.Background(), "lease-b", now, now.Add(BindingSMTPLease))
		if err != nil || second.Outcome == ClaimClaimed {
			t.Fatalf("second %+v err %v", second, err)
		}
	})

	t.Run("live sending lease is not claimed", func(t *testing.T) {
		store := newMemStore(&DeliveryResolution{
			ResolutionID: "res", SendStatus: SendSending, LeaseID: "holder", LeaseUntil: &future,
		})
		got, err := store.ClaimDueResolution(context.Background(), "other", now, now.Add(BindingSMTPLease))
		if err != nil || got.Outcome != ClaimAlreadyOwned || store.must("res").LeaseID != "holder" {
			t.Fatalf("%+v lease %s err %v", got, store.must("res").LeaseID, err)
		}
	})

	for _, status := range []string{SendSent, SendUnknown, SendPermanentFailed} {
		t.Run(status, func(t *testing.T) {
			store := newMemStore(&DeliveryResolution{ResolutionID: "res", SendStatus: status, ProviderID: "smtp-1"})
			got, err := store.ClaimDueResolution(context.Background(), "lease-a", now, now.Add(BindingSMTPLease))
			if err != nil || got.Outcome != ClaimAlreadyTerminal {
				t.Fatalf("%s %+v err %v", status, got, err)
			}
			if err := store.MarkSent(context.Background(), "res", "lease-a", "smtp-other", now); err == nil {
				t.Fatal("terminal row accepted a new provider id")
			}
			if store.must("res").ProviderID != "smtp-1" || store.must("res").SendStatus != status {
				t.Fatalf("%+v", store.must("res"))
			}
		})
	}
}

func TestMarkRequiresCurrentSendingLease(t *testing.T) {
	now := time.Now().UTC()
	until := now.Add(BindingSMTPLease)
	store := newMemStore(&DeliveryResolution{ResolutionID: "res", SendStatus: SendPending})
	claimed, err := store.ClaimDueResolution(context.Background(), "lease-a", now, until)
	if err != nil || claimed.Outcome != ClaimClaimed {
		t.Fatal(err)
	}
	if err := store.MarkSent(context.Background(), "res", "lease-a", "smtp-1", now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSent(context.Background(), "res", "lease-a", "smtp-2", now); err == nil {
		t.Fatal("second SENT must be rejected")
	}
	if store.must("res").ProviderID != "smtp-1" {
		t.Fatalf("provider changed to %s", store.must("res").ProviderID)
	}

	store = newMemStore(&DeliveryResolution{
		ResolutionID: "res", SendStatus: SendSending, LeaseID: "lease-a", AttemptCount: 1,
	})
	next := now.Add(3 * time.Minute)
	if err := store.MarkRetryableFailed(context.Background(), "res", "lease-a", ErrCodeSMTPTransient, next, now); err != nil {
		t.Fatal(err)
	}
	row := store.must("res")
	if row.SendStatus != SendRetryableFailed || row.AttemptCount != 2 || row.ProviderID != "" {
		t.Fatalf("%+v", row)
	}
	if err := store.MarkRetryableFailed(context.Background(), "res", "lease-a", ErrCodeSMTPTransient, next, now); err == nil {
		t.Fatal("retry mark must not run from RETRYABLE_FAILED")
	}

	store = newMemStore(&DeliveryResolution{ResolutionID: "res", SendStatus: SendSending, LeaseID: "lease-a"})
	if err := store.MarkSendUnknown(context.Background(), "res", "lease-a", ErrCodeSMTPUncertain, now); err != nil {
		t.Fatal(err)
	}
	if store.must("res").SendStatus != SendUnknown {
		t.Fatal(store.must("res").SendStatus)
	}
	if _, owned := ClaimToSending(SendUnknown); owned {
		t.Fatal("SEND_UNKNOWN was claimed")
	}

	store = newMemStore(&DeliveryResolution{ResolutionID: "res", SendStatus: SendSending, LeaseID: "lease-a"})
	if err := store.MarkPermanentFailed(context.Background(), "res", "lease-a", ErrCodeSMTPPermanent, now); err != nil {
		t.Fatal(err)
	}
	if store.must("res").SendStatus != SendPermanentFailed || store.must("res").ProviderID != "" {
		t.Fatalf("%+v", store.must("res"))
	}
	if err := store.MarkPermanentFailed(context.Background(), "res", "wrong-lease", ErrCodeSMTPPermanent, now); err == nil {
		t.Fatal("wrong lease must not mark permanent")
	}
}

func TestEmailBindingRequiresParentFlag(t *testing.T) {
	t.Setenv(EnvBindingEnabled, "")
	t.Setenv(EnvEmailBindingEnabled, "true")
	if EmailBindingEnabled() {
		t.Fatal("email flag alone must not enable the path")
	}
}

type panicStore struct{}

func (panicStore) ReapStaleSending(context.Context, time.Time) (int64, error) {
	panic("store used while flag off")
}
func (panicStore) ClaimDueResolution(context.Context, string, time.Time, time.Time) (ClaimResult, error) {
	panic("store used while flag off")
}
func (panicStore) MarkSent(context.Context, string, string, string, time.Time) error {
	panic("store used")
}
func (panicStore) MarkRetryableFailed(context.Context, string, string, string, time.Time, time.Time) error {
	panic("store used")
}
func (panicStore) MarkSendUnknown(context.Context, string, string, string, time.Time) error {
	panic("store used")
}
func (panicStore) MarkPermanentFailed(context.Context, string, string, string, time.Time) error {
	panic("store used")
}
func (panicStore) GetResolutionForDelivery(context.Context, string) (DeliveryResolution, error) {
	panic("store used")
}

type countingSender struct {
	calls     int
	id        string
	uncertain bool
	permanent bool
	err       error
}

func (s *countingSender) Send(context.Context, []string) (string, bool, bool, error) {
	s.calls++
	return s.id, s.uncertain, s.permanent, s.err
}

type timeoutErr string

func (e timeoutErr) Error() string { return string(e) }

func errTimeout(msg string) error { return timeoutErr(msg) }

type memStore struct {
	mu   sync.Mutex
	rows map[string]*DeliveryResolution
}

func newMemStore(rows ...*DeliveryResolution) *memStore {
	m := &memStore{rows: map[string]*DeliveryResolution{}}
	for _, row := range rows {
		cp := *row
		m.rows[row.ResolutionID] = &cp
	}
	return m
}

func (m *memStore) must(id string) DeliveryResolution {
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.rows[id]
}

func (m *memStore) ReapStaleSending(_ context.Context, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, row := range m.rows {
		if row.SendStatus == SendSending && row.LeaseUntil != nil && row.LeaseUntil.Before(now) {
			row.SendStatus = SendUnknown
			row.LeaseID = ""
			row.LeaseUntil = nil
			row.LastErrorCode = ErrCodeLeaseExpired
			n++
		}
	}
	return n, nil
}

func (m *memStore) ClaimDueResolution(_ context.Context, leaseID string, now, leaseUntil time.Time) (ClaimResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.rows {
		if row.SendStatus != SendPending && row.SendStatus != SendRetryableFailed {
			continue
		}
		if row.LeaseUntil != nil && !row.LeaseUntil.Before(now) {
			return ClaimResult{Outcome: ClaimAlreadyOwned, Resolution: *row}, nil
		}
		if row.NextRetryAt != nil && row.NextRetryAt.After(now) {
			continue
		}
		row.SendStatus = SendSending
		row.LeaseID = leaseID
		row.LeaseUntil = &leaseUntil
		cp := *row
		return ClaimResult{Outcome: ClaimClaimed, Resolution: cp}, nil
	}
	for _, row := range m.rows {
		switch row.SendStatus {
		case SendSent, SendUnknown, SendPermanentFailed:
			return ClaimResult{Outcome: ClaimAlreadyTerminal, Resolution: *row}, nil
		case SendSending:
			return ClaimResult{Outcome: ClaimAlreadyOwned, Resolution: *row}, nil
		}
	}
	return ClaimResult{Outcome: ClaimNotFound}, nil
}

func (m *memStore) MarkSent(_ context.Context, resolutionID, leaseID, providerID string, now time.Time) error {
	return m.transition(resolutionID, leaseID, func(row *DeliveryResolution) {
		row.SendStatus = SendSent
		row.ProviderID = providerID
		row.LeaseID = ""
		row.UpdatedAt = now
	})
}

func (m *memStore) MarkRetryableFailed(_ context.Context, resolutionID, leaseID, errorCode string, nextRetry, now time.Time) error {
	return m.transition(resolutionID, leaseID, func(row *DeliveryResolution) {
		row.SendStatus = SendRetryableFailed
		row.ProviderID = ""
		row.AttemptCount++
		row.NextRetryAt = &nextRetry
		row.LastErrorCode = errorCode
		row.LeaseID = ""
		row.UpdatedAt = now
	})
}

func (m *memStore) MarkSendUnknown(_ context.Context, resolutionID, leaseID, errorCode string, now time.Time) error {
	return m.transition(resolutionID, leaseID, func(row *DeliveryResolution) {
		row.SendStatus = SendUnknown
		row.LastErrorCode = errorCode
		row.LeaseID = ""
		row.UpdatedAt = now
	})
}

func (m *memStore) MarkPermanentFailed(_ context.Context, resolutionID, leaseID, errorCode string, now time.Time) error {
	return m.transition(resolutionID, leaseID, func(row *DeliveryResolution) {
		row.SendStatus = SendPermanentFailed
		row.ProviderID = ""
		row.AttemptCount++
		row.LastErrorCode = errorCode
		row.LeaseID = ""
		row.UpdatedAt = now
	})
}

func (m *memStore) GetResolutionForDelivery(_ context.Context, resolutionID string) (DeliveryResolution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[resolutionID]
	if !ok {
		return DeliveryResolution{}, ErrNotFound
	}
	return *row, nil
}

func (m *memStore) transition(id, leaseID string, fn func(*DeliveryResolution)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[id]
	if !ok || row.SendStatus != SendSending || row.LeaseID != leaseID {
		return ErrNotFound
	}
	fn(row)
	return nil
}
