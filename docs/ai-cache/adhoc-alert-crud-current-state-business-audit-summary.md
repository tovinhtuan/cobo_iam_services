# Ad-Hoc Alert CRUD Current-State Review

## Mục đích

Tài liệu này tổng hợp **logic thực tế đang có trong code mới nhất** cho luồng **Đề xuất Cảnh báo Bất thường** (`ad-hoc alert proposal`), theo góc nhìn:

- Business / BA / Marketing: hệ thống đang làm được gì, nên mô tả tính năng ra sao
- Product / QA: những phần nào đã có thật, phần nào chưa có hoặc mới ở mức partial
- Engineering: contract, issue, conflict, rủi ro tồn đọng

Phạm vi tài liệu này là **ad-hoc alert proposal flow**, không phải tab `Cảnh báo thời hạn` trong `/app/deadlines`.

---

## 1. Executive Summary

### Đang có thật trong sản phẩm

Hệ thống hiện đã có một luồng **đề xuất cảnh báo bất thường** theo dạng workflow:

1. Người dùng tạo một **đề xuất**
2. Đề xuất được **gửi duyệt**
3. Focal Point duyệt hoặc từ chối
4. Người kiểm soát quy trình duyệt hoặc từ chối
5. Khi được duyệt cuối, hệ thống tự động:
   - tạo `disclosure record`
   - tạo `workflow instance`

### Điều quan trọng cần hiểu đúng

Đây **chưa phải** là “CRUD đầy đủ cho một cảnh báo cuối cùng”.

Hiện tại hệ thống đang làm **CRUD + workflow action cho proposal**, tức là:

- tạo đề xuất
- xem danh sách / chi tiết đề xuất
- submit / approve / reject / cancel đề xuất
- sau khi duyệt xong mới sinh ra record công bố thật

Vì vậy nếu BA/Marketing mô tả tính năng, nên gọi đúng là:

**“Quy trình đề xuất và phê duyệt cảnh báo bất thường”**

thay vì:

**“Quản lý CRUD cảnh báo bất thường hoàn chỉnh”**

---

## 2. Scope Thực Tế Đang Có

### Frontend screens

- Danh sách đề xuất: `/app/ad-hoc-proposals`
- Tạo đề xuất mới: `/app/ad-hoc-proposals/new`
- Chi tiết đề xuất: `/app/ad-hoc-proposals/:proposalId`
- Entry point từ chi tiết loại CBTT bất thường: `/app/disclosure-types/:id`

### Backend endpoints thực tế

- `GET /api/v1/company/ad-hoc-proposals`
- `POST /api/v1/company/ad-hoc-proposals`
- `GET /api/v1/company/ad-hoc-proposals/{proposal_id}`
- `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/submit`
- `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/focal-approve`
- `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/admin-approve`
- `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/reject`
- `POST /api/v1/company/ad-hoc-proposals/{proposal_id}/cancel`
- `GET /api/v1/company/ad-hoc-proposals/eligible-controllers`

### Feature flags / prerequisite

- Module chỉ bật khi `WORKFLOW_ADHOC_ENABLED=true`
- Local dev default hiện tại là enabled
- Có flag `WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED` để bỏ qua vòng focal

---

## 3. Business Story Có Thể Dùng Cho BA / Marketing

### Câu mô tả đúng với current state

“Người dùng có thể tạo một đề xuất cảnh báo bất thường cho loại CBTT bất thường, chỉ định người kiểm soát quy trình, gửi đề xuất qua các vòng duyệt nội bộ, và khi được duyệt cuối thì hệ thống tự động tạo hồ sơ công bố cùng workflow thực thi.”

### Giá trị business hiện tại

- Chuẩn hóa việc xử lý các tình huống công bố đột xuất
- Có phân vai rõ giữa:
  - người đề xuất
  - người duyệt focal
  - người kiểm soát quy trình
- Có lưu lịch sử trạng thái ngay trên proposal
- Có liên kết từ proposal sang disclosure record / workflow instance sau khi duyệt

### Không nên hứa ở tài liệu business lúc này

- “Người dùng có thể sửa nháp đã tạo bất kỳ lúc nào”
- “Có CRUD đầy đủ cho alert cuối cùng”
- “Có đầy đủ thông báo / nhắc việc cho từng bước ad-hoc proposal”
- “Admin role mặc định là người duyệt cuối”

