# Ad-Hoc Alert Proposal Flow — Jira Ticket-Ready Checklist

## Cách dùng

Mỗi mục dưới đây có thể copy gần như nguyên khối vào Jira ticket.

Format cố định:

- Summary
- Problem
- Scope
- Out of scope
- Acceptance criteria
- Test scope
- Dependencies / notes

---

## P0-1

### Summary

`[P0] Align ad-hoc proposal deadline contract between FE, API, and DB`

### Problem

Hiện flow tạo proposal đang dùng các semantics không nhất quán:

- FE thu input theo nghĩa `proposed_deadline_days`
- wire body gửi field `proposed_deadline_date`
- DB schema lưu `proposed_deadline_date DATE`

Điều này tạo conflict contract và có rủi ro lưu/đọc sai nghĩa dữ liệu.

### Scope

- xác nhận business meaning đúng của field deadline trong proposal
- cập nhật FE form labels / field names / mapper
- cập nhật BE request contract và persistence mapping
- cập nhật docs liên quan

### Out of scope

- redesign toàn bộ deadline engine downstream
- thay đổi workflow runtime ngoài phạm vi proposal field semantics

### Acceptance criteria

- FE, API, DB dùng cùng một semantics cho deadline proposal
- không còn field naming mâu thuẫn giữa `days` và `date`
- create/get/list proposal trả và nhận dữ liệu đúng theo semantics đã chốt
- docs business và docs contract được cập nhật

### Test scope

- FE contract tests cho create/get/list proposal
- BE unit/integration tests cho create/get/list proposal
- manual test tạo proposal với giá trị deadline hợp lệ
- negative test với payload sai format / sai semantics

### Dependencies / notes

- cần BA/product chốt business meaning cuối cùng: “số ngày” hay “ngày tuyệt đối”

---

## P0-2

### Summary

`[P0] Promote ad-hoc proposal title and description to first-class backend contract`

### Problem

Hiện FE đang encode:

- dòng đầu `change_note` => `title`
- phần còn lại => `description`

Đây là contract suy diễn, không phải schema rõ ràng.

### Scope

- thêm contract rõ ràng cho `title` và `description` ở backend
- bỏ logic split/join `change_note` ở FE mapper
- cập nhật DB/DTO/API/docs nếu cần

### Out of scope

- redesign toàn bộ proposal table ngoài các field cần cho title/description

### Acceptance criteria

- proposal create/get/list hỗ trợ `title` và `description` rõ ràng
- FE không còn phải suy diễn title/description từ `change_note`
- export/report/integration có field semantics ổn định

### Test scope

- FE mapper tests
- FE contract tests
- BE unit/integration tests cho create/get/list
- regression test cho proposal detail/list rendering

### Dependencies / notes

- nếu vẫn giữ `change_note`, phải mô tả rõ nó còn mang nghĩa gì

---

## P0-3

### Summary

`[P0] Harden admin-approve consistency and idempotency for ad-hoc proposal final approval`

### Problem

`admin-approve` có side effect downstream:

- tạo disclosure record
- submit record
- tạo workflow instance

Hiện có rủi ro inconsistent state hoặc duplicate downstream entities nếu fail giữa chừng rồi retry.

### Scope

- review sequence thực tế của `admin-approve`
- xác định điểm partial-failure có thể xảy ra
- harden idempotency / progress persistence / replay
- bổ sung tests cho retry/failure scenarios

### Out of scope

- redesign toàn bộ disclosure/workflow modules

### Acceptance criteria

- retry `admin-approve` không tạo thêm downstream record/workflow ngoài dự kiến
- proposal progress phản ánh đúng trạng thái downstream
- failure giữa chừng có path recover rõ ràng

### Test scope

- BE unit tests cho reservation / replay
- integration tests cho mid-flight failure scenarios
- manual retry scenario nếu có thể mô phỏng local

### Dependencies / notes

- có thể cần sequence note hoặc ADR ngắn trước khi sửa code

---

## P0-4

### Summary

`[P0] Fix or clarify actor attribution when ad-hoc approval creates downstream disclosure record`

### Problem

Adapter hiện truyền `createdByMembershipID` vào cả `UserID` và `MembershipID` khi tạo disclosure record downstream. Điều này có thể làm sai actor attribution.

### Scope

- review actor mapping hiện tại
- xác nhận business rule đúng:
  - ai là actor tạo record downstream
  - field user id / membership id phải mang giá trị nào
- fix adapter và tests nếu attribution sai

### Out of scope

- redesign toàn bộ audit model của disclosure module

### Acceptance criteria

