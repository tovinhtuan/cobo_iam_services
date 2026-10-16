# 04: Deploy DEV và smoke QA (16/10/2026)

## Deploy
| Mục | Giá trị |
|---|---|
| Commit | `bedb8d7` fix(workflow), cùng `dec192d` (tài liệu quyết định PO) |
| Lệnh | `bash deploy-dev.sh be --skip-tests` (test đã chạy trước: `02-fix.md`) |
| Migration | Không có |
| Biến môi trường mới | Không có |
| Sau deploy | `/healthz` ok, `/readyz` ready, `/api` qua nginx :3000 trả 200. Log api/worker không có panic/fatal/error, kể cả 10 phút sau smoke |
| Rollback | Deploy lại `ad54bd3` (bản BE trước) |

## Smoke
Script: `scripts/devqa/smoke_workflow_step_skip.py`, chạy qua nginx bằng persona ENT trong `c_001`. Script **có ghi** dữ liệu kiểm thử: tạo hồ sơ tiêu đề `[QA smoke MR-1] …`, submit, hoàn tất các bước và hoàn tất workflow.

Kết quả: **20/20 PASS** với loại `qa-af-option-a-ui-1788961913245` (`global_template`, 6 bước, không có tài liệu bắt buộc):
- submit tạo instance template; có đúng 1 task bước 1, giao cho chính người submit (đúng điều kiện của WF-01);
- lúc còn bước dở: `available_actions` rỗng;
- `review` trả 409 `WORKFLOW_STEPS_INCOMPLETE`, `missing_step_codes` = 6 bước. Instance vẫn `in_progress`, hồ sơ vẫn `PendingReview`;
- hoàn tất bước 1 ở màn hạn: `review` vẫn 409, thiếu 5 bước còn lại;
- hoàn tất đủ 6 bước: `available_actions` = `[review, confirm]`, `review` trả 200, instance chuyển `approved`.

## Ghi chú
- **Persona ENT trên DEV không có quyền `workflow.approve`.** Approve bị policy chặn (403 `RESPONSIBILITY_REQUIRED`) trước khi tới cổng mới; hành vi này có từ trước. Smoke dùng `review`, cùng một cổng.
  - Lần chạy đầu dùng `approve` nên 5 check fail vì lý do trên. Hồ sơ của lần đó (`01a1435e-1b56-…`) sau đó được đóng bằng `review`, instance `approved`.
- **WF-07 thấy được trên DEV:** `review` hoàn tất instance nhưng hồ sơ vẫn `PendingReview`, vì chỉ `approve` mới duyệt hồ sơ. Đây là follow-up đã ghi, không phải lỗi của MR-1.
- **Luồng proposal (ghi `completed_at`, cổng B3 khi duyệt) không smoke trên DEV.** Muốn smoke phải tạo proposal có người duyệt từng bước. Phần này đã được 5 test MySQL 8 phủ, trong đó có test xen kẽ hai transaction.
- Dữ liệu kiểm thử để lại: 2 hồ sơ `[QA smoke MR-1]` trong `c_001`, instance đều `approved`, hồ sơ `PendingReview`.