Các câu trên **không phản ánh đúng code hiện tại**.

---

## 4. Actors Và Quyền Nghiệp Vụ

### Vai trò thực tế trong code

- `ad_hoc_alert.propose`
  - tạo proposal
  - submit proposal
  - cancel proposal
  - xem danh sách eligible process controllers
- `ad_hoc_alert.read`
  - xem danh sách proposal
  - xem chi tiết proposal
- `ad_hoc_alert.focal_review`
  - duyệt / từ chối ở trạng thái `pending_focal_approval`
- `ad_hoc_alert.process_control`
  - **không phải** quyền để bấm approve trực tiếp
  - đây là quyền để một thành viên **được phép được chỉ định** làm process controller

### Gate quan trọng nhất

Vòng duyệt cuối không còn dựa trên `ad_hoc_alert.admin_review` làm gate chính.

Gate thật hiện tại là:

- proposal phải đang ở `pending_admin_approval`
- `process_controller_id` của proposal phải **đúng bằng** `membership_id` hiện tại

Nói ngắn gọn:

**“Người duyệt cuối là người được creator chỉ định, không phải bất kỳ ai có role admin.”**

### Permission deprecated

- `ad_hoc_alert.admin_review` vẫn còn tồn tại trong DB / FE types để tương thích
- nhưng không còn là business gate chính cho `admin-approve`

---

## 5. State Machine Thực Tế

### Trạng thái đang có

- `ad_hoc_draft`
- `pending_focal_approval`
- `pending_admin_approval`
- `approved`
- `rejected`
- `cancelled`

### Luồng chuyển trạng thái

1. `POST create` -> `ad_hoc_draft`
2. `submit`:
   - bình thường -> `pending_focal_approval`
   - nếu auto-approve bật -> `pending_admin_approval`
3. `focal-approve` -> `pending_admin_approval`
4. `admin-approve` -> `approved`
5. `reject`:
   - từ focal stage -> `rejected`
   - từ admin stage -> `rejected`
6. `cancel`:
   - từ draft / pending_focal / pending_admin -> `cancelled`

### Terminal states

- `approved`
- `rejected`
- `cancelled`

Không có logic reopen / resubmit / edit after terminal state.

---

## 6. CRUD Matrix Thực Tế

| Capability | Có thật? | Ghi chú |
|---|---|---|
| Create proposal | Có | `POST /company/ad-hoc-proposals` |
| Read list | Có | Có filter trạng thái + paging |
| Read detail | Có | Hiển thị proposal + approval history |
| Update draft | **Chưa có** | Không có `PUT/PATCH` cho proposal draft |
| Delete proposal hard-delete | **Chưa có** | Chỉ có `cancel` là soft terminal state |
| Submit proposal | Có | Action riêng |
| Focal approve | Có | Action riêng |
| Final approve | Có | Action riêng + sinh disclosure/workflow |
| Reject | Có | Action riêng |
| Withdraw / cancel | Có | Creator-only |

### Kết luận product

Current state là:

- **Proposal CRUD một phần**
- cộng với **workflow actions**
- chưa phải generic CRUD hoàn chỉnh cho entity “alert”

---

## 7. Dữ Liệu Và Contract Thực Tế

### Proposal data thực sự được lưu ở backend

Các field chính trong `ad_hoc_proposals`:

- `proposal_id`
- `company_id`
- `type_id`
- `status`
- `proposed_workflow_json`
- `proposed_t0_date`
- `proposed_deadline_date`
- `change_note`
- `process_controller_id`
- `created_by`
- metadata approve/reject/final fields
- `record_id`
- `workflow_instance_id`

### FE contract hiện đang dùng

FE đang hiển thị proposal bằng các field business-friendly:

- `title`
- `description`
- `proposed_t0`
- `proposed_deadline_days`
- `process_controller_id`
- `proposed_steps`
- `final_t0`
- `final_deadline_date`

### Business contract thực tế giữa FE và BE

Đây là điểm rất quan trọng:

- backend **không có field riêng** `title` và `description` cho proposal
- FE đang gói `title + description` vào `change_note`
- FE khi đọc proposal sẽ:
  - lấy dòng đầu của `change_note` làm `title`
  - lấy phần còn lại làm `description`

