# Ad-Hoc Alert P0 Review — Bug vs Tech Debt Classification

## Mục tiêu

Tài liệu này chốt lại từng mục `P0` trong action plan theo 3 loại:

- `Confirmed bug`
- `Tech debt / design debt`
- `Risk / needs confirmation`

Mục tiêu là tránh ghi sai severity vào backlog Jira.

---

## Kết luận nhanh

| Item | Classification | Kết luận ngắn |
|---|---|---|
| `P0.1` deadline contract mismatch | `Confirmed bug` + `contract debt` | Runtime đang dùng semantics lệch nhau |
| `P0.2` title/description from change_note | `Tech debt` | Chưa thấy bug runtime rõ, nhưng contract xấu |
| `P0.3` admin-approve consistency | `Risk / needs confirmation` | Có risk correctness cao, chưa có bằng chứng duplicate đã xảy ra |
| `P0.4` downstream actor attribution | `Likely bug / needs confirmation` | Code hiện rất đáng nghi, cần trace persisted data |
| `P0.5` backend validation parity | `Confirmed robustness bug` | API boundary cho phép behavior FE sẽ chặn |

---

## P0.1 Deadline contract mismatch

### Classification

`Confirmed bug` + `contract debt`

### Vì sao

Đây không còn là chuyện “đặt tên xấu”.

Hiện có mismatch trực tiếp giữa:

- FE label/field meaning: `proposed_deadline_days`
- wire field: `proposed_deadline_date`
- DB column type: `DATE`

Nghĩa là cùng một dữ liệu đang được hiểu theo hai nghĩa khác nhau.

### Evidence

- FE create form thu số ngày
- FE API mapper gửi giá trị đó qua field `proposed_deadline_date`
- DB schema lưu `proposed_deadline_date DATE`

### Tác động

- nguy cơ dữ liệu lưu sai nghĩa
- API contract gây hiểu nhầm cho client khác
- QA/BA không thể xác minh đúng expectation nếu không đọc code

### Ticket severity recommendation

`P0 bug`

---

## P0.2 Title/description encoded into change_note

### Classification

`Tech debt`

### Vì sao

Hiện chưa thấy bug runtime rõ ràng trong UI chính:

- FE create vẫn gửi được
- FE detail/list vẫn render được

Nhưng contract này không sạch:

- backend không có field first-class cho title/description
- FE phải split/join `change_note`

### Evidence

- FE mapper lấy dòng đầu `change_note` làm `title`
- phần còn lại làm `description`

### Tác động

- khó mở rộng
- dễ lỗi ở integration/report/export/search
- semantics không self-documenting

### Ticket severity recommendation

`P0 tech debt`

Lý do để giữ ở P0 là vì nó là contract debt nền tảng, không phải vì nó là crash bug.

---

## P0.3 Admin-approve consistency / duplicate downstream creation

### Classification

`Risk / needs confirmation`

### Vì sao

Đây là risk correctness cao, nhưng từ code review alone chưa thể kết luận chắc chắn là bug đang xảy ra ở runtime.

Đã thấy pattern có thể gây vấn đề:

1. downstream record/workflow được tạo
2. sau đó mới persist progress/complete proposal
3. nếu fail giữa chừng rồi retry, có nguy cơ duplicate

Nhưng để gắn nhãn `confirmed bug`, cần:

- trace transaction boundary rõ hơn
- hoặc reproduce failure scenario

### Evidence

- sequence hiện tại của `admin-approve`
- progress persistence diễn ra sau downstream side effects
- idempotency hiện có nhưng chưa đủ để tự động chứng minh duplicate impossible

### Tác động

- duplicate disclosure record
- duplicate workflow instance
- proposal state và downstream state lệch nhau

### Ticket severity recommendation

`P0 investigation / correctness hardening`

Không nên viết ticket như một bug đã được chứng minh 100%.
Nên viết là:

**“Investigate and harden final-approval consistency”**

---

## P0.4 Actor attribution when creating downstream disclosure record

### Classification

`Likely bug / needs confirmation`

### Vì sao

Code hiện tại truyền `createdByMembershipID` vào cả `UserID` và `MembershipID`.

Nếu downstream thật sự kỳ vọng:

- `UserID` là user id
- `MembershipID` là membership id

thì đây là bug.

Nhưng để khẳng định hoàn toàn, vẫn nên:

- trace persistence cuối cùng của disclosure record
- đối chiếu rule domain: ai phải là actor

### Evidence

- adapter mapping hiện tại rất đáng nghi về type semantics

### Tác động

- audit sai actor
- ownership/downstream reporting sai
- khó đối chiếu người tạo thật

### Ticket severity recommendation

`P0 likely bug`

Nên ghi ticket theo dạng:

**“Verify and fix actor attribution for downstream disclosure record creation”**

---

## P0.5 Backend validation parity for create proposal

### Classification

`Confirmed robustness bug`

### Vì sao

Đây không chỉ là “FE validate nhiều hơn”.

Nếu API boundary chấp nhận payload mà business/UI coi là invalid, backend đang không enforce invariant của chính feature đó.

Với public/internal API, đây là bug ở boundary robustness.

### Evidence

Backend hiện chưa validate đủ các rule mà FE đang áp, ví dụ:

- title bắt buộc
- semantics deadline
- một số business input expectations khác

### Tác động

- direct API call có thể tạo dữ liệu lệch expectation UI
- future clients khác FE hiện tại có thể tạo proposal invalid
- dữ liệu hệ thống không còn được bảo vệ bởi invariant ở backend

### Ticket severity recommendation

`P0 bug`

---

## Backlog Wording Recommendation

### Nên ghi là bug

- `P0.1`
- `P0.5`

### Nên ghi là tech debt

- `P0.2`

### Nên ghi là investigation / hardening

- `P0.3`

### Nên ghi là likely bug, verify-first

- `P0.4`

---

## Recommended Order For P0 Execution

1. `P0.1` fix semantics mismatch
2. `P0.5` enforce backend invariants
3. `P0.4` verify/fix attribution
4. `P0.3` harden consistency + replay
5. `P0.2` clean contract debt

Lý do:

- `P0.1` và `P0.5` trực tiếp ảnh hưởng contract boundary
- `P0.4` và `P0.3` ảnh hưởng correctness/data consistency downstream
- `P0.2` quan trọng nhưng phù hợp xử lý sau khi semantics nền tảng đã ổn định
