# Deadline Alerts Tab Plan Review Summary

## Task Type
- understand / plan / review

## Objective
- ghi lại solution direction, implementation plan dạng ticket-ready, và các conflict high cần làm rõ cho yêu cầu điều chỉnh tab `Cảnh báo thời hạn`

## Scope
- FE repo chính bị ảnh hưởng: `cobo_web_design`
- BE repo liên quan contract/timeline workflow: `cobo_iam_services`
- mục tiêu: đúng scope, không ảnh hưởng `History`, `DeadlineDetail`, workflow action flows, menu/route guard

## Current State Recheck

### FE route / UI
- route `/app/deadlines` render `DeadlineList`
- screen này có 2 tab nội bộ:
  - `Cảnh báo thời hạn`
  - `Lịch sử CBTT`
- sidebar label là `Cảnh báo về thời hạn`

### Current alert-card behavior
- dữ liệu đang lấy từ `mockDeadlines`
- mỗi card hiện render:
  - status chip
  - title
  - description
  - due date
  - owner (`Người phụ trách`)
  - CTA `Chi tiết`
  - CTA `Tạo cảnh báo mới` nếu đủ quyền và chưa `Done`

### Current FE model gap
- `DeadlineAlert` hiện chỉ có:
  - `id`
  - `disclosureTypeId`
  - `title`
  - `description`
  - `dueDate`
  - `status`
  - `owner`
  - `linkedDisclosureId?`
- chưa có:
  - `workflowInstanceId`
  - `activeDepartments`
  - timeline-derived state

### Current BE workflow contract
- workflow instance contract hiện có:
  - `current_step_code`
  - `snapshot[]`
  - `t0_date`
  - `t0_policy`
- `snapshot` step hiện có:
  - `step_code`
  - `stage`
  - `department`
  - `processing_days`
- đây là nền tảng tốt để derive `phòng đang thực hiện`, nhưng chưa nối vào alert card hiện tại

## Requirement Understanding

### 4.1 CTA changes
- bỏ nút `Tạo cảnh báo mới` ở từng item cảnh báo
- đổi `Chi tiết` thành `Chi tiết cảnh báo`

### 4.2 Alert card content
- mỗi item cần hiển thị:
  - `Tiêu đề cảnh báo`
  - `Thời hạn cảnh báo`
  - `Phòng đang thực hiện`

### 4.2 active department logic
- dựa trên `T0 + ngày`
- ví dụ đang ở ngày thuộc bước 2 thì hiển thị phòng của bước 2
- nếu cùng thời điểm có nhiều phòng cùng thực hiện thì hiển thị tất cả

## Recommended Architecture Decision

- không dùng `owner` hiện tại làm source of truth cho `phòng đang thực hiện`
- phase 1 chỉ thay FE card rendering + adapter boundary
- phase 2 nối dữ liệu workflow-backed theo hướng additive
- source of truth khuyến nghị:
  - backend workflow state / snapshot
  - FE chỉ map hiển thị
- không để FE tự suy workflow state độc lập nếu backend đã có `current_step_code` hoặc active-step semantics

## Ticket-Ready Plan

### Phase 1 - FE only

#### Ticket 1 - Remove obsolete item CTA and rename detail CTA
- description:
  - bỏ CTA `Tạo cảnh báo mới` ở từng alert item
  - đổi CTA `Chi tiết` thành `Chi tiết cảnh báo`
- acceptance criteria:
  - item trong tab `Cảnh báo thời hạn` không còn nút `Tạo cảnh báo mới`
  - item trong tab `Cảnh báo thời hạn` có nút `Chi tiết cảnh báo`
  - tab `Lịch sử CBTT` không đổi behavior
- test scope:
  - manual check `/app/deadlines`
  - manual check `/app/deadlines?tab=history`
  - FE build

#### Ticket 2 - Replace alert-card metadata with business fields
- description:
  - card chỉ còn render:
    - `Tiêu đề cảnh báo`
    - `Thời hạn cảnh báo`
    - `Phòng đang thực hiện`
- acceptance criteria:
  - `owner` không còn render trực tiếp dưới label `Người phụ trách`
  - không ảnh hưởng filters / status chips / route behavior
- test scope:
  - manual check tất cả trạng thái card hiện có
  - FE build

#### Ticket 3 - Introduce alert-card view model boundary
- description:
  - tạo mapper / view-model riêng cho alert cards
  - thêm `activeDepartments: string[]`
- acceptance criteria:
  - `DeadlineList` không bind trực tiếp vào `owner` để hiển thị business field mới
  - tab `History` và `DeadlineDetail` không bị phụ thuộc vào mapper mới
