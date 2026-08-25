# 03 — periodic_cycles (DEV read-only)

```text
type_id=bang-tinh-luong-nhan-vien-thang-ban-sao
SELECT COUNT(*) FROM periodic_cycles WHERE type_id=... → 0

CURRENT_SLOT_CYCLE_EXISTS=false
PERIODIC_CYCLE_ID=null
CYCLE_LABEL=null
CYCLE_START=null
OPEN_AT=null
DUE_AT=null
RECORD_ID=null
```

Consequence: no occurrence for `2026-08-25`; pipeline stops before materialization / Deadline Alert membership.