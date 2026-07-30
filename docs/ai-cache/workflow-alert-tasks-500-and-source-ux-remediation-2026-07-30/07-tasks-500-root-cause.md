# Root cause
**T500-RC4** nullable actor/department not handled.
Primary: department subquery NULL Scan into Go string.
Contributing: system assignee has no users row (handled fail-soft after fix with actor_type=SYSTEM).
Not FE wrong ID.
