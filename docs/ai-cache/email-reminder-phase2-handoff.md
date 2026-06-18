# Email Reminder Content Upgrade — PHASE 2 Handoff (Email Template Contract)

**Mode:** Controlled Implementation — PHASE 2
**Date:** 2026-06-19
**Scope boundary:** `subject.txt` + `body.txt` + `body.html` + `meta.yaml` cho 2 template:
`reminder.deadline_approaching`, `reminder.workflow_step_due`.
**Explicitly NOT touched:** service, query, database.

> Lưu ý parity: contract `TestContract_VariableParity` kiểm tra biến khai báo (meta.yaml) phải == biến dùng trên **subject + body.txt + body.html gộp lại**. Vì cả 2 template đều có `body.txt`, mỗi template được cập nhật đủ **4 file** (meta + 3 file nội dung).

---

## 1. Template Diff

### 1.1 reminder.deadline_approaching (8 biến)

**meta.yaml** — từ 4 biến → 8 biến (tất cả `required: true`):
```
+ recipient_name        (required)
+ urgency_status        (required)
  company_name          (required)
  disclosure_title      (required)
  due_date              (required)
+ remaining_days        (required)
+ implementation_guide  (required)
  portal_url            (required)
```

**subject.txt**
```
- [CoBo] Nhắc nhở: {{.disclosure_title}} sắp đến hạn vào {{.due_date}}
+ [CẢNH BÁO] {{.urgency_status}} công bố {{.disclosure_title}}
```

**body.html / body.txt** (thay đổi nội dung chính):
- Lời chào: `Kính gửi,` → `Kính gửi {{.recipient_name}},`
- Câu mở: cố định → `... đầu mục công việc dưới đây {{.urgency_status}} thực hiện:`
- Row "Nghĩa vụ công bố" → đổi nhãn **"Công việc"** (vẫn `{{.disclosure_title}}`)
- Row "Hạn nộp" → **"Hạn chót"**, bổ sung **`Số ngày còn lại: {{.remaining_days}} ngày`** cùng dòng
- **Mục mới "Hướng dẫn thực hiện:"** → block `{{.implementation_guide}}` (HTML: `white-space:pre-line`, viền trái xanh)
- CTA: link text trần → **button "Xem thêm"** (`<a href="{{.portal_url}}">`, style nút bo góc)

### 1.2 reminder.workflow_step_due (9 biến)

**meta.yaml** — từ 5 biến → 9 biến (tất cả `required: true`); thêm: `recipient_name`, `urgency_status`, `remaining_days`, `implementation_guide` (giữ `step_name`).

**subject.txt**
```
- [CoBo] Cần xử lý: Bước {{.step_name}} của {{.disclosure_title}} đến hạn {{.due_date}}
+ [CẢNH BÁO] {{.urgency_status}} bước {{.step_name}} - {{.disclosure_title}}
```

**body.html / body.txt**: cùng bố cục với deadline_approaching, **giữ thêm row "Bước cần xử lý": {{.step_name}}**; bổ sung `recipient_name`, `urgency_status`, `remaining_days`, mục "Hướng dẫn thực hiện" (`implementation_guide`), button "Xem thêm".

### 1.3 Quyết định thiết kế (kế thừa khuyến nghị PHASE 0)
- **Tất cả biến `required: true`** (theo CL-1(a)): renderer **fail-loud** nếu thiếu/rỗng → email **không gửi** thay vì rò rỉ `<no value>`. ⇒ ràng buộc cho Phase 3/4: **mọi biến phải được điền non-empty**.
- **`urgency_status` là cụm tự chứa**: Phase 3 sẽ set `"Sắp đến hạn"` / `"Đã đến hạn"` / `"Quá hạn"`. Subject `[CẢNH BÁO] {{.urgency_status}} công bố {{.disclosure_title}}` khớp **đúng** mockup ("[CẢNH BÁO] Sắp đến hạn công bố ...").
- **Button text = "Xem thêm"** (theo content spec dạng chữ; mockup minh hoạ "Xử lý ngay"). Đã được phép chỉnh text; keys đảm bảo đầy đủ.

