# M2 — ANY completion + concurrency
- CAS: UPDATE ... status=pending
- Concurrent M1/M2 → exactly one success + one 409
- Markers: ANY_COMPLETION_SINGLE_WINNER, NEXT_STEP_MATERIALIZED_EXACTLY_ONCE
