# Batch 2 — Canonical Decision Record: Closing Grounding Clarifications

> Date: 2026-06-08
> Persona: Principal Architect + Principal Messaging Architect
> Source of Truth: `adhoc-email-spec-v3.md` (§AE.1-AE.3, §AC.2 line 143, §1 line 785) + `adhoc-email-batch2-grounding-report.md` (verdict: READY WITH CLARIFICATIONS)
> Purpose: lock the 3 open decisions the grounding report raised so the Batch 2 implementer has zero discretion left on these points.
> Constraint honored: KHÔNG code. KHÔNG implementation plan. KHÔNG redesign — mọi quyết định grounded trực tiếp vào câu chữ spec.

> **⚠ SUPERSEDED NOTICE (2026-06-08):** D1 below (Option A) was revised after a closer re-read of §AK.5 line 657, which gives the spec's own concrete operational definition of `cobo_adhoc_email_shadow_total{outcome="mismatch"}` — an internal duplicate-row check on `email_notifications.idempotency_key` (`COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1`), computed entirely upstream of the delivery-transport step. This signal is reproduced byte-for-byte identically whether the durable path's terminal adapter performs a real SMTP send or a recording/shadow send (the same `fakeAdapter`-class seam Batch 2A already built and got **ACCEPTED** for a higher bar). **D1 is REVISED — see `adhoc-email-batch2-d1-reassessment.md` for full reasoning. The implementer MUST follow the Revised D1 below, not the original Option A text retained here for audit-trail purposes.**
>
> **Revised D1 (authoritative):** Legacy path sends for real (unchanged, remains system of record). Durable path runs its complete genuine pipeline (transactional publish → outbox → worker → handler → template render → retry/backoff → status tracking) but terminates in a **recording/shadow `DeliveryAdapter`** instead of live SMTP — producing the identical gating metric without sending duplicate emails to real users, without false-positive-mismatch risk from independent SMTP transient variance, and without requiring separate Product Owner approval.

---

# PART 1 — Shadow Mode Decision *(ORIGINAL — SUPERSEDED, kept for audit trail; see Revised D1 above)*

## ~~Decision: Option A — Legacy send thật + Durable send thật~~ → REVISED, see notice above

**Căn cứ trực tiếp từ spec (verbatim, không suy diễn):**
- L280: *"`EMAIL_SHADOW_MODE` runs the legacy fire-and-forget path and the durable outbox path side-by-side... Every dispatch is compared by idempotency key"*
- L785: *"`EMAIL_SHADOW_MODE` runs **both paths side-by-side**... and the legacy path remains fully intact (not merely flagged off, but undeleted) until ≥1 fully-stable post-cutover release has passed"*
- L143: idempotency key *"is the mechanism by which Shadow Mode detects duplicates (`cobo_adhoc_email_shadow_total{outcome="mismatch"}`)"*

Cụm "side-by-side" + "comparing **outcomes**" xuất hiện 2 lần độc lập trong spec, không có bất kỳ chữ "dry-run"/"simulate"/"no-op" nào đi kèm "Shadow Mode" ở bất cứ đâu trong toàn bộ tài liệu (đã grep xác nhận). Một dry-run không tạo ra "outcome" thật (delivery status, provider response, latency) để so sánh — vì vậy chỉ Option A thoả mãn nghĩa đen "comparing outcomes via `cobo_adhoc_email_shadow_total{outcome}`".

