# 04: Deploy DEV và smoke QA (16/10/2026)

## Deploy
| Mục | Giá trị |
|---|---|
| Commit | BE `2d9df78` (`cobo_iam_services`), FE `a9823e59` (`cobo_web_design`) |
| Thứ tự | BE trước, FE sau |
| Lệnh BE | `bash deploy-dev.sh be --skip-tests` (test đã chạy trước: `02-fix.md`) |
| Lệnh FE | `VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED=true VITE_STEP_COMMENT_MENTIONS_V11=true sh deploy-dev.sh fe --skip-tests` (cùng bộ cờ với release RBAC ngày 16/10) |
| Migration | Không có |
| Sau deploy | `/healthz` ok, `/readyz` ready, `/api` qua nginx 200. Bundle FE `index-CXR8Bzca.js`. Log api/worker không có panic/fatal/error, cũng không có `adhoc: finalize failed`, kể cả 15 phút sau smoke |
| Rollback | **FE trước:** build lại `3442d8fa` với cùng bộ cờ. **BE sau:** deploy lại `28142fb` |

## Smoke
Script: `scripts/devqa/smoke_adhoc_v12.py`. Chạy qua nginx; người đề xuất là ENT, 2 người duyệt là ENT2 và USER, trong `c_001`. Loại thông tin: `qa-irregular-alert-20260904a`.

Script **có ghi** dữ liệu kiểm thử: 3 đề xuất có ghi chú `[QA smoke MR-2] …`, trong đó 2 đã duyệt (mỗi đề xuất có record và workflow) và 1 bị từ chối.

Kết quả: **19/19 PASS**.
- **H1:**
  - A nhập T0 chốt và ghi chú. Khi đề xuất còn chờ, B thấy đó là giá trị hiện hành, và `finalize_retry_available` chưa bật.
  - B duyệt với body rỗng thì finalize chạy. Đề xuất `approved` giữ T0 và ghi chú của A.
  - Record có đúng 1 workflow instance, `t0_date` = T0 của A. Như vậy finalize truyền đúng T0 chốt.
- **H2 (hồi quy):** khi còn phiếu chưa bỏ, B vẫn từ chối được.
- **DB-01:** sau khi A duyệt, B gửi 2 request approve cùng lúc (double-click). Cả hai trả 200 `finalized=true`, đề xuất `approved`, và record có **đúng 1** workflow instance.
- **FE:** bundle đang chạy có trang chi tiết mới ("Hoàn tất phê duyệt", `finalize_retry_available`).
- Submit của cả 3 đề xuất đều đi qua bước kiểm lại reviewer và quyền của người đề xuất (XL-06).

## Không smoke trên DEV
Các trường hợp sau cần ép finalize lỗi giữa chừng hoặc làm hỏng quyền của persona, nên đã được MySQL 8 và test phủ thay (`02-fix.md`):
- đủ phiếu mà còn chờ (409 `all_reviewers_approved`, nút thử lại);
- nhả phiếu và khoá giá trị chốt;
- 422/403 khi submit.
