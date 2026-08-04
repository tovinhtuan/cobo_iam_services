# Cursor Skill Pack for Cobo Repos

## Tín hiệu tuân thủ — phải thấy được trong Chat (bắt buộc)

Giống `cobo_web_design/docs/ai-cache/README.md`: mọi câu trả lời có nội dung phải có **dòng đầu** bắt đầu **`[ai-cache]`** + README + file `docs/ai-cache/` đã dùng + skill + **`Mandatory README: đã áp dụng`**. Chi tiết: xem README trong `cobo_web_design` hoặc sao chép mục đó vào repo này nếu làm việc chỉ IAM.

Snippet **“Bắt buộc: tuân thủ docs/ai-cache/README.md…”** (dán đầu prompt) và **lệnh Docker/build hoặc `BLOCKED:`** sau implement: xem **`cobo_web_design/docs/ai-cache/README.md`** và bản siết trong **`.cursor/rules/ai-cache-read-first.mdc`** của repo này.

**Áp dụng tự động (Cursor):** Luật **`.cursor/rules/ai-cache-read-first.mdc`** (`alwaysApply: true`) + **`AGENTS.md`** ở root.

Pack này gồm 2 bộ cấu hình:
- `cobo_web_design/.cursor/...`
- `cobo_iam_services/.cursor/...`

## Cách dùng
1. Copy thư mục `.cursor` trong từng repo vào đúng project tương ứng.
2. Giữ các `rules/*.mdc` để luôn bật guardrails kiến trúc.
3. Dùng Agent trong Cursor và prompt theo skill tương ứng.
4. Với task lớn, bắt đầu bằng `system-design-feature`.
5. Trước khi hoàn tất, luôn chạy `premerge-system-review`.

## Prompt mặc định nên dán cho hầu hết mọi câu hỏi

Dùng prompt này như prompt khởi đầu gần như mỗi lần hỏi Cursor.

```text
Use the relevant project skill for this task.
First identify the architectural boundary, affected layers, domain invariants, failure modes, validation strategy, and test scope before writing code.
Preserve backward compatibility unless explicitly asked otherwise.
Prefer minimal, reviewable diffs.
Do not skip loading/error/empty states on frontend.
Do not skip validation, authorization, idempotency, migration safety, or observability on backend.
Before marking the task done, run a pre-merge review and report risks, gaps, and verification steps.
```




## Company Premium implementation — Phase 0 contract lock (2026-08-04)

- Verdict **PHASE_0_CONTRACT_LOCKED**: `docs/ai-cache/company-premium-implementation-2026-08-04/`
- Case C approved via user instruction 2026-08-04; `company_subscriptions` SoT; `plan: null` no-plan; badge = PREMIUM+ACTIVE+COMPANY_SUBSCRIPTION
- Awaiting user confirmation before Phase 1 (domain + migration)

## Company Premium Backend implementation plan (2026-08-03)

- Verdict **COMPANY_PREMIUM_IMPLEMENTATION_PLAN_READY**: `docs/ai-cache/company-premium-backend-implementation-plan-2026-08-03/`
- Deep audit @ `9284a31`; Case B paid-plan source; A+B API `RECOMMENDED_PENDING_APPROVAL`; exact file map + phases; no source/migration/deploy

## Operational Dashboard real KPI aggregation (2026-08-03)

- Verdict **DASHBOARD_REAL_KPI_IMPLEMENTATION_READY** — pointer `docs/ai-cache/dashboard-kpi-real-aggregation-2026-08-03/`; canonical pack in FE sibling
- DEV deploy smoke **PASS_WITH_DATA_LIMITATIONS** — see `20-dev-deploy-pointer.md` + FE `results.json`
- Remap overview KPIs + completed_at read-only; no migration/deploy

## Workflow alert tasks 500 + source UX (DEV)

- Verdict **PASS_TASKS_API_AND_WORKFLOW_SOURCE_UX** (2026-07-30): `docs/ai-cache/workflow-alert-tasks-500-and-source-ux-remediation-2026-07-30/`
- T500-RC4 NULL department Scan; CMS dual-source Global Template v1 · 4 steps; no rematerialize