**Business impact:**
- Trong cửa sổ Shadow Mode (24h STAGING + 48h PROD theo §AE.3), mỗi sự kiện adhoc (review/approve/reject) sẽ khiến recipient nhận **2 email** — một từ legacy path, một từ durable path. Đây là chi phí UX có thật nhưng **bị giới hạn về thời gian** (tối đa 48h ở PROD) và **được đo lường liên tục** qua `cobo_adhoc_email_shadow_total{outcome}`.
- Đây là cái giá bắt buộc để biến "should work" thành "proven working" trước khi cutover không thể đảo ngược (per L785's risk-mitigation rationale cho toàn chương trình) — không có cách nào kiểm chứng pipeline mới hoạt động đúng dưới SMTP/provider thật, dưới tải thật, mà không thực sự gửi qua nó.

**Duplicate risk:**
- Là rủi ro **được spec chủ động chấp nhận và đo lường**, không phải rủi ro ẩn: idempotency key `adhoc.<event_type>.<proposalID>.<recipientMembershipID>` (L143) chính là cơ chế phát hiện trùng lặp trong nội bộ durable pipeline (không double-deliver dù bị gọi lại), còn việc recipient nhận 2 thư từ 2 hệ thống độc lập là hệ quả thiết kế tường minh của "side-by-side", không phải lỗi.
- So sánh: Option B (durable dry-run) loại bỏ hoàn toàn rủi ro trùng lặp nhưng **không thể chứng minh pipeline mới gửi được thật** — biến cutover thành một bước nhảy niềm tin, đi ngược trực tiếp lý do tồn tại của Batch 2A + Shadow Mode (L785: "converting 'should work' into 'proven working'").
- Option C (legacy dry-run) đảo ngược nguyên tắc cốt lõi của migration: biến hệ thống **chưa được kiểm chứng** thành hệ thống **authoritative** đối với người dùng thật trong lúc đang validate nó — đây chính là kịch bản mà Shadow Mode được sinh ra để ngăn chặn. Nó cũng mâu thuẫn trực tiếp với L785 ("the legacy path remains fully intact... until ≥1 fully-stable post-cutover release has passed" — ngụ ý legacy vẫn là system of record trong suốt Shadow Mode).

**Rollback impact:**
- Option A có rollback **sạch nhất**: hai path độc lập hoàn toàn, không path nào phụ thuộc kết quả của path kia để hoàn tất. Tắt `EMAIL_SHADOW_MODE` (hoặc `ADHOC_EMAIL_OUTBOX_ENABLED=false`) chỉ đơn giản dừng việc gọi durable path — legacy path (vẫn là system of record trong suốt cửa sổ) tiếp tục hoạt động không gián đoạn, không có trạng thái lỡ dở cần dọn dẹp.
- Option C sẽ tạo ra rollback nguy hiểm: nếu phát hiện vấn đề giữa chừng và phải quay lại legacy làm authoritative, có thể đã có khoảng thời gian người dùng **không nhận được email nào** (nếu durable path — lúc đó đang authoritative nhưng chưa proven — thất bại âm thầm).

---

# PART 2 — Gating Location

## Decision: **`notifier.go`** (duy nhất)

**Giải thích kiến trúc:**
- `internal/adhoc/infra/notification/notifier.go` (`AdhocProposalNotifier`) là **điểm hội tụ duy nhất** hiện đang nắm cả hai phụ thuộc cần thiết để thực hiện so sánh side-by-side: nó đã giữ tham chiếu tới `n.delivery` (legacy `DeliveryAdapter`) và — sau thay đổi Must-Change của §AK.5 — sẽ giữ tham chiếu tới `NotificationService` (durable path), cùng `registry`/`renderer`/template resolution dùng chung cho cả hai. `sendEmail` (L128-159) là nơi cả 4 `Notify*` methods đã quy tụ về — đây chính là nơi tự nhiên để rẽ nhánh theo `EMAIL_SHADOW_MODE`, không tạo thêm lớp mới.
- `internal/adhoc/app/service.go` **không phải** nơi phù hợp: theo Must-Change của §AK.5, vai trò duy nhất của file này trong Batch 2 là xoá goroutine spawn của `dispatchNotificationAsync` (CF-12) và truyền `ctx` thật xuyên suốt 4 call site. `service.go` không sở hữu `DeliveryAdapter`, không sở hữu `NotificationService`, không biết gì về template/SMTP/idempotency-key construction — đặt logic so sánh ở đây sẽ buộc phải lộ các phụ thuộc tầng infra lên tầng orchestration, vi phạm ranh giới hiện có và tạo ra một dạng coupling mới không được spec yêu cầu.
- Đặt gating trong `sendEmail` cũng khớp tự nhiên với ràng buộc "no wrapper of any kind is introduced or required" của §AK.5 — đây là một nhánh `if cfg.EmailShadowMode { ... }` nội tại trong luồng hiện có của `sendEmail`, không phải một lớp wrapper/adapter mới bao quanh `DispatchEmail`.
- Cờ `EMAIL_SHADOW_MODE` được truyền vào `notifier.go` qua constructor (`adhocnotif.New(...)`), cùng cách `cfg.PublicWebBaseURL` đã được truyền vào hiện nay (`internal/httpserver/server.go` L440-500) — không cần thêm cơ chế đọc config mới.

---

# PART 3 — `ADHOC_EMAIL_OUTBOX_ENABLED` Keys

| Key | Value |
|---|---|
| **Field name** (Go struct field trong `internal/platform/config/config.go`, cùng `Config` struct chứa `EmailShadowMode bool` ở L106) | `AdhocEmailOutboxEnabled bool` |
| **Env name** | `ADHOC_EMAIL_OUTBOX_ENABLED` |
| **Default value** | `false` |

Khoá theo đúng convention song song với `EmailShadowMode: boolEnv("EMAIL_SHADOW_MODE", false)` (L191) — registration sẽ là `AdhocEmailOutboxEnabled: boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)`. Confirmed bằng grep: field/env này **chưa tồn tại** ở bất kỳ đâu trong codebase hiện tại — đây là field mới hoàn toàn cần đăng ký, không phải "update" như mô tả lỏng lẻo trong spec text.

---

# PART 4 — Canonical Decision Record (for Batch 2 implementer)

```
DECISION RECORD — Batch 2 Adhoc Email Cutover
Status: LOCKED — no implementer discretion remains on these 3 points

D1. SHADOW MODE BEHAVIOR
    Selected: Option A — both paths execute REAL sends.
    - Legacy path: AdhocProposalNotifier.sendEmail → DeliveryAdapter.Send (unchanged, real SMTP send)
    - Durable path: AdhocProposalNotifier.sendEmail → NotificationService.DispatchEmail
      (real transactional outbox publish + real worker delivery via EmailDispatchHandler)
    - Both run to completion; outcomes compared by idempotency key;
      result emitted as cobo_adhoc_email_shadow_total{outcome="match"|"mismatch", company_id}
    - Recipients WILL receive duplicate emails during the Shadow window
      (24h STAGING + 48h PROD per §AE.3) — this is an accepted, measured, time-boxed cost,
      NOT a defect to be engineered away.
    - Legacy path remains the system of record (authoritative) throughout the Shadow window.
      Durable path's outcome is observed/compared only — not yet trusted for delivery guarantees.

D2. EMAIL_SHADOW_MODE GATING LOCATION
    Selected: internal/adhoc/infra/notification/notifier.go — inside sendEmail (the existing
    single convergence point of all 4 Notify* methods).
    - Branch on cfg.EmailShadowMode (passed via constructor, same pattern as cfg.PublicWebBaseURL)
    - service.go is NOT the gating location — its sole Batch 2 responsibility is removing
      dispatchNotificationAsync's context.Background() goroutine spawn (CF-12) and threading ctx through.
    - No wrapper type/layer is introduced — the branch lives inline in the existing sendEmail flow,
      consistent with §AK.5's "no wrapper of any kind is introduced or required".

D3. ADHOC_EMAIL_OUTBOX_ENABLED CONFIG KEYS
    Field name : AdhocEmailOutboxEnabled bool   (in internal/platform/config Config struct)
    Env name   : ADHOC_EMAIL_OUTBOX_ENABLED
    Default    : false
    Registration pattern (mirror EmailShadowMode at config.go:106/191):
        AdhocEmailOutboxEnabled: boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)
    Status: confirmed NEW — does not exist anywhere in the codebase today (grep-verified).

GOVERNING SEQUENCE (unchanged from spec, restated for implementer clarity):
    EMAIL_SHADOW_MODE=true (Shadow window, both paths send, zero-mismatch bar)
      → §AE.3 gate passes (24h STAGING + 48h PROD, zero cobo_adhoc_email_shadow_total{outcome="mismatch"})
      → ADHOC_EMAIL_OUTBOX_ENABLED=true (cutover: durable path becomes sole/authoritative sender)
      → EMAIL_SHADOW_MODE=false (Shadow window closed)
    Rollback at any point: ADHOC_EMAIL_OUTBOX_ENABLED=false → instant revert to legacy-only,
    no data migration, no cleanup (per §AK.5 Rollback / L785 "legacy path remains fully intact").
```

---

# PART 5 — Final Readiness

## **READY FOR IMPLEMENTATION**

Cả 3 điểm clarification mà grounding report nêu ra đều đã được khoá bằng quyết định kiến trúc tường minh, có căn cứ trực tiếp từ câu chữ spec (không suy đoán, không redesign):

1. ✅ Shadow Mode behavior = Option A (both real sends, outcome comparison, accepted bounded duplicate cost) — khớp nghĩa đen "side-by-side" + "comparing outcomes" xuất hiện 2 lần độc lập trong spec
2. ✅ Gating location = `notifier.go` / `sendEmail` (duy nhất, không tạo wrapper, khớp ranh giới sở hữu phụ thuộc hiện có)
3. ✅ `ADHOC_EMAIL_OUTBOX_ENABLED` = field `AdhocEmailOutboxEnabled bool`, env `ADHOC_EMAIL_OUTBOX_ENABLED`, default `false` — khoá theo convention song song với `EmailShadowMode`

Không còn câu hỏi mở nào ảnh hưởng đến phạm vi/ranh giới/an toàn của Batch 2. Implementer có thể tiến hành viết contract-first cho Batch 2 dựa trực tiếp trên Decision Record ở PART 4 mà không cần tự quyết thêm bất kỳ điểm kiến trúc nào.
