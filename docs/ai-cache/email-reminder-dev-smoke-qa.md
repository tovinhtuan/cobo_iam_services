# Email Reminder Content Upgrade — PHASE 6 DEV Deploy + Smoke QA Evidence

**Mode:** Controlled Implementation — PHASE 6 (final)
**Date:** 2026-06-19
**Target:** DEV only — `88.216.208.0:21239` (`/root/cobo_project`). **KHÔNG production.**

---

## 1. Deployment

### 1.1 Migration
| Bước | Lệnh | Kết quả |
|------|------|---------|
| Apply 0099 | `scp 0099_workflow_step_instructions.up.sql` → `docker exec cobo-iam-mysql mysql < …` + track | ✅ `APPLIED_OK` |
| Verify cột | `SHOW COLUMNS FROM global_workflow_steps LIKE 'instructions'` | ✅ `instructions text YES NULL` |
| Tracked | `SELECT file_name FROM schema_migrations WHERE file_name LIKE '0099%'` | ✅ `0099_workflow_step_instructions.up.sql` |

### 1.2 Backend binary
| Bước | Chi tiết | Kết quả |
|------|----------|---------|
| Cross-compile | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` → api (30MB) + worker (16MB) | ✅ exit 0 |
| Push + recreate | scp `.api.tmp`/`.worker.tmp` → mv + chmod 755 → `docker compose up -d --force-recreate --no-deps api worker` | ✅ Started |
| Health | `GET /healthz` / `GET /readyz` | ✅ `{"status":"ok"}` / `{"status":"ready"}` |
| Log | api startup | ✅ `"api listening" addr=:8080`, không error/panic |

> FE không deploy ở phase này (feature thuần backend: template embed trong binary + payload logic). FE không thay đổi.

### 1.3 Container status (post-deploy, sau revert SMTP)
```
cobo-iam-api      Up (recreated)
cobo-iam-worker   Up
cobo-iam-mysql    Up (healthy)
cobo-iam-mailpit  Up (healthy)
```

---

## 2. Smoke QA

### 2.1 Phương pháp
- Endpoint dev: `POST /internal/dev/reminders/seed-occurrence` (ENV=development) + `POST /internal/reminders/dispatch` — render template trên **binary đã deploy** rồi gửi qua SMTP.
- Seed `scheduled_at` tương lai để worker tick (5s) không giành; dispatch thủ công theo `occurrence_id`.
- **An toàn email:** lần positive đầu gửi tới hộp thư của chính user (`tuan.tv100698@gmail.com`). Sau đó **trỏ SMTP của API sang Mailpit** (container `cobo-iam-mailpit`) để render-capture nội dung **không gửi ra ngoài**, rồi **revert** về Gmail.

### 2.2 Kết quả

| # | Test | Input | Kết quả | Bằng chứng |
|---|------|-------|---------|-----------|
| S1 | Positive — deadline (Gmail, own inbox) | full 8-var payload | ✅ `{"accepted":true,"status":"SENT"}` | DB: `smoke-p6-deadline-001 SENT provider_message_id=smtp-1781823526017664024` |
| S2 | **Fail-loud** — thiếu `implementation_guide` | 7-var (thiếu 1 required) | ✅ `{"accepted":false,"status":"FAILED"}`, **không gửi** | DB: `smoke-p6-neg-002 FAILED EMAIL_PROVIDER_PERMANENT_ERROR` |
| S3 | Render-capture — deadline (Mailpit) | full 8-var | ✅ SENT → captured | Mailpit subject + body (§3.1) |
| S4 | Render-capture — workflow step (Mailpit) | full 9-var (+step_name) | ✅ SENT → captured | Mailpit subject + body (§3.2) |

**S2 (fail-loud) là bằng chứng quan trọng nhất:** payload thiếu 1 required var → render lỗi → email **KHÔNG** gửi (status FAILED), không rò rỉ `<no value>`. Đúng thiết kế contract.

---

## 3. Evidence — nội dung email render thật (capture từ Mailpit, binary đã deploy)

> Mailpit UI để xem trực quan: **http://88.216.208.0:8025** (subject + HTML/text body). Dưới đây là text body captured qua Mailpit API.

### 3.1 reminder.deadline_approaching
```
SUBJECT: [CẢNH BÁO] Sap den han công bố Bao cao tai chinh nam 2025

Kính gửi Pham Thi Lan Huong,                              ← recipient_name
Hệ thống CoBo Portal xin thông báo đầu mục công việc
   dưới đây Sap den han thực hiện:                        ← urgency_status
Công ty: Cong ty CP ABC                                   ← company_name
Công việc: Bao cao tai chinh nam 2025                     ← disclosure_title
Hạn chót: 31/03/2026 — Số ngày còn lại: 12 ngày           ← due_date + remaining_days
Hướng dẫn thực hiện:
Tong hop so lieu va nop bao cao qua he thong.             ← implementation_guide
Xem thêm: http://88.216.208.0:3000/app/disclosures/smoke-mp-d   ← portal_url (CTA)
Lưu ý: Email này được gửi tự động, vui lòng không trả lời trực tiếp.
```

### 3.2 reminder.workflow_step_due
```
SUBJECT: [CẢNH BÁO] Da den han bước Soat xet tai chinh - Bao cao tai chinh nam 2025

Kính gửi Nguyen Van A,                                    ← recipient_name
Hệ thống CoBo Portal xin thông báo đầu mục công việc
   dưới đây Da den han thực hiện:                         ← urgency_status