## Resolved deadline rule by company (Phase 5 DEV deploy)

- Verdict **PHASE_5_DEV_DEPLOYMENT_READY** (2026-07-31): pointer `00-phase-5-pointer.md`; full evidence in FE pack `cobo_web_design/.../resolved-deadline-rule-implementation-2026-07-31/`
- API-only additive deploy from `e2e3f1c`; worker/MySQL unchanged; no migration; await Phase 6

## Resolved deadline rule by company (Phase 2 backend)

- Verdict **PHASE_2_BACKEND_READY** (2026-07-31): `docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/`
- Additive `resolved_deadline_rule` on type detail; reuses ResolveStructure / ResolveDeadlineDays; no deploy

## Workflow config vs deadline alert steps (DEV audit)

- Verdict **EXPECTED_RUNTIME_REPRESENTATION** (2026-07-30): `docs/ai-cache/workflow-config-vs-deadline-alert-steps-audit-2026-07-30/`
- RC-4 dual-SoT; snapshot/alert steps match effective; CMS global config empty
- Paired FE screenshots under same folder name in `cobo_web_design`

## Guarded periodic one-shot materialization (DEV)

- Verdict **PASS_ONE_SHOT_MATERIALIZATION_AND_ALERT_E2E** (2026-07-30): `docs/ai-cache/guarded-periodic-one-shot-materialization-2026-07-30/`
- CLI one-shot exact QA scope; production calculator/materializer; seeding remains OFF
- Paired FE UI evidence under same folder name in `cobo_web_design`

## Prompt tái sử dụng theo từng tình huống

### 1) Khi xây feature mới từ đầu

```text
Use system-design-feature first, then switch to the relevant repo-specific implementation skill.
Before coding, define the objective, user flow, domain invariants, API contract, UI states, data flow, failure modes, rollout approach, and test plan.
Only then implement with minimal and reviewable diffs.
```

### 2) Khi làm feature frontend trong `cobo_web_design`

```text
Use the frontend skill that best matches this task.
Follow the vertical slice structure: route -> screen -> feature components -> hooks/services -> types.
Do not mix route concerns, fetching concerns, and presentation concerns in one large file.
Handle loading, error, empty, success, disabled, and invalid-param states explicitly.
Add focused Vitest/testing-library coverage for the core user-visible behavior.
```

### 3) Khi làm feature backend trong `cobo_iam_services`

```text
Use the backend skill that best matches this task.
Keep boundaries clear: handler -> service/usecase -> repository -> external systems.
Define request/response contract, validation rules, authorization rules, transaction boundaries, cache impact, retry/idempotency considerations, and test matrix before coding.
Do not hide security, migration, or data consistency risks.
```

### 4) Khi sửa bug

```text
Use the relevant debugging or repo-specific skill.
First restate the symptom, expected behavior, actual behavior, likely root causes, and the most probable failure path from code.
Fix the root cause with the smallest safe change, then add regression protection and list any remaining uncertainty.
```

### 5) Khi review trước merge

```text
Run premerge-system-review.
Audit requirement coverage, architectural fit, frontend state completeness, validation completeness, API/contract consistency, auth/security risks, data consistency risks, migration/deployment risks, observability gaps, and missing regression tests.
Group findings into critical, important, and nice-to-have.
```

## Prompt siêu ngắn để ghim cố định

Nếu bạn muốn một prompt ngắn hơn để dùng liên tục:

```text
Use the relevant project skill. Think in layers, define contracts first, handle failure modes explicitly, keep changes minimal, and do a system-level review before done.
```

## Mẹo dùng thực tế
- Với task mơ hồ: luôn bắt đầu bằng prompt feature mới.
- Với task chỉ chạm UI: dùng prompt frontend.
- Với task auth/API/data: dùng prompt backend.
- Với bug khó: dùng prompt sửa bug.
- Với PR sắp xong: dùng prompt review trước merge.

## Prompt khuyên dùng cũ

