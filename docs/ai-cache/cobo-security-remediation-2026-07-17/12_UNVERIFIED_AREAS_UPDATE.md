# Unverified Areas Update

## Retest status in this cycle
- Stored XSS runtime payload test: not executed in this cycle (safe window deferred).
- Horizontal same-tenant object-ID access (User A vs User B): not executed (requires dedicated paired fixtures).
- Password reset / OTP abuse-rate path: source reviewed, runtime abuse matrix deferred.
- SSRF probes: deferred.
- Workflow bypass fuzz (safe): partial regression only.
- Replay/race double-submit: deferred.

## Action
- Keep these as `UNVERIFIED` (not PASS).
- Schedule focused mini-retest pack in next remediation batch.
