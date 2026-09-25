package workflowdept

// Send states for one pinned dispatch. SEND_UNKNOWN is never auto-resent.
const (
	SendPending         = "PENDING"
	SendSending         = "SENDING"
	SendSent            = "SENT"
	SendUnknown         = "SEND_UNKNOWN"
	SendRetryableFailed = "RETRYABLE_FAILED"
	SendPermanentFailed = "PERMANENT_FAILED"
)

// ClaimToSending reports whether this worker may call SMTP.
// Only PENDING → SENDING is allowed. A lost race stays on the committed state.
func ClaimToSending(current string) (next string, owned bool) {
	if current == SendPending {
		return SendSending, true
	}
	return current, false
}

// AfterProvider classifies the outcome after a send attempt.
// uncertain means the provider may already have accepted the message.
func AfterProvider(hadMessageID bool, accepted bool, uncertain bool) string {
	if hadMessageID || accepted {
		return SendSent
	}
	if uncertain {
		return SendUnknown
	}
	return SendRetryableFailed
}

// WatchdogStaleSending moves a stuck SENDING row to SEND_UNKNOWN.
// It does not return PENDING, so the legacy reaper must not resend it.
func WatchdogStaleSending(current string) string {
	if current == SendSending {
		return SendUnknown
	}
	return current
}

// MayCallSMTP is false once a provider id exists or the state is unknown/terminal.
func MayCallSMTP(status string, providerMessageID string) bool {
	if providerMessageID != "" {
		return false
	}
	switch status {
	case SendPending, SendRetryableFailed:
		return true
	default:
		return false
	}
}