Nghĩa là:

**title/description hiện là contract suy diễn ở FE, không phải contract 1:1 từ backend schema.**

### Step override contract thực tế

Hiện proposal chỉ hỗ trợ override:

- `step_id`
- `processing_days`

Không hỗ trợ:

- thêm/xóa step
- đổi assignee
- đổi document requirement
- đổi group/department binding

---

## 8. Luồng Approval Cuối Và Tạo Record Thật

Khi `admin-approve` thành công:

1. Proposal được check identity với `process_controller_id`
2. Hệ thống tạo `disclosure record`
3. Hệ thống submit record đó
4. Nếu workflow bật, hệ thống tạo `workflow instance`
5. Proposal được cập nhật:
   - `status = approved`
   - `record_id`
   - `workflow_instance_id`
   - `final_t0_date`
   - `final_deadline_date`
   - `adjustment_note`

### Idempotency đã có

`admin-approve` đã có lớp chống retry trùng:

- idempotency key
- reservation
- replay response
- progress fields trong proposal table

Đây là phần khá tốt của implementation hiện tại.

---

## 9. Phần Đã Có Logic Thực Tế

### Đã có ở mức production-like

- State machine proposal
- Permission checks backend cho create/read/submit/focal/reject/cancel
- Identity-based final approval
- Eligible controller lookup theo permission
- Paging + filter cho list
- Idempotency cho final approval
- Tự động sinh disclosure record + workflow instance khi approved
- FE screens cho list / create / detail
- FE gating cho route create/list/detail

### Đã có nhưng còn “contract debt”

- FE title/description <-> BE change_note mapping
- FE proposed_deadline_days <-> BE proposed_deadline_date mapping
- FE process controller hiển thị bằng `membership_id`, chưa có display object đầy đủ ở detail

---

## 10. Phần Chưa Có Hoặc Chưa Hoàn Chỉnh

### Chưa có update draft thật sự

- UI có khái niệm “Lưu nháp”
- nhưng sau khi draft đã tạo xong, không có API để sửa draft đó
- không có màn edit draft

Kết luận:

**“Lưu nháp” hiện là tạo mới một proposal ở trạng thái draft, không phải save-progress/edit-later đầy đủ.**

### Chưa có hard delete

- proposal không bị xóa vật lý
- chỉ có `cancelled`

### Chưa có reopen / resubmit rejected

- proposal bị reject phải tạo lại proposal mới

### Chưa có dedicated notification flow cho ad-hoc proposal stages

Trong code hiện không thấy logic riêng để:

- gửi thông báo cho focal khi có proposal mới
- gửi thông báo cho process controller khi focal approve
- gửi thông báo trạng thái proposal cho proposer

Sau khi approved, workflow/disclosure downstream có thể có logic riêng, nhưng proposal stage chưa thấy wiring rõ.

### Chưa có dedicated audit service integration cho module ad-hoc

Khác với một số module khác, handler ad-hoc hiện không append audit log tập trung.

Lịch sử hiện tại chủ yếu dựa trên:

- `created_at`
- `focal_approved_by/at`
- `admin_approved_by/at`
- `rejected_by/at`

---

## 11. Issue Và Conflict Tồn Đọng

## 11.1 High: “Save draft để sửa tiếp” chưa đúng thực tế

Business wording hiện dễ khiến người dùng hiểu rằng draft có thể sửa lại sau.

Code thực tế:

- create draft: có
- open detail draft: có
- edit draft existing: không có
- update draft API: không có

Ảnh hưởng:

- BA/Marketing nếu mô tả sai sẽ tạo expectation gap

---

## 11.2 High: Contract title/description đang là workaround, không phải schema thật

BE không lưu riêng:

- `title`
- `description`

FE ghép 2 field này vào `change_note`, rồi tách ngược khi đọc.

Ảnh hưởng:

- contract không sạch
- search/reporting/export sau này khó ổn định
- integrations dễ hiểu sai field semantics

---

## 11.3 High: `proposed_deadline_days` ở FE lệch nghĩa với `proposed_deadline_date` ở BE

FE:

- user nhập **số ngày**
- field UI là `proposed_deadline_days`

