# 01 — Active + ApplicableFrom freeze

## CHECK_01 — Active

```text
CHECK_01_TEMPLATE_ACTIVE=PASS
ACTIVE_VERSION_NO=1
active_version_no > 0 ✓
```

## CHECK_02 — Frequency

```text
CHECK_02_FREQUENCY_DAILY=PASS
PERSISTED_FREQUENCY=daily
```

From `deadline_config_json.frequency_unit` on active version (not UI label alone).

## CHECK_03 — ApplicableFrom freeze

```text
ACTIVE_APPLICABLE_FROM_MODE=CURRENT_SLOT
ACTIVE_APPLICABLE_FROM_SLOT=2026-08-25
CHECK_03_APPLICABLE_FROM_FROZEN=PASS
```

Authoring CURRENT_SLOT was resolved at Activate into concrete daily slot `2026-08-25` (same HCM calendar day as activation).

## Deadlines on version

```text
deadline_days=5
open_days_before_t=NULL
```

## Activation vs investigation day

```text
ACTIVATION_HCM_DATE=2026-08-25
CURRENT_HCM_DATE=2026-08-25
EXPECTED_CURRENT_DAILY_SLOT=2026-08-25
ACTUAL_ACTIVE_APPLICABLE_FROM_SLOT=2026-08-25
```

Same-day CURRENT activation: boundary slot equals current logical slot; eligibility would be `candidate >= boundary` → true **if** seeder ran.