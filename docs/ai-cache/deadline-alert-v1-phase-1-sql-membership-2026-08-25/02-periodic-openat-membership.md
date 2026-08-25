# 02 — Periodic OpenAt membership

```text
NEW_V2_SEMANTIC=OpenAt
LEGACY_NULL_OPEN_AT_COMPATIBILITY=cycle_start
LEGACY_FALLBACK_ONLY=true
PERIODIC_ALERT_FROM=OpenAt
```

## Periodic (cycle row exists)

```text
AlertFrom = COALESCE(pc.open_at, pc.cycle_start)
eligible iff AlertFrom IS NOT NULL AND AlertFrom <= TodayHCM
```

| Case | Result |
|------|--------|
| open_at past | included |
| open_at = todayHCM | included (inclusive) |
| open_at future | excluded |
| open_at NULL, cycle_start past | included (legacy) |
| open_at NULL, cycle_start future | excluded |
| open_at NULL, cycle_start NULL | excluded (malformed fail-safe) |

## Irregular (no cycle row)

```text
no OpenAt gate
Draft + submitted_at IS NULL → included
```

## Explicit non-policies

```text
Worker +7d materialization lookahead = NOT alert policy
created_at / materialized_at = NOT alert boundary
ApplicableFrom = NOT queried in deadlinealerts repository
```