BE schema:

- cột DB là `proposed_deadline_date DATE`

FE request mapping hiện gửi số ngày vào field wire tên `proposed_deadline_date`.

Đây là một conflict contract nghiêm trọng:

- tên field backend mang nghĩa “ngày”
- dữ liệu frontend đang mang nghĩa “số ngày”

Tối thiểu đây là technical debt rất lớn.
Ở tình huống xấu hơn, nó có thể tạo dữ liệu sai hoặc khó kiểm soát khi đổi môi trường / SQL mode.

---

## 11.4 High: Backend không validate business input đủ chặt cho create proposal

Backend create hiện validate mạnh ở:

- permission
- template category
- process controller
- step override id / processing_days >= 0

Nhưng **không validate business-level** cho:

- title bắt buộc
- description format
- proposed T0 format rõ ràng
- proposed deadline semantics rõ ràng

Nghĩa là:

Nếu gọi API trực tiếp, có thể tạo proposal không phản ánh đúng validation FE đang áp.

---

## 11.5 High: Final approval side effect có nguy cơ partial-failure tạo record không đồng bộ

`admin-approve` hiện:

1. gọi `CreateAndSubmitRecord`
2. sau khi thành công mới save progress vào proposal

Vấn đề:

- nếu lỗi xảy ra **sau khi record đã được tạo** nhưng **trước khi progress được persist đủ**
- retry có thể tạo thêm downstream record/workflow lần nữa

Điểm này cần engineering review riêng vì liên quan data consistency khi final approval gặp lỗi giữa chừng.

---

## 11.6 High: Attribution của disclosure record sau final approval đang đáng nghi

Adapter tạo disclosure record hiện truyền:

- `createdByMembershipID` vào cả `UserID` và `MembershipID`

Điều này làm phát sinh rủi ro:

- `created_by` ở disclosure record có thể lưu membership id thay vì user id
- chủ thể tạo record downstream có thể bị ghi nhận sai

Ngoài ra final record hiện được tạo theo actor ở bước approve cuối, không phải rõ ràng theo proposer ban đầu.

Đây là issue dữ liệu/nghiệp vụ cần làm rõ.

---

## 11.7 Medium: CTA “Tạo cảnh báo bất thường” ở disclosure type detail chưa gate theo quyền propose

Hiện link CTA xuất hiện theo `templateCategory === irregular`, không check rõ `ad_hoc_alert.propose`.

Ảnh hưởng:

- user có quyền xem loại CBTT nhưng không có quyền propose vẫn có thể thấy nút
- bấm vào sau đó mới bị chặn ở route/API

Đây là UX mismatch.

---

## 11.8 Medium: Detail UI hiển thị process controller bằng ID, không phải tên người

Detail page hiện show:

- `process_controller_id`

chứ chưa show object kiểu:

- họ tên
- email
- vai trò

Điều này làm tài liệu business/demo kém thân thiện.

---

## 11.9 Medium: FE/API vẫn còn artifact của contract cũ / deprecated

Ví dụ:

- `ad_hoc_alert.admin_review` vẫn còn trong FE types/tests
- `comment` được khai báo ở nhiều action nhưng chưa được business flow tận dụng rõ
- `final_steps` có type ở FE nhưng backend approve không dùng

Đây không phải bug blocking ngay, nhưng là tín hiệu contract chưa được dọn sạch.

---

## 11.10 Medium: Tài liệu hiện có trong repo đang conflict với runtime thực tế

### Conflict A

`../cobo_web_design/docs/canh-bao-bat-thuong-feature-doc.md` mô tả trạng thái tương đối “ready/demoable”, nhưng không nhấn mạnh đủ các gap hiện tại như:

- không có edit draft
- contract title/description là suy diễn
- final approval attribution / partial-failure risk

### Conflict B

`docs/api-contracts-json.md` vẫn mô tả:

- `template.workflow.override.read`: GET override + GET versions + GET effective workflow

Trong runtime mới nhất, `effective-workflow` đã được nới về boundary disclosure catalog read để không chặn người proposer hợp lệ.

---

## 12. Business Contract Khuyến Nghị Cho BA / Marketing

### Cách gọi tên đúng

Nên dùng:

- “Đề xuất cảnh báo bất thường”
- “Quy trình đề xuất và phê duyệt cảnh báo bất thường”
- “Ad-hoc proposal workflow”

Không nên dùng:

- “CRUD cảnh báo bất thường hoàn chỉnh”
- “Người dùng chỉnh sửa cảnh báo bất thường tự do”
- “Admin cuối luôn là người duyệt”

### Cách mô tả ngắn để demo / tài liệu bán hàng nội bộ

“Người dùng tạo đề xuất công bố bất thường cho một loại CBTT bất thường, chỉ định người kiểm soát quy trình, gửi qua các vòng duyệt nội bộ, và khi được duyệt cuối thì hệ thống tự động khởi tạo hồ sơ công bố cùng workflow để thực thi.”

### Những disclaimer BA nên giữ

- draft hiện mang nghĩa “proposal nháp”, chưa phải editable draft đầy đủ
- proposal approved sẽ sinh ra disclosure/workflow downstream
- final approver là người được chỉ định, không phải mọi admin

---

## 13. Khuyến Nghị Product / Engineering

### Nếu muốn claim “CRUD hoàn chỉnh”

Cần làm thêm tối thiểu:

1. Update draft API + UI edit draft
2. Contract riêng cho `title`, `description`, `proposed_deadline_days`
3. Clean separation giữa proposal entity và final disclosure entity
4. Dọn deprecated contract / permission cũ
5. Chốt notification/audit story

### Nếu chỉ muốn ổn định current flow

Ưu tiên nên làm:

1. Fix contract `proposed_deadline_days` vs `proposed_deadline_date`
2. Fix/confirm attribution khi sinh disclosure record
3. Review partial failure + idempotent replay của `admin-approve`
4. Bổ sung backend validation cho create proposal
5. Gate CTA `Tạo cảnh báo bất thường` theo quyền `ad_hoc_alert.propose`

---

## 14. Source Of Truth Được Rà Soát

### Backend

- `internal/adhoc/app/contracts.go`
- `internal/adhoc/app/service.go`
- `internal/adhoc/infra/mysql/repository.go`
- `internal/adhoc/infra/mysql/membership_validator.go`
- `internal/adhoc/infra/disclosure/record_creator.go`
- `internal/adhoc/transport/http/handler.go`
- `internal/httpserver/server.go`
- `internal/disclosure/app/service.go`
- `migrations/0032_customize_workflow_extension.up.sql`
- `migrations/0033_smoke_workflow_dev_seed.up.sql`
- `migrations/0041_adhoc_admin_approve_progress.up.sql`
- `migrations/0042_adhoc_process_controller.up.sql`

### Frontend

- `../cobo_web_design/src/App.tsx`
- `../cobo_web_design/src/pages/portal/DisclosureTypeDetail.tsx`
- `../cobo_web_design/src/pages/portal/AdHocProposalListPage.tsx`
- `../cobo_web_design/src/pages/portal/AdHocProposalCreatePage.tsx`
- `../cobo_web_design/src/pages/portal/AdHocProposalDetailPage.tsx`
- `../cobo_web_design/src/services/adHocAlertsApi.ts`
- `../cobo_web_design/src/services/disclosureTypesApi.ts`
- `../cobo_web_design/src/services/workflowOverrideMappers.ts`
- `../cobo_web_design/src/types.ts`

### Existing docs reviewed

- `../cobo_web_design/docs/canh-bao-bat-thuong-feature-doc.md`
- `../cobo_web_design/docs/permission_catalog.md`
- `docs/api-contracts-json.md`
- `docs/ai-cache/reusable-task-updates.md`

---

## 15. Final Assessment

Nếu đánh giá trung thực theo code hiện tại:

- **proposal workflow**: đã có khá nhiều logic thật, đủ để demo và test flow chính
- **business contract cleanliness**: còn debt rõ ràng
- **full CRUD claim**: chưa đúng
- **documentation hiện hữu**: cần chỉnh lại để khớp runtime và khớp scope thật

Nói ngắn gọn:

**Tính năng đã có lõi nghiệp vụ chạy được, nhưng vẫn đang là một proposal workflow có một số contract debt và vài issue correctness cần xử lý trước khi coi là “hoàn chỉnh”.**
