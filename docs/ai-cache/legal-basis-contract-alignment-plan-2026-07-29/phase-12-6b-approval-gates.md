# Phase 12.6B — Approval gates

Planning a backfill is **not** permission to mutate.

| Gate | Description | Required before | Status (Plan end) |
| --- | --- | --- | --- |
| **Approval 1** | Dataset boundary + exact 6-record allowlist | Snapshot / apply design acceptance | **GRANTED** (user LOCKED_ALL_6 + inventory match) |
| **Approval 2** | Snapshot + rollback design adequate | Apply implementation | **GRANTED** (design locked in docs; secure path ops still operator-owned at run time) |
| **Approval 3** | Explicit DEV mutation permission | `--apply` | **PENDING** |
| **Approval 4** | Execution window (timebox / change freeze) | `--apply` | **PENDING** |
| **Approval 5** | Post-write acceptance | Close apply phase | **PENDING** |

## Mutate unlock phrase (required)

User must say:

```text
Cho phép thực thi Controlled DEV Backfill theo exact allowlist đã duyệt.
```

Any weaker wording ("continue", "LGTM plan", "implement tool") does **not** unlock DB writes.
