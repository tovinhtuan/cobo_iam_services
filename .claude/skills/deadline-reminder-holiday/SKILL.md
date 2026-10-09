---
name: deadline-reminder-holiday
description: Dùng khi sửa logic hạn CBTT, cảnh báo hạn và nhắc việc trong cobo_iam_services - deadline engine, periodic cycle, deadlinealerts (UPCOMING/DUE_SOON/OVERDUE), reminder milestone/dispatch, lịch nghỉ holiday (import XLSX, non_trading_days), ngày làm việc vs ngày lịch, timezone Asia/Ho_Chi_Minh.
---

# Deadline, Reminder & Holiday

Pair with `cobo-domain-cbtt` for the legal rule behind each deadline.

## Components (verify; as of 2026-10)
- Deadline engine: `internal/disclosure/app/deadlineengine` (`add_days.go`
  skips Saturday, Sunday and holidays). Flags `DEADLINE_ENGINE_V2*`,
  `PERIODIC_SEEDING_ENABLED`; `cmd/periodic-materialize-one` (guarded tool).
- Periodic cycles: seeded and materialized by the worker each tick.
- `internal/deadlinealerts`: status UPCOMING / DUE_SOON / OVERDUE /
  PENDING_CONFIRM / DONE computed in `Asia/Ho_Chi_Minh`.
- `internal/reminder`: milestones → occurrences; PENDING → DISPATCHING →
  SENT / RETRY_SCHEDULED / FAILED; deterministic idempotency key; dispatch via
  worker and `/internal/reminders/dispatch` (`X-Internal-Token`).
- `internal/holiday`: CMS uploads XLSX (excelize; col A date, B name, C type);
  DB calendar overrides `configs/non_trading_days/<year>.json`. Year map
  cached in-process separately in API and worker.
- Workflow `dueRule`: `T+N` calendar days, `H+N` hours, or month literal.

## Rules
1. All date math in `Asia/Ho_Chi_Minh`; store instants in UTC; compare dates
   as local calendar dates, never by truncating UTC timestamps.
2. Be explicit per rule: calendar days with roll-forward to next working day,
   or working days. TT96 periodic deadlines are calendar days (see
   `cobo-domain-cbtt`); do not silently use `add_days` working-day logic for
   them.
3. Inclusive/exclusive boundaries documented and tested (event day = day 0).
4. Holiday data is per year; missing year → explicit warning/metric, not
   silent "no holidays". Don't cache load errors long-term; invalidate caches
   in both API and worker after upload (or document restart/TTL).
5. Reminders are idempotent per (occurrence, milestone, recipient, channel).
6. Recomputing deadlines (holiday change, template change) must not
   duplicate reminders or resurrect DONE items.

## Test matrix
- Deadline landing on Saturday, Sunday, Tết, a substituted working day
  (ngày làm bù) if the calendar supports it.
- Year boundary (Q4 FS due in January; 90/110-day rules across Tết).
- Leap year Feb 29; month-end (e.g. quarter end 31/03 + 20 days).
- Event at 23:30 local vs 00:30 local (24h rules).
- Holiday uploaded after alerts computed → recompute correctness.
- Duplicate dispatch / worker restart during DISPATCHING.

## Output format
- Rule(s) changed and legal basis
- Day-counting mode per rule
- Recompute/migration impact
- Test cases added