- test scope:
  - unit test mapper
  - FE build

#### Ticket 4 - Mock-safe department derivation
- description:
  - phase 1 derive `activeDepartments` từ mock workflow / disclosure type data hiện có
- acceptance criteria:
  - 1 active department hiển thị đúng
  - nhiều department cùng active hiển thị đủ
  - thiếu workflow data không crash UI và có fallback an toàn
- test scope:
  - mapper unit tests:
    - single department
    - multiple departments
    - missing workflow
  - manual check `/app/deadlines`
  - FE build

### Phase 2 - FE + BE additive

#### Ticket 5 - Define workflow-backed alert-card contract
- description:
  - chốt contract hiển thị cho alert card từ workflow thật
- acceptance criteria:
  - có mapping rõ cho:
    - `title`
    - `dueDate`
    - `workflowInstanceId`
    - `activeDepartments[]`
  - chốt source of truth cho active department
- test scope:
  - contract review / doc review

#### Ticket 6 - Extend BE contract only if current contract is insufficient
- description:
  - nếu `current_step_code` singular không đủ cho business multi-active semantics thì mở rộng additive
- acceptance criteria:
  - endpoint cũ vẫn backward compatible
  - workflow action flows không đổi semantics
  - FE có thể resolve `activeDepartments[]` đúng
- test scope:
  - Go tests cho workflow service/handler
  - Docker build `api`

#### Ticket 7 - Add FE workflow fetch + mapper
- description:
  - FE fetch workflow instance theo `workflowInstanceId`
  - map `snapshot/current step` -> `activeDepartments[]`
- acceptance criteria:
  - fallback an toàn khi thiếu instance hoặc snapshot
  - `History` tab không phụ thuộc data path mới
- test scope:
  - FE service contract tests
  - mapper unit tests
  - FE build

#### Ticket 8 - Wire real workflow-backed active departments into deadline tab
- description:
  - thay mock-safe derivation bằng real workflow-backed derivation
- acceptance criteria:
  - card hiển thị đúng `Phòng đang thực hiện`
  - hỗ trợ nhiều phòng theo contract đã chốt
  - không regression ở filters / routes / history
- test scope:
  - manual regression
  - FE build
  - nếu BE change: Docker build `api`

## High Conflicts Requiring Clarification

### H1 - Alert to workflow-instance join is undefined
- blocker:
  - `DeadlineAlert` hiện không có `workflowInstanceId`
- cần làm rõ:
  - alert item sẽ nối tới workflow instance bằng field nào trong live data flow

### H2 - Multi-department semantics is not explicit in current BE contract
- blocker:
  - `current_step_code` hiện là singular
- cần làm rõ:
  - có thật sự nhiều active steps cùng lúc không
  - hay chỉ một active step nhưng step đó có nhiều department

### H3 - T0-driven wording vs backend-state-driven architecture
- conflict:
  - business wording nói `dựa trên T0 + ngày`
  - khuyến nghị kỹ thuật là backend state phải là source of truth
- cần làm rõ:
  - FE có được phép chỉ hiển thị theo backend active step không
  - hay phải tự tính timeline từ `T0 + processing_days`

### H4 - Header CTA scope is ambiguous
- conflict:
  - requirement nói bỏ nút ở từng item
  - screen hiện còn có 1 CTA header `Tạo cảnh báo mới` cho tab `Deadlines`
- cần làm rõ:
  - giữ hay bỏ CTA header

## Decision Recommendation
- proceed ngay với Phase 1 sau khi xác nhận H4
- không bắt đầu Phase 2 cho tới khi chốt H1 + H2 + H3
- nếu cần giữ blast radius thấp nhất:
  - implement only item-level UI change first
  - tách adapter boundary sẵn để không phải refactor lại nhiều ở phase 2

## Affected Files Likely

### FE
- `cobo_web_design/src/pages/portal/DeadlineList.tsx`
- `cobo_web_design/src/types.ts`
- `cobo_web_design/src/mockData.ts`
- `cobo_web_design/src/services/*` (new mapper / adapter likely)

### BE (phase 2 only if needed)
- `cobo_iam_services/internal/workflow/app/contracts.go`
- `cobo_iam_services/internal/workflow/app/service.go`
- `cobo_iam_services/internal/workflow/transport/http/handler.go`

## Verification Result
- documentation-only task
- no runtime code changes
- docker/build verification not required

## Next Steps
- confirm 4 high-conflict questions
- once confirmed:
  - execute Phase 1 FE-only safely
  - re-open Phase 2 only if workflow-backed integration is approved
