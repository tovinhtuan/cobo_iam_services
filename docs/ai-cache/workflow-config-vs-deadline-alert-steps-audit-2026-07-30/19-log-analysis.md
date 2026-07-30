# 19 — Log analysis

`docker logs cobo-iam-api/worker --since 2026-07-30T04:00:00Z --until 04:20:00Z` filtered for qa-monthly/snapshot/materialize: **no matching lines** (one-shot ran as host CLI binary, not API/worker process).

Instance row confirms `workflow_source=global_template` at create time.
No evidence of legacy linear hardcoded steps in this materialization.
