# 02: Khoá contract (GATE A của MR-4)

Base path: `/api/v1/company/disclosure-types/{type_id}`. Công ty lấy từ token (`sub.CompanyID`). Không có route mới dưới `/api/v1/platform/...`.

## Endpoint
| Method | Path | Quyền | Thay đổi |
|---|---|---|---|
| GET | `…/workflow-override` | đọc | `draft_version` là version mới nhất còn mở (không `approved`, không `discarded`). `state` thuộc `draft`, `pending_approval`, `rejected`. Thêm `submitted_by`, `submitted_at`, `submit_note`, `rejected_by`, `rejected_at`, `decision_note` (omitempty). `submit_note` là ghi chú khi gửi duyệt; `decision_note` chỉ là lý do từ chối |
| PUT | `…/workflow-override/draft` | ghi; thêm duyệt nếu `publish:true` | `publish:true` kiểm quyền duyệt và validate kích hoạt **trước** khi lưu (403/422 thì không lưu gì), rồi lưu và kích hoạt trong **một transaction**. Response `state=approved`, `self_approved=true` khi publish |
| DELETE | `…/workflow-override/draft/{version_no}` | ghi | Không đổi với client. Dòng version giữ lại với `state=discarded` (không xoá), nên số version không bị dùng lại |
| PUT | `…/workflow-override/draft/steps/{step_id}/groups` | ghi | Ghi thành version **mới** (`v:N+1`) thay vì sửa tại chỗ; trả `draft_etag` mới. 409 `STATE_CONFLICT` khi version mới nhất không phải `draft` (trước đây 404) |
| POST (mới) | `…/workflow-override/submit` | ghi | Body `{version_no, base_etag?, note?}`. `draft` → `pending_approval`. Trả version DTO |
| POST | `…/workflow-override/approve` | duyệt | Nhận `draft` hoặc `pending_approval`, version mới nhất. **Không còn** 403 `SELF_APPROVAL_NOT_ALLOWED`. Response thêm `self_approved` |
| POST (mới) | `…/workflow-override/reject` | duyệt | Body `{version_no, base_etag?, reason}`, reason 1..500 ký tự. `pending_approval` → `rejected` |
| POST | `…/workflow/override/rebase-apply` | **duyệt** (trước đây là ghi) | 403 nếu chỉ có quyền ghi. 409 `STATE_CONFLICT` khi version mới nhất đang `pending_approval` (duyệt hoặc từ chối trước). Response `data` thêm `self_approved` (false khi người khác đã xử lý xung đột) và `resolved_by`. Thêm audit |
| POST | `…/workflow/override/conflicts/{id}/resolve` | ghi | 422 `INVALID_RESOLUTION` nếu `resolution` không nằm trong lựa chọn của loại xung đột (MySQL không lưu options: lấy theo `conflict_type`). 422 `INVALID_REQUEST` nếu `merge_manual` có giá trị không hợp lệ. Thêm audit `conflict_resolved` |
| GET (mới) | `/api/v1/company/workflow-overrides/pending` | duyệt | `{data: {items: [{type_id, type_name, version_no, draft_etag, change_note, submit_note, created_by, created_by_name, submitted_by, submitted_by_name, submitted_at}], total}}`. Gồm các override có version mới nhất là `pending_approval`; sắp theo `submitted_at`, tối đa 100. `type_name` chỉ lấy từ loại hệ thống hoặc loại của chính công ty |

## Lỗi
| HTTP | Code | Khi nào |
|---|---|---|
| 403 | `PERMISSION_DENIED` | Thiếu quyền ghi (submit, resolve) hoặc duyệt (approve, reject, publish, rebase-apply, danh sách chờ duyệt) |
| 409 | `STALE_ETAG` | `version_no`/`base_etag` không phải version mới nhất |
| 409 | `STATE_CONFLICT` | Submit bản không phải `draft`; approve bản `rejected`, hoặc bản đã là bản đang dùng (duyệt hai lần); reject bản không `pending_approval`; sửa nhóm bước trên bản không `draft`; rebase-apply khi có bản chờ duyệt |
| 404 | `INVALID_REQUEST` | Không có override hoặc version |
| 400 | `INVALID_REQUEST` | Body submit/reject không phải JSON hợp lệ |
| 400/422 | `WORKFLOW_OVERRIDE_EMPTY`, `WORKFLOW_OVERRIDE_INVALID`, `WORKFLOW_STEP_DESCRIPTION_REQUIRED` | Workflow không đủ điều kiện kích hoạt khi submit/approve (cùng mã với kích hoạt) |
| 422 | `INVALID_REQUEST` | Reason trống hoặc quá dài (`details.field=reason`); note quá dài (`details.field=note`); `merge_manual` không hợp lệ (`details.field=resolution_value`, `details.field_path`) |
| 422 | `INVALID_RESOLUTION` | Resolution không có trong lựa chọn của xung đột (`details.field=resolution`, `details.resolution_options`) |

