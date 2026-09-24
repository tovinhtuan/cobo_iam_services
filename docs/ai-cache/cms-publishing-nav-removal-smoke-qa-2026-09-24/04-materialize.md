# Flow D — Materialize

- time_utc: 2026-09-24T08:38:26.110134+00:00
- expected: eligible preview; create NotStarted; rematerialize exists; no dup
- actual: preview=200 eligible=4 skipped=16
  mat=200 created=4 failed=0
  remat exists=4 children=4 statuses=['NotStarted']
- result: PASS
