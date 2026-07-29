# Phase 12.6B — Controlled DEV backfill runbook

**Status:** PLAN / not executed
**Credentials:** never paste DSN/password/token here.

| Step | Owner | Command skeleton | Expected | Failure | Evidence |
| --- | --- | --- | --- | --- | --- |
| **A. Preconditions** | Eng | Confirm branch, `0c6dcca` ancestor, allowlist present, no Docker verify | Docs + tool RO ready | STOP | git rev-parse |
| **B. Approvals** | PO/BA/Eng | Check `phase-12-6b-approval-gates.md` A1–A4 | All required true | STOP | approvals checklist |
| **C. Environment** | Eng | Verify DEV host/db name `cobo_iam` / MySQL 8 | Match | STOP wrong env | masked env note |
| **D. Fresh inventory** | Eng | `go run ./cmd/legal-basis-inventory --docker-dev --out-dir …` | A=6, total=6, match hashes | `STALE_DRY_RUN` | RO summary JSON |
| **E. Allowlist verify** | Eng | Diff inventory vs `phase-12-6b-record-allowlist.json` | Exact 6 | STOP | allowlist lock |
| **F. Snapshot** | Eng | Secure snapshot per `phase-12-6b-snapshot-runbook.md` | count=6, checksum | ABORT | masked path + checksum |
| **G. Dry confirmation** | Eng+PO | Future CLI without `--apply` | guards PASS | fix | dry log |
| **H. Explicit apply** | Eng | Future CLI `--apply --confirm-token …` only after exact user phrase | Txn start | refuse | apply audit |
| **I. In-txn verification** | Tool | Read-back 6 rows | schema PASS | rollback | tool log |
| **J. Commit** | Tool | COMMIT | ok | rollback | — |
| **K. Post inventory** | Eng | RO inventory again | C=6, A=0 | rollback window | post summary |
| **L. Functional read-back** | QA | API/detail sample (optional) | structured 1 item | escalate | notes |
| **M. Rollback triggers** | Eng | On POST_WRITE_FAIL / abort | enter N | — | — |
| **N. Rollback** | Eng | `phase-12-6b-rollback-runbook.md` | A=6 restored | escalate | rollback verification |
| **O. Cleanup** | Eng | shred/secure-delete tokens; chmod snapshot retained per retention | done | note residual | checklist |
| **P. Evidence** | Eng | Update handoff / readiness (hashes only) | redact PASS | fix redact | ai-cache |
| **Q. Stop conditions** | All | See stop list below | — | — | — |

## Stop conditions (pre-mutation)

Wrong env/DB; snapshot invalid; allowlist ≠ 6; missing row; hash mismatch; not Group A; structured already present; Group D/malformed/overflow; unexpected schema (`is_released` appear changing dataset).

## Stop / rollback (in mutation)

RowsAffected ≠ 1; UUID duplicate; projection mismatch; read-back mismatch; timeout/SQL error → ROLLBACK entire txn.