## `merge_manual`: giá trị hợp lệ theo field
| `field_path` | Giá trị bắt buộc |
|---|---|
| `due_rule` | Chuỗi `T+N`, N ≥ 1 (cùng validator với chỉnh sửa của doanh nghiệp) |
| `assignee_role_ids` | Mảng không rỗng các chuỗi không rỗng |
| `reminder_config` | Object có `enabled` (bool, bắt buộc). Nếu `enabled=true` thì `days_before` là mảng không rỗng các số nguyên 1..365. `{}` không hợp lệ |
| `stage` | Chuỗi không rỗng, tối đa 255 ký tự (stage là nhãn tự do, không có tập cố định) |
| `documents`, `groups` | Không cho `merge_manual`: giá trị tham chiếu dữ liệu công ty và Portal chưa có ô nhập (review ROLE-06) |
| `department_id` | Không cho `merge_manual` (Rule 8) |
| `__step_existence__` | Object bước có `step_id` và `stage` (cho xung đột `step_identity_unclear`); toàn bộ workflow sau rebase được validate lại trước khi ghi |

## Audit (`audit_logs`, sau khi commit)
| Action | Meta |
|---|---|
| `disclosure.workflow_override.submitted` | `version_no` |
| `disclosure.workflow_override.approved` | `version_no`, `self_approved`, `from_state` |
| `disclosure.workflow_override.applied` (PUT `publish:true`) | `self_approved=true` |
| `disclosure.workflow_override.rejected` | `version_no`, `reason` |
| `disclosure.workflow_override.rebase_applied` (mới) | `new_version_no`, `self_approved` (false khi người khác xử lý xung đột), `resolved_by` |
| `disclosure.workflow_override.conflict_resolved` (mới) | `conflict_id`, `step_key`, `field_path`, `resolution`, `resolution_value` |

## Quyền và phạm vi
- Không thêm quyền mới: dùng `template.workflow.override.write` và `.approve` sẵn có.
- Mọi truy vấn lọc theo `company_id` của token. Danh sách chờ duyệt chỉ của công ty hiện tại.
- Không đụng Admin Web (`/cms`, `/api/v1/platform`).

## Migration
`0156_workflow_override_version_review`: thêm 6 cột nullable (`submitted_by`, `submitted_at`, `submit_note`, `rejected_by`, `rejected_at`, `decision_note`), có guard qua `information_schema`. File không ghi `ALGORITHM`; MySQL 8 tự chọn INSTANT cho cột nullable thêm ở cuối. Down xoá các cột (chỉ chạy sau khi rollback BE, xem runbook). Thêm vào `run_dev_migrations.sh`. `state` thêm giá trị `discarded` (cột `VARCHAR(32)` sẵn có, không cần migration).

## Tương thích khi deploy lệch phiên bản
| Kịch bản | Kết quả |
|---|---|
| FE cũ, BE mới | Chạy được, có suy giảm. "Kích hoạt" của người chỉ có quyền ghi trả 403 (chuyển sang trang Forbidden) và không còn lưu bản nháp. `merge_manual {}` trả 422 thay vì tắt nhắc lặng lẽ (FE cũ nuốt lỗi). Bản `pending_approval`/`rejected` hiện là "Bản nháp". DEV có **4 membership được cấp trực tiếp quyền ghi mà không có quyền duyệt** (truy vấn chỉ đọc 16/10, review ROLE-05): họ dùng "Gửi duyệt" ở FE mới. Deploy FE ngay sau BE để thu ngắn khoảng lệch |
| FE mới, BE cũ | Submit, reject và danh sách chờ duyệt trả 404: FE ẩn danh sách và báo lỗi tại chỗ. Deploy BE trước |
| Rollback BE | Binary cũ chỉ đọc `state='draft'`. **Runbook (chạy trước khi rollback, sau khi sao lưu):** (1) sao lưu `SELECT override_id, version_no, state, submitted_by, submitted_at, submit_note, rejected_by, rejected_at, decision_note FROM company_template_workflow_override_versions WHERE state IN ('pending_approval','rejected','discarded')`; (2) chỉ version **mới nhất** đang `pending_approval` của mỗi override về `draft`: `UPDATE company_template_workflow_override_versions v JOIN (SELECT override_id, MAX(version_no) mv FROM company_template_workflow_override_versions GROUP BY override_id) l ON l.override_id = v.override_id AND l.mv = v.version_no SET v.state = 'draft' WHERE v.state = 'pending_approval'`; (3) giữ `rejected` và `discarded` (binary cũ không đọc chúng, nên không duyệt đè được). Chỉ chạy down migration khi đã rollback xong và không cần metadata review |
| Thứ tự | Migration `0156` (rồi kiểm `information_schema` đủ 6 cột, vì `push-migration.sh` luôn exit 0), rồi BE, rồi FE |