---

## 2. Contract Result

Lệnh: `go -C cobo_iam_services test -run TestContract_ -v ./internal/notification/app/`

**2 template mục tiêu — TẤT CẢ subtests PASS:**

| Test | reminder.deadline_approaching | reminder.workflow_step_due |
|------|:---:|:---:|
| `TestContract_VariableParity` | ✅ PASS | ✅ PASS |
| `TestContract_RenderForbiddenContent` | ✅ PASS | ✅ PASS |
| `TestContract_CTAAbsolute` | ✅ PASS | ✅ PASS |
| `TestContract_GoldenCoverage` | ✅ PASS (cấp suite) | |

→ Parity đúng (declared == used), không token cấm (`<no value>`/`{{`/`localhost`/...), CTA href absolute https.

**Lỗi package-level (pre-existing, KHÔNG do Phase 2):**
- `TestContract_VariableParity/workflow.approved` → FAIL: `workflow_instance_id` khai báo trong meta nhưng không dùng. Đã ghi nhận tại Phase 1 handoff (mục 7 P1-R2). Template `workflow.approved` **không nằm trong scope Phase 2** và **không bị Phase 2 chạm tới**.
- Xử lý theo **option (a)** đã nêu ở Phase 1 handoff: chấp nhận lỗi pre-existing, chỉ verify subtests của 2 template mục tiêu PASS. (Chưa sửa `workflow.approved` vì ngoài scope — cần user quyết nếu muốn fix vệ sinh.)

---

## 3. Render Result

`TestContract_RenderForbiddenContent` render **end-to-end** (subject + body.txt + body.html) cả 2 template với đầy đủ biến mẫu → **PASS** (không placeholder chưa resolve, không token cấm). Đây là bằng chứng render tự động.

**Minh hoạ render với dữ liệu thực tế (mockup):**

`reminder.deadline_approaching`, urgency_status="Sắp đến hạn":
```
Subject: [CẢNH BÁO] Sắp đến hạn công bố Báo cáo tài chính năm 2025

Kính gửi Phạm Thị Lan Hương,
Hệ thống CoBo Portal xin thông báo đầu mục công việc dưới đây Sắp đến hạn thực hiện:

  Công ty:   Công ty CP ABC
  Công việc: Báo cáo tài chính năm 2025
  Hạn chót:  31/03/2026   Số ngày còn lại: 12 ngày

Hướng dẫn thực hiện:
  Tổng hợp số liệu, lập BCTC năm đã kiểm toán và nộp qua hệ thống.

  [ Xem thêm ]  → https://portal.cobo.vn/app/disclosures/disc-123
```

`reminder.workflow_step_due`, urgency_status="Đã đến hạn":
```
Subject: [CẢNH BÁO] Đã đến hạn bước Soát xét tài chính - Báo cáo tài chính năm 2025

  Công ty:        Công ty CP ABC
  Công việc:      Báo cáo tài chính năm 2025
  Bước cần xử lý: Soát xét tài chính
  Hạn chót:       28/03/2026   Số ngày còn lại: 0 ngày

Hướng dẫn thực hiện:
  Kiểm tra đối chiếu số liệu trước khi trình ký.

  [ Xem thêm ]  → https://portal.cobo.vn/app/disclosures/disc-123
```

> Các giá trị `urgency_status`, `remaining_days`, `recipient_name`, `implementation_guide` ở trên là **minh hoạ**; logic điền thật là Phase 3 (3 biến đầu + guide) và Phase 4 (`recipient_name`).

---

## 4. Files Modified