```text
Use the relevant project skill for this task. Start by identifying the architectural boundary, domain invariants, failure modes, and validation strategy before coding. Prefer minimal, reviewable diffs and preserve backward compatibility unless explicitly asked otherwise.
```

```text
For this feature, use system-design-feature first, then use the repo-specific implementation skill. Do not start coding until you have listed API contract, UI states, data flow, edge cases, and test plan.
```

```text
Before marking this done, run premerge-system-review and report missing validation, missing UI states, contract mismatches, auth risks, data consistency risks, and regression gaps.
```

## Mandatory Prompt Requirement (2 repos)

Áp dụng bắt buộc cho mọi prompt liên quan `cobo_web_design` và/hoặc `cobo_iam_services`:

1. Trước mọi bước, đọc `docs/ai-cache/README.md` và toàn bộ context tái sử dụng trong `docs/ai-cache/`.
2. Thứ tự ưu tiên khi có xung đột:
   - `docs/ai-cache/README.md`
   - các file còn lại trong `docs/ai-cache/`
   - project rules
   - docs/pattern cũ trong repo
3. Chọn skill phù hợp; nếu task chạm cả 2 repo thì dùng `integration-cross-repo`.
4. Với feature mới: bắt buộc contract-first trước khi code (contract + request/response/error matrix + FE mapping + BE expectations + failure modes + rollout risks + validation plan).
5. Với task implement:
   - diff nhỏ, dễ review
   - kết thúc bằng `premerge-system-review`
   - sau mỗi cycle có thay code phải rerun fresh Docker build cho services bị ảnh hưởng
   - không coi là xong cho tới khi Docker build mới nhất đã chạy và báo kết quả
6. Với task phân tích/review:
   - không sửa code nếu chưa được yêu cầu explicit
   - phân tích dựa trên code/docs thực tế, không phỏng đoán
7. Sau mỗi task (implement hoặc understand), ghi tóm tắt tái sử dụng vào `docs/ai-cache/` theo format ngắn, nhất quán:
   - task type
   - objective/question
   - implemented/discovered
   - affected repos/files/modules
   - contracts/behaviors/constraints/decisions
   - build/verification result (nếu có)
   - remaining gaps/risks/next steps
## Workflow Configuration load 404 (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-audit-2026-07-30/`
- Remediation (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-remediation-2026-07-30/` — DEV flag ON, isolation PASS
- **ROOT_CAUSE_CONFIRMED** RC-4 then remediated: `WORKFLOW_VERSIONING_ENABLED` was OFF → routes not registered
- Mirror note: `reusable-task-updates.md` (2026-07-30)

## Current-month periodic deadline alert E2E (DEV)

- Canonical FE pack: `../cobo_web_design/docs/ai-cache/current-month-periodic-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_PERIODIC_MATERIALIZATION** — QA active; seeding remains OFF
- Materialization later unblocked by guarded one-shot pack above

## Q2 Financial Report Deadline Alert E2E (DEV)

- Canonical (FE pack): `../cobo_web_design/docs/ai-cache/q2-financial-report-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_DEADLINE_CONTRACT** — no PERIOD_END in deadline engine/CMS; seeding remains OFF

## Deadline alert — active report missing? (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/deadline-alert-active-report-audit-2026-07-30/`
- Verdict: **EXPECTED_BEHAVIOR_CONFIRMED** (RC-1) — `bao-cao-tai-chinh-quy-2` on `c_001` returns `PENDING_CONFIRM`, not open OVERDUE
- Flags on DEV: `PERIODIC_SEEDING_ENABLED=false`, `DEADLINE_ENGINE_V2=false` (shadow true)

## Legal Basis Phase 12.6 (IAM mirror)

- Phase 12.6A: operational `PASS_READ_ONLY_DRY_RUN`; governance `FAIL_SCOPE_CREEP` — see plan folder `phase-12-6a-scope-exception.md`
- Phase 12.6B-Plan: `BACKFILL_PLAN_READY` — `phase-12-6b-plan-handoff.md` (docs only; no apply)
