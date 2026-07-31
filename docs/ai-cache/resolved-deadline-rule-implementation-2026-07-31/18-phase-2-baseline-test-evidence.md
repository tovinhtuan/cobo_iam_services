# Phase 2 baseline full-suite evidence

## Purpose
Chứng minh `go test ./...` FAIL tại Phase 2 **không** do thay đổi deadline DTO.

## Baseline (pre–Phase 2 feat)
| Field | Value |
|-------|-------|
| Commit | `661413d` (detached worktree `/tmp/cobo-be-phase2-baseline`) |
| Command | `go test ./... -count=1` |
| Exit | `1` |
| Matching failure | `internal/notification/app` → `TestContract_VariableParity/workflow.approved` |
| Message | `variable "workflow_instance_id" is declared in meta.yaml but NEVER used in any template file` |

Note: baseline worktree cũng FAIL thêm một số integration packages (adhoc/companyaccess/httpserver) — phụ thuộc môi trường/DB; **cùng** failure notification parity là bằng chứng pre-existing liên quan báo cáo Phase 2.

## Current HEAD (post–Phase 2)
| Field | Value |
|-------|-------|
| Commit | `579e4a0` |
| Command | `go test ./internal/notification/app/ -count=1 -run TestContract_VariableParity` |
| Exit | `1` |
| Failure | **Identical** `workflow_instance_id` / `workflow.approved` |

## Comparison
| Check | Result |
|-------|--------|
| Same failing test name | YES |
| Same error text | YES |
| Phase 2 feat `1522839` touches `internal/notification/**` | **NO** (`git diff --name-only 661413d..1522839` — no notification paths) |
| Phase 2 scope | disclosure/app applicability + contracts + service + docs only |

## Accurate Phase 2 quality statement
```text
Targeted disclosure/deadline tests: PASS
go vet ./...: PASS
go build API: PASS
Docker API build: PASS
git diff --check: PASS

Full go test ./...: FAIL
Reason: pre-existing notification/app TestContract_VariableParity
(proven at baseline 661413d; unchanged at 579e4a0)
```

**Không** ghi “all backend tests PASS”.
