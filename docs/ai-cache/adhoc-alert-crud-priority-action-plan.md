# Ad-Hoc Alert Proposal Flow — Priority Action Plan

## Mục tiêu

Biến kết quả review current-state thành backlog thực thi rõ ràng cho dev team, theo mức ưu tiên:

- `P0`: cần xử lý sớm vì có rủi ro correctness / data / contract cao
- `P1`: nên xử lý để giảm tech debt và làm flow product sạch hơn
- `P2`: cải thiện UX, tài liệu, và mở rộng tính năng

Phạm vi của plan này là **luồng proposal cảnh báo bất thường**, không bao gồm tab `Cảnh báo thời hạn`.

---

## P0 — Correctness / Contract / Data Safety

## P0.1 Fix contract `proposed_deadline_days` vs `proposed_deadline_date`

### Vấn đề

- FE đang thu input theo nghĩa “số ngày deadline đề xuất”
- wire body hiện gửi vào field tên `proposed_deadline_date`
- DB schema cũng đang lưu `proposed_deadline_date DATE`

Đây là conflict contract mức cao.

### Mục tiêu

Chốt một semantics duy nhất:

1. hoặc backend/DB thực sự lưu **ngày deadline tuyệt đối**
2. hoặc đổi contract để backend/DB lưu **số ngày deadline**

### Khuyến nghị

Ưu tiên làm rõ domain trước:

- nếu business muốn “số ngày”, tạo field chuẩn `proposed_deadline_days`
- nếu business muốn “ngày cụ thể”, FE phải đổi input/label/validation cho đúng

### Deliverables

- contract note được chốt
- FE form + mapper cập nhật
- BE request/DB mapping cập nhật
- test contract FE/BE cập nhật
- docs nghiệp vụ cập nhật

### Acceptance

- không còn field naming mâu thuẫn giữa UI, API, DB
- payload create proposal phản ánh đúng business meaning
- QA có thể xác minh giá trị lưu đúng mà không cần suy đoán

---

## P0.2 Tách `title` / `description` khỏi `change_note`

### Vấn đề

Hiện FE đang encode:

- dòng đầu `change_note` = `title`
- phần còn lại = `description`

Đây là workaround, không phải schema sạch.

### Mục tiêu

Cho proposal có contract first-class:

- `title`
- `description`
- `change_note` chỉ còn dùng đúng nghĩa nếu vẫn cần

### Deliverables

- migration DB nếu cần
- DTO backend rõ ràng
- FE mapper bỏ logic split/join `change_note`
- docs business và API cập nhật

### Acceptance

- proposal create/get/list không cần suy diễn `title`/`description`
- report/export/integration có thể dùng field rõ nghĩa

---

## P0.3 Review and harden final approval consistency

### Vấn đề

`admin-approve` có side effect downstream:

- tạo disclosure record
- submit record
- tạo workflow instance

Nếu fail giữa chừng có nguy cơ inconsistent state / duplicate downstream records.

### Mục tiêu

Chốt strategy nhất quán cho:

- reservation
- progress persistence
- replay
- duplicate prevention

### Khuyến nghị

Review theo 2 hướng:

1. persist progress sớm hơn trước khi gọi downstream
2. hoặc đảm bảo downstream create cũng có natural idempotency key / dedupe key

### Deliverables

- sequence diagram ngắn cho admin approve
- test case cho retry/failure giữa chừng
- fix implementation nếu xác nhận bug

### Acceptance

- retry không tạo thêm disclosure record/workflow ngoài dự kiến
- trạng thái proposal phản ánh đúng downstream progress

---

## P0.4 Fix/clarify actor attribution when creating downstream disclosure record

### Vấn đề

Adapter hiện truyền `createdByMembershipID` vào cả `UserID` và `MembershipID`.

Điều này có thể làm sai:

- audit identity
- trường `created_by`
- ownership downstream

### Mục tiêu

Chốt rõ:

- ai là actor business của disclosure record được sinh ra
- user id nào, membership id nào phải được ghi

### Deliverables

- review adapter `internal/adhoc/infra/disclosure/record_creator.go`
- fix nếu attribution sai
- regression test cho created actor fields

### Acceptance

- disclosure record có actor attribution nhất quán với domain decision

---

## P0.5 Add backend validation parity for create proposal

### Vấn đề

FE có validate:

- type bắt buộc
- title bắt buộc
- process controller bắt buộc
- deadline days > 0

Nhưng backend chưa validate đủ cùng semantics.

### Mục tiêu

Không để direct API call tạo proposal invalid mà UI bình thường sẽ chặn.

### Deliverables

- backend validation matrix cho create
- tests cho invalid payload

### Acceptance

- API trả 4xx nhất quán cho payload thiếu/invalid
- FE và BE không lệch validation expectation

---

## P1 — Product Completeness / UX Alignment

## P1.1 Add real draft editing

### Vấn đề

Hiện có “Lưu nháp” nhưng chưa có:

- API update draft
- UI edit draft
- reopen draft form