| File | Loại |
|------|------|
| `internal/notification/templates/reminder.deadline_approaching/meta.yaml` | 4→8 biến |
| `internal/notification/templates/reminder.deadline_approaching/vi/subject.txt` | nội dung |
| `internal/notification/templates/reminder.deadline_approaching/vi/body.txt` | nội dung |
| `internal/notification/templates/reminder.deadline_approaching/vi/body.html` | nội dung |
| `internal/notification/templates/reminder.workflow_step_due/meta.yaml` | 5→9 biến |
| `internal/notification/templates/reminder.workflow_step_due/vi/subject.txt` | nội dung |
| `internal/notification/templates/reminder.workflow_step_due/vi/body.txt` | nội dung |
| `internal/notification/templates/reminder.workflow_step_due/vi/body.html` | nội dung |

Không tạo file mới. Không xoá file.

---

## 5. Risks

| # | Risk | Mức độ | Ghi chú |
|---|------|--------|---------|
| P2-R1 | Template yêu cầu 8/9 biến `required` nhưng Phase 3/4 chưa điền → **render fail, email không gửi** nếu deploy lẻ | 🔴 HIGH | **Không deploy Phase 2 độc lập.** Template + payload (P3) + recipient (P4) phải lên cùng Phase 6. Kế thừa PHASE 0 R1. |
| P2-R2 | `notification/app` đỏ do `workflow.approved` (pre-existing) | 🟡 MEDIUM | Ngoài scope; xử lý option (a). Cân nhắc fix vệ sinh riêng (cần user duyệt). |
| P2-R3 | `urgency_status` mid-sentence viết hoa ("...dưới đây Sắp đến hạn thực hiện") hơi cứng văn phong | ⚪ LOW | Chấp nhận trong ngữ cảnh email cảnh báo; có thể tinh chỉnh sau. Keys không đổi. |
| P2-R4 | Button text "Xem thêm" khác mockup "Xử lý ngay" | ⚪ LOW | Theo content spec dạng chữ; user đã cho phép chỉnh text. |

---

## 6. Build/Test Summary

- Contract tests 2 template mục tiêu: ✅ **PASS** (Variable Parity / Forbidden Content / CTA Absolute).
- Package `notification/app`: đỏ **chỉ vì** pre-existing `workflow.approved` (không liên quan Phase 2).
- Không build lại toàn module ở Phase 2 (scope chỉ template — không có Go code thay đổi). Build Go không bị ảnh hưởng vì template là embed asset; thay đổi nội dung không phá compile.

---

## 7. Ready For Phase 3

✅ **READY** — Phase 3 (Payload Enrichment) phải điền **non-empty** cho các biến template vừa khai báo:
- `remaining_days`, `urgency_status`, `implementation_guide` ← **Phase 3** (đúng scope P3).
- `recipient_name` ← **Phase 4**.
- `company_name`, `disclosure_title`, `due_date`, `portal_url`, `step_name` ← đã có sẵn từ pipeline hiện tại (xác nhận lại ở Phase 3).

**Open clarifications cần chốt trước Phase 3:**
- **CL-1:** `implementation_guide` — Phase 2 đã khoá `required: true`. Phase 3 PHẢI đảm bảo fallback non-empty (step.Instructions → generic string). Xác nhận hướng (a) đơn giản, không cần cross-module reader.

**Mapping urgency_status (đề xuất cho Phase 3, chờ xác nhận):**
| Điều kiện (theo remaining_days) | urgency_status |
|---|---|
| remaining_days > 0 | `Sắp đến hạn` |
| remaining_days == 0 | `Đã đến hạn` |
| remaining_days < 0 | `Quá hạn` |

---

## 8. Trạng thái

PHASE 2 hoàn tất. Contract tests cho 2 template mục tiêu: **PASS**. Lỗi còn lại là pre-existing `workflow.approved` (ngoài scope, đã ghi nhận).

⏸️ **DỪNG — chờ `CONFIRM PHASE 3`.**

Không tự động chuyển phase.
