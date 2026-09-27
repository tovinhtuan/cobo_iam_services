package workflowdept

import "time"

// BindingSMTPLease sits above the reminder SMTP ceiling (10s dial + 30s op) plus buffer.
const BindingSMTPLease = 90 * time.Second

// RetryAfterFailure is the binding-path schedule. attemptJustFinished is 1-based
// and is recorded only after a failed attempt. Claim must not call this.
func RetryAfterFailure(attemptJustFinished int) (delay time.Duration, permanent bool) {
	switch attemptJustFinished {
	case 1:
		return time.Minute, false
	case 2:
		return 3 * time.Minute, false
	case 3:
		return 6 * time.Minute, false
	case 4:
		return 10 * time.Minute, false
	default:
		return 0, true
	}
}

// AfterProviderClassified maps a finished attempt. Uncertain never becomes a retry.
func AfterProviderClassified(hadMessageID, accepted, uncertain, permanent bool) string {
	if hadMessageID || accepted {
		return SendSent
	}
	if uncertain {
		return SendUnknown
	}
	if permanent {
		return SendPermanentFailed
	}
	return SendRetryableFailed
}
