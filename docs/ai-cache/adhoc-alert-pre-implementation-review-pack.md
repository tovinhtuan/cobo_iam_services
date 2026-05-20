# Ad-Hoc Alert Pre-Implementation Review Pack

## Mục tiêu

Đây là bộ câu hỏi bắt buộc nên chốt trong buổi review trước khi cho team triển khai tiếp luồng **Đề xuất Cảnh báo Bất thường**.

Nếu chưa chốt hết các câu hỏi này, rất dễ:

- code sai contract
- fix nhầm symptom thay vì root issue
- phải refactor lại giữa chừng
- BA/PM và dev hiểu khác nhau về cùng một flow

---

## 1. Proposal deadline đang mang nghĩa gì: số ngày hay ngày tuyệt đối?

### Cần chốt

- field này là:
  - `số ngày deadline đề xuất`
  - hay `ngày deadline cụ thể`
Answer: Sẽ có 2 type (số ngày/ngày chính xác)
     - Nếu type là số ngày: số ngày deadline đề xuất, tính từ ngày tạo cảnh báo
     - Nếu type là ngày chính xác: Ngày chính xác deadline
### Vì sao phải chốt

Hiện FE, API và DB đang hiểu khác nhau.

### Quyết định mong muốn

- canonical business meaning
- canonical field name
- canonical data type

---

## 2. Proposal có cần là một entity editable thực sự ở trạng thái draft không?

### Cần chốt

- draft chỉ là “tạo nháp rồi submit sớm”
- hay user thật sự phải có:
  - mở lại draft
  - sửa draft
  - lưu draft nhiều lần

### Vì sao phải chốt

Điều này ảnh hưởng trực tiếp tới:

- có cần API update draft không
- có cần màn edit draft không
- BA có được mô tả “save draft để làm tiếp sau” hay không

---

## 3. Title và description của proposal có phải business field chính thức không?

### Cần chốt

- proposal có field chính thức:
  - `title`
  - `description`
- hay team chấp nhận tiếp tục encode vào `change_note`

### Vì sao phải chốt

Nếu coi đây là field chính thức, backend contract phải nâng cấp thành first-class.
Nếu không chốt, FE và BE sẽ tiếp tục lệch semantics.

---

## 4. Ai là “actor business” khi proposal được duyệt cuối và sinh disclosure record?

### Cần chốt

Disclosure record downstream được xem là do ai tạo:

- người đề xuất
- người duyệt cuối
- hay hệ thống tạo thay mặt proposal

### Vì sao phải chốt

Điều này quyết định:

- audit
- ownership
- reporting
- field `created_by` / actor attribution downstream

---

## 5. Mức đảm bảo đúng dữ liệu cho `admin-approve` cần đến đâu?

### Cần chốt

Team có coi đây là critical flow cần:

- strict idempotency
- duplicate-proof downstream creation
- recoverable mid-flight failure

hay chỉ cần “best effort” ở giai đoạn hiện tại.

### Vì sao phải chốt

Nếu coi đây là critical flow, `P0.3` phải được làm như correctness hardening trước khi mở rộng UX.

---

## 6. Vòng duyệt cuối có phải luôn là identity-based theo process controller không?

### Cần chốt

- final approver luôn là đúng người được chỉ định
- hay vẫn cần fallback cho một nhóm admin đặc biệt

### Vì sao phải chốt

Hiện code đang đi theo:

- identity-based final approval

Nếu business muốn fallback admin override, contract và UI sẽ phải đổi.

---

## 7. Product muốn truyền thông tính năng này là “proposal workflow” hay “full alert CRUD”?

### Cần chốt

Ngôn ngữ chính thức dùng cho:

- tài liệu BA
- tài liệu marketing nội bộ
- release note
- demo script

### Vì sao phải chốt

Current runtime phù hợp với:

- `proposal workflow`

chưa phù hợp để gọi là:

- `full CRUD cảnh báo bất thường`

---

## Recommendation: thứ tự chốt trong buổi review

1. Câu 1 — deadline semantics
2. Câu 2 — draft semantics
3. Câu 3 — title/description contract
4. Câu 4 — downstream actor attribution
5. Câu 5 — admin-approve correctness target
6. Câu 6 — final approval business rule
7. Câu 7 — product/business wording

---

## Output mong muốn sau buổi review

Buổi review chỉ nên coi là “đủ để triển khai” khi chốt được:

- 1 quyết định semantics cho deadline
- 1 quyết định semantics cho draft
- 1 quyết định semantics cho title/description
- 1 quyết định về actor downstream
- 1 quyết định về mức hardening cho final approval
- 1 quyết định về final approval rule
- 1 wording chính thức cho BA/marketing

Nếu muốn gọn hơn, có thể chốt bằng một bảng 7 dòng:

| Question | Decision | Owner | Follow-up ticket |
|---|---|---|---|
| Deadline semantics |  |  |  |
| Draft semantics |  |  |  |
| Title/description contract |  |  |  |
| Downstream actor |  |  |  |
| Final approval hardening level |  |  |  |
| Final approval rule |  |  |  |
| Product wording |  |  |  |
