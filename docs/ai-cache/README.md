# Cursor Skill Pack for Cobo Repos

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