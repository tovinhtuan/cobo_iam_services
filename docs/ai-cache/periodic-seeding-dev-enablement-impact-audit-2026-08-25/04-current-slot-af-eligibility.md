# 04 — Current slots + ApplicableFrom

Investigation HCM: 2026-08-25

```text
DAILY_CURRENT_SLOT=2026-08-25
WEEKLY_CURRENT_SLOT=2026-08-23  # Sunday-based
MONTHLY_CURRENT_SLOT=2026-08
QUARTERLY_CURRENT_SLOT=2026-Q3
YEARLY_CURRENT_SLOT=2026
HISTORICAL_RANGE_GENERATION=false  # current slot only
```

AF evaluation (template-level, before company loop):

```text
CURRENT_SLOT_ELIGIBLE_TEMPLATE_COUNT=13
BEFORE_AF_BOUNDARY_COUNT=1
  bao-cao-tai-chinh-quy: candidate 2026-Q3 < boundary 2026-Q4 → skip
LEGACY_NULL_AF_COUNT=11
INVALID_AF_COUNT=0
```

Company filter after AF: `auto_create` (default true) + `IsApplicable` when global+strict.