### Mục tiêu

Biến “nháp” thành khái niệm usable thật sự.

### Deliverables

- `GET detail draft` -> hydrate form
- `PATCH/PUT draft`
- edit CTA từ detail draft
- state guard: chỉ draft mới được sửa

### Acceptance

- user có thể tạo draft, rời màn hình, quay lại sửa và lưu lại

---

## P1.2 Gate entry CTA by propose permission

### Vấn đề

Nút `Tạo cảnh báo bất thường` ở disclosure type detail hiện dựa vào loại `irregular`, chưa gate rõ bằng `ad_hoc_alert.propose`.

### Mục tiêu

Tránh UX mismatch “thấy nút nhưng không vào được”.

### Deliverables

- FE gating ở `DisclosureTypeDetail`
- regression test tương ứng

### Acceptance

- user không có `ad_hoc_alert.propose` không thấy CTA tạo proposal

---

## P1.3 Show process controller display info in detail/list

### Vấn đề

UI hiện hiển thị `process_controller_id`, không thân thiện với BA/demo/user.

### Mục tiêu

Hiển thị:

- họ tên
- email
- có thể thêm vai trò nếu cần

### Deliverables

- response hoặc FE enrichment cho process controller display
- cập nhật detail UI
- nếu cần, thêm list column

### Acceptance

- người dùng hiểu ai là người kiểm soát quy trình mà không cần membership id

---

## P1.4 Clean deprecated contract artifacts

### Vấn đề

Hiện còn artifact:

- `ad_hoc_alert.admin_review` trong FE types/tests
- `final_steps` type nhưng backend không dùng
- `comment` fields chưa có business meaning rõ

### Mục tiêu

Làm contract gọn và ít gây hiểu nhầm.

### Deliverables

- danh sách artifact cần giữ vì backward compatibility
- danh sách artifact có thể remove
- cleanup plan additive/safe

### Acceptance

- docs, FE types, API contract nhất quán hơn

---

## P1.5 Review audit and notification coverage for proposal stages

### Vấn đề

Proposal stage hiện chưa có audit/notification story đủ rõ.

### Mục tiêu

Chốt tối thiểu:

- có cần audit tập trung cho create/submit/approve/reject/cancel không
- có cần notify focal/controller/proposer ở từng state change không

### Deliverables

- decision note
- implementation plan riêng nếu được duyệt

### Acceptance

- BA/ops biết chính xác event nào được log, event nào được gửi thông báo

---

## P2 — Documentation / BA / Long-term Improvements

## P2.1 Refresh business docs in frontend repo

### Mục tiêu

Đồng bộ:

- `docs/canh-bao-bat-thuong-feature-doc.md`
- `docs/permission_catalog.md`

với runtime thật mới nhất.

### Acceptance

- docs không còn hứa editable draft/full CRUD nếu code chưa có
- docs phản ánh gate identity-based final approval

---

## P2.2 Refresh API contract doc for effective-workflow permission boundary

### Mục tiêu

Cập nhật `docs/api-contracts-json.md` để không còn ghi rule cũ rằng `effective-workflow` cần `template.workflow.override.read`.

### Acceptance

- doc khớp runtime hiện tại

---

## P2.3 Consider splitting “proposal” and “final alert” language across product surfaces

### Mục tiêu

Tránh nhầm giữa:

- proposal để xin duyệt
- disclosure/workflow thật sau approved

### Gợi ý

- dùng nhãn “Đề xuất cảnh báo đột xuất” cho module hiện tại
- chỉ gọi “cảnh báo/công bố” cho entity downstream nếu có màn riêng

---

## Execution Order Khuyến Nghị

1. `P0.1` contract deadline semantics
2. `P0.2` title/description contract cleanup
3. `P0.3` admin-approve consistency review/fix
4. `P0.4` attribution fix
5. `P0.5` backend validation parity
6. `P1.2` CTA permission gating
7. `P1.1` real draft editing
8. `P1.3` process controller display enrichment
9. `P1.4` deprecated artifact cleanup
10. `P1.5` audit/notification decision
11. `P2.*` documentation cleanup

---

## Suggested Ownership Split

### Backend-heavy

- `P0.1`
- `P0.2`
- `P0.3`
- `P0.4`
- `P0.5`
- `P1.5`

### Frontend-heavy

- `P1.1`
- `P1.2`
- `P1.3`
- `P1.4`

### Cross-repo / PM-BA-Engineering alignment

- `P2.1`
- `P2.2`
- `P2.3`

---

## Done Definition Cho Đợt Ổn Định Tối Thiểu

Có thể coi flow này đạt mức “ổn định để truyền thông nội bộ / demo rộng hơn” khi:

- contract proposal không còn mâu thuẫn rõ nghĩa
- final approval không còn rủi ro duplicate/inconsistent nghiêm trọng
- docs business không còn overclaim
- CTA/permission UX khớp runtime
- draft semantics được chốt rõ: hoặc thật sự editable, hoặc docs nói rõ là không