- disclosure record downstream có actor attribution đúng theo domain decision
- tests chứng minh field user/membership được set đúng

### Test scope

- adapter-level unit tests
- integration test cho admin-approve -> downstream record creation
- verification của persisted actor fields

### Dependencies / notes

- cần product/engineering thống nhất ai là “business actor” sau final approval:
  - proposer
  - final approver
  - system-generated on behalf of proposal

---

## P0-5

### Summary

`[P0] Add backend validation parity for ad-hoc proposal creation`

### Problem

FE đang validate nhiều hơn backend cho create proposal. API trực tiếp hiện có thể nhận payload không phản ánh đúng business expectations của UI.

### Scope

- lập validation matrix cho create proposal
- bổ sung backend validation còn thiếu
- đồng bộ error semantics FE/BE

### Out of scope

- validation cho các module không phải ad-hoc proposal

### Acceptance criteria

- create proposal API trả 4xx nhất quán cho payload thiếu/invalid
- FE và BE cùng hiểu một bộ rule validation
- direct API call không thể tạo proposal “invalid nhưng FE would block”

### Test scope

- BE unit tests cho invalid payload
- integration tests cho create endpoint
- FE regression test cho error handling nếu response shape thay đổi

### Dependencies / notes

- phụ thuộc một phần vào việc chốt semantics ở `P0-1` và contract cleanup ở `P0-2`

---

## P1-1

### Summary

`[P1] Add real draft editing for ad-hoc proposals`

### Problem

Current UX có “Lưu nháp” nhưng chưa có edit flow cho draft đã tạo.

### Scope

- draft edit API
- hydrate form từ draft
- edit CTA / flow ở FE

### Out of scope

- reopen rejected/cancelled proposal

### Acceptance criteria

- user có thể mở draft cũ, sửa và lưu lại
- chỉ draft mới được edit

### Test scope

- FE create/edit flow tests
- BE update draft tests

---

## P1-2

### Summary

`[P1] Gate ad-hoc create CTA by propose permission on disclosure type detail`

### Problem

CTA hiện xuất hiện theo loại `irregular`, chưa gate rõ theo `ad_hoc_alert.propose`.

### Scope

- update FE gating
- update regression test

### Out of scope

- backend permission changes

### Acceptance criteria

- user thiếu `ad_hoc_alert.propose` không thấy CTA

### Test scope

- FE regression tests cho CTA visibility

---

## P1-3

### Summary

`[P1] Show human-readable process controller info on ad-hoc proposal screens`

### Problem

UI hiện hiển thị `process_controller_id` thay vì tên/email.

### Scope

- enrich response hoặc FE mapping
- update detail/list UI

### Out of scope

- full people directory feature

### Acceptance criteria

- người dùng thấy tên/email người kiểm soát quy trình thay vì chỉ membership id

### Test scope

- FE rendering tests
- BE response tests nếu response được mở rộng

---

## P1-4

### Summary

`[P1] Clean deprecated ad-hoc proposal contract artifacts`

### Problem

Còn artifact deprecated hoặc không dùng rõ ràng trong FE/API/docs.

### Scope

- inventory artifact
- classify keep/remove
- cleanup safe additive

### Out of scope

- breaking-change cleanup không có migration plan

### Acceptance criteria

- docs, FE types, API contract gọn và nhất quán hơn

### Test scope

- FE build/tests
- BE tests nếu response shape thay đổi

---

## P1-5

### Summary

`[P1] Review audit and notification coverage for ad-hoc proposal stages`

### Problem

Current proposal stage chưa có story rõ cho audit và notifications.

### Scope

- decision note về audit events cần log
- decision note về notification events cần gửi
- nếu được duyệt, tạo implementation plan tiếp theo

### Out of scope

- implement full notification engine trong ticket này

### Acceptance criteria

- có tài liệu chốt event matrix cho audit/notification

### Test scope

- review-only hoặc ADR-style artifact; chưa yêu cầu runtime tests nếu chưa implement

---

## P2-1

### Summary

`[P2] Refresh BA-facing ad-hoc alert business docs to match runtime`

### Acceptance criteria

- business docs không overclaim
- phản ánh đúng identity-based final approval

---

## P2-2

### Summary

`[P2] Refresh API contract docs for effective-workflow permission boundary`

### Acceptance criteria

- `docs/api-contracts-json.md` khớp runtime hiện tại

---

## P2-3

### Summary

`[P2] Clarify product language between proposal and final alert/disclosure entities`

### Acceptance criteria

- product surfaces và docs dùng terminology nhất quán giữa `proposal` và entity downstream
