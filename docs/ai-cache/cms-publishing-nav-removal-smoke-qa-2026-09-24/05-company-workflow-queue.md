# Flow E — Company queue

- time_utc: 2026-09-24T08:38:26.110134+00:00
- expected: submit→PendingReview; appears in GET /reviews; Global Record not in queue; hiding CMS nav does not break API
- actual: submit=200 reviews=200 child_in_queue=1 global_in_queue=0
- result: PASS
