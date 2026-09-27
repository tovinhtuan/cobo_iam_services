package workflowdept

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

const (
	ErrCodeSMTPHostEmpty        = "smtp_host_empty"
	ErrCodeRecipientKeyMissing  = "recipient_key_missing"
	ErrCodeRecipientDecryptFail = "recipient_decrypt_failed"
	ErrCodeSMTPUncertain        = "smtp_uncertain"
	ErrCodeSMTPTransient        = "smtp_transient"
	ErrCodeSMTPPermanent        = "smtp_permanent"
	ErrCodeSMTPUnwired          = "smtp_transport_unwired"
	ErrCodeLeaseExpired         = "lease_expired_uncertain"
)

// ClaimOutcome is the result of an atomic claim. Lost races are not system errors.
type ClaimOutcome string

const (
	ClaimClaimed         ClaimOutcome = "claimed"
	ClaimAlreadyOwned    ClaimOutcome = "already_owned"
	ClaimAlreadyTerminal ClaimOutcome = "already_terminal"
	ClaimNotFound        ClaimOutcome = "not_found"
)

// DeliveryResolution is the row a worker may send. Ciphertext stays encrypted until claim succeeds.
type DeliveryResolution struct {
	ResolutionID  string
	OccurrenceID  string
	CompanyID     string
	Ciphertext    []byte
	KeyVersion    uint32
	SendStatus    string
	ProviderID    string
	AttemptCount  int
	NextRetryAt   *time.Time
	LeaseID       string
	LeaseUntil    *time.Time
	LastErrorCode string
	UpdatedAt     time.Time
}

// ClaimResult is returned by the repository. Only Claimed may call SMTP.
type ClaimResult struct {
	Outcome    ClaimOutcome
	Resolution DeliveryResolution
}

// DeliveryStore is the binding-path repository. Implementations must use the atomic claim predicate.
type DeliveryStore interface {
	ReapStaleSending(ctx context.Context, now time.Time) (int64, error)
	ClaimDueResolution(ctx context.Context, leaseID string, now, leaseUntil time.Time) (ClaimResult, error)
	MarkSent(ctx context.Context, resolutionID, leaseID, providerID string, now time.Time) error
	MarkRetryableFailed(ctx context.Context, resolutionID, leaseID, errorCode string, nextRetry time.Time, now time.Time) error
	MarkSendUnknown(ctx context.Context, resolutionID, leaseID, errorCode string, now time.Time) error
	MarkPermanentFailed(ctx context.Context, resolutionID, leaseID, errorCode string, now time.Time) error
	GetResolutionForDelivery(ctx context.Context, resolutionID string) (DeliveryResolution, error)
}

// BindingSender is the binding transport. It must not be the legacy reminder sender.
type BindingSender interface {
	Send(ctx context.Context, recipients []string) (providerID string, uncertain, permanent bool, err error)
}

// BindingDeps wires one tick. SMTPHost empty is fail-closed on this path only.
type BindingDeps struct {
	Store    DeliveryStore
	Sender   BindingSender
	Key      *RecipientKey
	SMTPHost string
	Now      func() time.Time
	NewLease func() (string, error)
	Log      *slog.Logger
}

// RunBindingTick returns immediately unless both binding flags are on.
// It never calls the legacy reminder sender.
func RunBindingTick(ctx context.Context, deps BindingDeps) error {
	if !EmailBindingEnabled() {
		return nil
	}
	if deps.Store == nil {
		return nil
	}
	if deps.Key == nil {
		if key, err := LoadRecipientKey(); err == nil {
			deps.Key = &key
		}
	}
	now := time.Now().UTC()
	if deps.Now != nil {
		now = deps.Now().UTC()
	}
	if _, err := deps.Store.ReapStaleSending(ctx, now); err != nil {
		return err
	}
	leaseID := ""
	if deps.NewLease != nil {
		id, err := deps.NewLease()
		if err != nil {
			return err
		}
		leaseID = id
	}
	if leaseID == "" {
		leaseID = now.Format("20060102150405.000000000")
	}
	claimed, err := deps.Store.ClaimDueResolution(ctx, leaseID, now, now.Add(BindingSMTPLease))
	if err != nil {
		return err
	}
	if claimed.Outcome != ClaimClaimed {
		return nil
	}
	return finishClaimed(ctx, deps, claimed.Resolution, now)
}

func finishClaimed(ctx context.Context, deps BindingDeps, row DeliveryResolution, now time.Time) error {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	failPermanent := func(code string) error {
		log.Warn("workflow department binding delivery stopped",
			slog.String("resolution_id", row.ResolutionID),
			slog.String("error_code", code),
		)
		return deps.Store.MarkPermanentFailed(ctx, row.ResolutionID, row.LeaseID, code, now)
	}
	if deps.Key == nil || len(deps.Key.Key) != 32 {
		return failPermanent(ErrCodeRecipientKeyMissing)
	}
	plain, err := DecryptRecipientEmails(deps.Key.Key, deps.Key.Version, row.CompanyID, row.OccurrenceID, row.ResolutionID, row.Ciphertext)
	if err != nil {
		return failPermanent(ErrCodeRecipientDecryptFail)
	}
	recipients, ok := validRecipients(splitRecipients(string(plain)))
	if !ok {
		return failPermanent(ErrCodeInvalidRecipient)
	}
	if strings.TrimSpace(deps.SMTPHost) == "" {
		return failPermanent(ErrCodeSMTPHostEmpty)
	}
	if deps.Sender == nil {
		return failPermanent(ErrCodeSMTPUnwired)
	}
	providerID, uncertain, permanent, sendErr := deps.Sender.Send(ctx, recipients)
	accepted := sendErr == nil && !uncertain && strings.TrimSpace(providerID) != ""
	if accepted {
		return deps.Store.MarkSent(ctx, row.ResolutionID, row.LeaseID, strings.TrimSpace(providerID), now)
	}
	if uncertain || sendErr == nil || isUncertain(sendErr) {
		log.Warn("workflow department binding delivery uncertain",
			slog.String("resolution_id", row.ResolutionID),
			slog.String("error_code", ErrCodeSMTPUncertain),
		)
		return deps.Store.MarkSendUnknown(ctx, row.ResolutionID, row.LeaseID, ErrCodeSMTPUncertain, now)
	}
	attempt := row.AttemptCount + 1
	_, isPermanentAttempt := RetryAfterFailure(attempt)
	if permanent || isPermanentAttempt {
		code := ErrCodeSMTPPermanent
		if !permanent {
			code = ErrCodeSMTPTransient
		}
		return failPermanent(code)
	}
	delay, _ := RetryAfterFailure(attempt)
	return deps.Store.MarkRetryableFailed(ctx, row.ResolutionID, row.LeaseID, ErrCodeSMTPTransient, now.Add(delay), now)
}

func splitRecipients(plain string) []string {
	parts := strings.FieldsFunc(plain, func(r rune) bool { return r == ',' || r == '\n' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isUncertain(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") || strings.Contains(msg, "uncertain")
}