## Dữ liệu
- Không cần chuyển đổi. 8 bản tự duyệt trên DEV vẫn hợp lệ theo B.
- Xung đột đã xử lý bằng `merge_manual` với giá trị cũ `{}` sẽ làm rebase-apply trả 422 (review API-02). DEV: 0 dòng `workflow_override_conflicts` (truy vấn chỉ đọc 16/10), nên không cần bước dữ liệu. Môi trường khác: đếm `resolution='merge_manual' AND resolution_status='resolved'` trước khi deploy; nếu > 0 thì đặt lại về `unresolved`.
- Bản nháp cũ nằm dưới một version `approved` mới hơn (sau rebase) không còn hiện là "bản nháp". DEV: kiểm lại khi smoke.

## Ma trận test
| Quy tắc | Test |
|---|---|
| Chỉ quyền ghi: submit được; `publish:true`, approve, reject, rebase-apply trả 403 và không có side effect | Unit service (in-memory) + MySQL |
| Quyền duyệt: kích hoạt bản mình soạn được, `self_approved=true` trong response và audit | Unit + handler |
| Chỉ version mới nhất: duyệt bản cũ sau rebase-apply trả 409 | MySQL + in-memory |
| `pending_approval` → `approved`/`rejected`; reject cần lý do; trạng thái sai trả 409 | MySQL + in-memory |
| Danh sách chờ duyệt: chỉ công ty của token, chỉ version mới nhất đang chờ, 403 khi thiếu quyền duyệt | MySQL |
| `merge_manual` theo field: `{}` và giá trị sai trả 422; giá trị đúng áp dụng được (nhắc không bị tắt) | Unit |
| FE: nút theo quyền; "Gửi duyệt"; nhãn trạng thái; duyệt/từ chối; danh sách chờ; ẩn "Áp dụng rebase" với người chỉ có quyền ghi | Vitest |

## Ngoài phạm vi (follow-up, đã chốt ở GATE A)
- Thông báo in-app cho người duyệt khi có bản gửi duyệt, và khi có người tự duyệt.
- Ô nhập giá trị cho "Xử lý thủ công" (`merge_manual`). Trong lúc chờ, Portal ẩn lựa chọn này. Hệ quả (review API-03): xung đột chỉ có lựa chọn `merge_manual` (rule 5 vai trò chưa xác định ở bước mới, rule 7 vai trò không còn, `step_identity_unclear`) không xử lý được trên Portal; FE hiện ghi chú thay cho nút. DEV hiện không có xung đột nào.
- Phương án C: doanh nghiệp tự chọn bắt buộc người duyệt khác.

## GATE A
| Mục | Giá trị |
|---|---|
| Trạng thái | **Đã duyệt** |
| Thông báo cho người duyệt | Để follow-up |
| `merge_manual` ở Portal | Ẩn; BE kiểm giá trị theo field |
| Danh sách chờ duyệt | Thẻ trên màn "Loại thông tin báo cáo", chỉ người duyệt thấy |
| Smoke DEV luồng "Gửi duyệt" | Tạo role QA tạm (chỉ đọc và ghi override), gán cho persona MEMBER ở c_001, chỉ trên một loại thông tin thử; dọn lại sau smoke |
| Người duyệt | User, trong hội thoại (trả lời câu hỏi tại GATE A) |
| Thời điểm | 16/10/2026 |

## Thay đổi contract tại GATE B (theo review, 16/10/2026)
Các điểm sau được chốt khi sửa finding của reviewer; đều nằm trong ý định đã duyệt ở GATE A (Q5-B) và đã ghi vào các bảng trên:
- `publish:true` lưu và kích hoạt trong một transaction (DB-01, API-01).
- Version không bao giờ đổi nội dung tại chỗ: sửa nhóm bước tạo version mới; xoá bản nháp giữ dòng `discarded` (DB-02, ROLE-03). Nhờ vậy duyệt `v:N` luôn đúng nội dung người duyệt đã thấy.
- Duyệt hai lần trả 409 `STATE_CONFLICT` (DB-03, API-05).
- Cột `submit_note` tách khỏi `decision_note` (DB-06, API-07).
- Rebase-apply trả 409 khi có bản chờ duyệt (DB-07); response và audit có `self_approved`/`resolved_by`; audit `conflict_resolved` (ROLE-02).
- `merge_manual` không nhận `documents`/`groups`; `__step_existence__` nhận object bước (ROLE-06, API-03). Lựa chọn hợp lệ lấy theo `conflict_type` trên MySQL (API-04).
- Danh sách chờ duyệt chỉ lấy tên loại hệ thống hoặc loại của chính công ty (ROLE-07).
- Body submit/reject không hợp lệ trả 400 (API-13).