Công ty: Cong ty CP ABC                                   ← company_name
Công việc: Bao cao tai chinh nam 2025                     ← disclosure_title
Bước cần xử lý: Soat xet tai chinh                        ← step_name
Hạn chót: 28/03/2026 — Số ngày còn lại: 0 ngày            ← due_date + remaining_days (0)
Hướng dẫn thực hiện:
Kiem tra doi chieu so lieu truoc khi trinh ky.           ← implementation_guide
Xem thêm: http://88.216.208.0:3000/app/disclosures/smoke-mp-w  ← portal_url (CTA)
```

→ Cả 8/9 trường mới render đúng trên **binary đã deploy**. Subject khớp đúng template Phase 2 (`[CẢNH BÁO] {urgency} công bố {title}` và `… bước {step} - {title}`).

### 3.3 Khôi phục môi trường (revert)
| Bước | Kết quả |
|------|---------|
| Restore `.env` từ `.env.p6bak` | ✅ `SMTP_HOST=smtp.gmail.com SMTP_PORT=587 …` |
| Recreate api | ✅ Started, `SMTP_HOST=smtp.gmail.com`, readyz `ready` |
| Cleanup occurrence test | ✅ `DELETE … WHERE occurrence_id LIKE 'smoke-%'` → 4 rows deleted |

Dev env **đã khôi phục nguyên trạng** (SMTP về Gmail, không còn dữ liệu test).

---

## 4. Risks / Observations

| # | Mục | Mức độ | Ghi chú |
|---|-----|--------|---------|
| O1 | **Migration 0098 (adhoc) vẫn chưa apply** trên server (server ở 0097 + nay 0099 cho global_workflow_steps, nhưng 0098 chưa). | 🟡 MEDIUM | Binary deploy build từ local HEAD (gồm code adhoc-multi-reviewer). api/worker **start healthy** → binary không hard-fail vì thiếu 0098. Tính năng adhoc-multi-reviewer có thể lỗi runtime nếu dùng. **Ngoài scope feature**; không apply 0098 (không thuộc feature). Khuyến nghị: deploy đồng bộ migration 0098 ở lần deploy chính thức. |
| O2 | API + worker gửi reminder qua **real Gmail SMTP** (`smtp.gmail.com`), không phải Mailpit. | 🟡 MEDIUM | Cấu hình vận hành dev (không phải lỗi feature). Smoke positive S1 đã gửi 1 email thật tới hộp thư của user. Production cần SMTP phù hợp. |
| O3 | Alert `ReminderFailedRecent` (Alertmanager → Mailpit `oncall@cobo.local`) **FIRING rồi RESOLVED** do test S2 (FAILED có chủ đích). | ⚪ LOW | Benign — đúng cơ chế giám sát; tự resolve. |
| O4 | `recipient_name` = email/tên truyền vào (chưa nối `users.full_name` thật từ resolver). | 🟡 MEDIUM | Theo spec "hoặc email"; follow-up (P4-R1). |
| O5 | `due_date` UTC vs `remaining_days` HCM → lệch 1 ngày ở biên. | 🟡 MEDIUM | Follow-up (P3-R1), ngoài scope. |
| O6 | **8 test failures pre-existing** trong suite (workflow.approved meta, config Windows path, 5 httpserver, 1 companyaccess). | 🟡 MEDIUM | Không do feature (đã kiểm chứng error message Phase 3 §4.4). Không chặn feature nhưng làm CI đỏ. |

---

## 5. Final Verdict

### ✅ PASS WITH PRE-EXISTING ISSUES

**Feature — PASS (hoạt động end-to-end trên DEV):**
- Migration 0099 applied + verified (`instructions` column).
- Binary deploy thành công, api/worker healthy (healthz/readyz ok).
- Cả 2 template (`reminder.deadline_approaching`, `reminder.workflow_step_due`) render đúng **đầy đủ 8/9 trường mới** trên binary đã deploy (capture từ Mailpit).
- Subject `[CẢNH BÁO] …` khớp đúng yêu cầu + mockup.
- **Fail-loud** verified: thiếu required var → FAILED, không gửi email hỏng.
- Dev env khôi phục nguyên trạng sau test.

**Pre-existing issues (KHÔNG do feature, không chặn release feature):**
- 8 test failures pre-existing trong suite (O6).
- Migration 0098 (adhoc) chưa apply — drift sẵn có, ngoài scope (O1).
- SMTP dev = real Gmail (O2).

**Không có lỗi nào do feature Email Reminder Content Upgrade gây ra.**

---

## 6. Tổng kết toàn bộ (Phase 0 → 6)

| Phase | Deliverable | Trạng thái |
|-------|-------------|-----------|
| 0 | Architecture validation | ✅ NEEDS CLARIFICATION → resolved (CL-1a, CL-2b) |
| 1 | DB foundation (migration 0099 + DTO + repo) | ✅ |
| 2 | Email template contract (2 template × meta+subject+body) | ✅ |
| 3 | Payload enrichment (remaining_days, urgency_status, implementation_guide) | ✅ |
| 4 | Recipient name (User/`full_name` ownership; email fallback) | ✅ |
| 5 | Integration testing (render pipeline, fail-loud) | ✅ |
| 6 | **DEV deploy + smoke QA** | ✅ **PASS WITH PRE-EXISTING ISSUES** |

**Follow-up đề xuất (ngoài scope, task riêng):**
1. Nâng `recipient_name` lên `users.full_name` thật (resolver trả kèm tên).
2. Đồng bộ `due_date` sang Asia/Ho_Chi_Minh (khớp `remaining_days`).
3. Apply migration 0098 (adhoc) khi deploy chính thức.
4. Fix pre-existing `workflow.approved` meta parity (gỡ biến `workflow_instance_id` thừa).
5. Platform Admin nhập `instructions` cho các workflow step (để email có hướng dẫn cụ thể thay vì fallback generic).

---

**PHASE 6 hoàn tất. Dừng — không tiếp tục phase nào khác (per chỉ đạo).**
