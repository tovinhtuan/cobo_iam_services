# Email Reminder Content Upgrade — PHASE 4 Handoff (Recipient Name Resolution)

**Mode:** Controlled Implementation — PHASE 4
**Date:** 2026-06-19
**Scope boundary:** điền `recipient_name` vào payload, đủ để đáp ứng feature.
**Explicitly NOT done (per chỉ đạo):** batch optimization, resolver redesign.

---

## 1. Ownership Decision (xác nhận trước khi code — bắt buộc)

**Email recipient thuộc: User.**
- Resolve company-scoped qua **Membership** (chỉ phục vụ tenant isolation). Định danh thật của người nhận là `users.user_id`.
- **Source of truth cho tên hiển thị = `users.full_name`** (`migrations/0001_init_core.up.sql:6` — `full_name VARCHAR(255) NOT NULL`).

**Bằng chứng code:**
- Mọi path resolve recipient trong `internal/reminder/infra/mysql/recipient_query.go` trả về:
  `COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)` — join `memberships m → users u`, scope `m.company_id = ?`
  (`EmailsByDepartments`, `EmailsByRoles`, `AdminEmailsByCompany`, `AssigneeEmailsByStep`).
- Interface `RecipientResolver` (`contracts.go:154-157`) trả **`[]string`** (chỉ email), **không** mang `user_id`/tên.

→ **Ownership đã xác định rõ → được phép implement** (theo điều kiện của Phase 4).

**Cách lấy `recipient_name` (quyết định CL-2, theo ràng buộc Phase 4):**
- Resolver hiện trả **email** (có thể là `users.email`, hoặc fallback `users.login_id` khi email rỗng) — **không** trả `full_name`.
- Lấy `full_name` thật ⇒ phải để resolver/query trả kèm tên ⇒ **redesign resolver** (BỊ CẤM ở phase này) hoặc thêm **lookup per-email** (N+1 mới / batch — BỊ CẤM).
- Do đó áp dụng đúng spec gốc: *"Kính gửi {tên user (nếu có) hoặc email}"* → **dùng email làm `recipient_name`**.
  - ✅ 0 query mới · ✅ không đổi interface resolver · ✅ đảm bảo biến template `required: true` luôn non-empty.

---

## 2. Implementation

`internal/reminder/app/service.go` → `prepareDispatch()`, thêm (additive, có guard):
```go
// recipient_name: greet by the first resolved recipient (email). recipients is guaranteed
// non-empty here (earlier guard skips occurrences with no recipient). No extra query.
if _, ok := payload["recipient_name"]; !ok && len(recipients) > 0 {
    payload["recipient_name"] = recipients[0]
}
```

**An toàn control-flow:** điểm chèn nằm **sau** guard `if len(recipients) == 0 { return ..., true }` (skip), nên `recipients[0]` luôn hợp lệ ⇒ `recipient_name` luôn được set non-empty. Khớp với template yêu cầu `required`.

**KHÔNG** thực hiện: batch email→name lookup, đổi `RecipientResolver` signature, thêm interface mới.

---

## 3. Files Modified

| File | Loại | Thay đổi |
|------|------|----------|
| `internal/reminder/app/service.go` | sản phẩm | thêm block điền `recipient_name` từ `recipients[0]` trong `prepareDispatch` |
| `internal/reminder/app/recipient_name_test.go` | test (mới) | 2 test recipient resolution |

Không đụng: resolver (`recipient_resolver.go`), query (`recipient_query.go`), template, database.

---

## 4. Test Result

### 4.1 Recipient resolution tests — ✅ PASS
```
go test -run 'TestRecipientName_' ./internal/reminder/app/
```
| Test | Kịch bản | Kết quả |
|------|----------|---------|
| `TestRecipientName_PrePopulatedRecipients_UsesEmail` | `RecipientEmails` pre-populated → `recipient_name` = email đầu tiên | ✅ PASS |
| `TestRecipientName_ResolvedRecipients_UsesResolvedEmail` | `RecipientEmails` rỗng → resolver trả email → `recipient_name` = email resolved | ✅ PASS |

### 4.2 Regression
- `internal/reminder/app` full package: ✅ PASS (gồm Fix3 portal_url, dispatch, Phase 3 helpers).
- **Full suite:** còn **đúng 8 failure pre-existing** (không đổi so với Phase 3), **0 regression mới**:
  - `notification/app` workflow.approved · `platform/config` Windows path · `httpserver` (5: template_category/session/state) · `companyaccess` feature-flag.
  - Tất cả đã kiểm chứng error message không liên quan feature (chi tiết ở Phase 3 handoff §4.4).

### 4.3 Build: ✅ `BUILD_EXIT=0`

---

## 5. Risks

| # | Risk | Mức độ | Ghi chú |
|---|------|--------|---------|
| P4-R1 | `recipient_name` hiển thị **email** thay vì `full_name` thật (mockup hiển thị "Phạm Thị Lan Hương") | 🟡 MEDIUM | Đúng spec "hoặc email" và ràng buộc no-redesign/no-batch. Nâng lên `full_name` là **follow-up** (cần resolver trả kèm tên). |
| P4-R2 | Khi recipient là nhóm (departments/admin fallback), chỉ greet theo `recipients[0]` | ⚪ LOW | Email gửi tới nhiều người nhưng lời chào theo người đầu. Chấp nhận cho bản này. |
| P4-R3 | Email là `login_id` (khi `users.email` rỗng) → lời chào là login_id | ⚪ LOW | Hệ quả của COALESCE sẵn có; hiếm. |

---

## 6. Trạng thái payload (sau Phase 1→4)

Tất cả 8/9 biến template đã được điền non-empty trong `prepareDispatch`:

| Biến | Nguồn | Phase |
|------|-------|-------|
| `company_name` | DispatchCandidate.CompanyName | (sẵn có) |
| `disclosure_title` | payload["title"] alias | (sẵn có) |
| `due_date` | ScheduledAt (DD/MM/YYYY) | (sẵn có) |
| `portal_url` | derive từ scope | (sẵn có) |
| `step_name` | GetStepByID.StageName | (sẵn có / P3 reuse) |
| `remaining_days` | calculateRemainingDays | **P3** |
| `urgency_status` | determineUrgencyStatus | **P3** |
| `implementation_guide` | step.Instructions → fallback | **P1 schema + P3 logic** |
| `recipient_name` | recipients[0] (email) | **P4** |

---

## 7. Ready For Phase 5

✅ **READY.** Payload đã đầy đủ cho cả 2 template. Phase 5 (Integration Testing) sẽ kiểm:
- Integration: dispatch end-to-end qua **real renderer** (embed registry) — xác nhận render không lỗi với payload `prepareDispatch` sản sinh (tất cả required vars non-empty).
- Contract tests (đã xanh cho 2 template mục tiêu).
- Render tests (subject/body khớp giá trị nghiệp vụ).

**Lưu ý mang sang Phase 5/6:**
- Pre-existing failures (8) sẽ vẫn đỏ — cần phân biệt rõ với kết quả feature khi báo cáo.
- Sequencing: template (P2) + payload (P3) + recipient (P4) phải lên **cùng** Phase 6, không deploy lẻ.

---

## 8. Trạng thái

PHASE 4 hoàn tất. Ownership xác định: **User / `users.full_name`**; điền `recipient_name` = email (đúng spec "hoặc email", 0 query, không redesign resolver). 2 test PASS. Build PASS. Full suite: 8 failure pre-existing, 0 regression mới.

⏸️ **DỪNG — chờ `CONFIRM PHASE 5`.**

Không tự động chuyển phase.
