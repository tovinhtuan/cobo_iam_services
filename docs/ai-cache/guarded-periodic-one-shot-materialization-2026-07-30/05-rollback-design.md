# Rollback design (not executed)

Preferred: application-level reverse for exact IDs only.

- cycle_id: 019fb134-b0c8-7e92-869c-bfc13f49accc
- record_id: 019fb134-b0d8-7f35-894c-120dd3dddbe5
- Refuse if status left PendingReview / user submitted / reminders exist
- No generic delete tool shipped
- Not run — success path; await user if rollback needed
