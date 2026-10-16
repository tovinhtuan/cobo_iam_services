# 03: Review (wf-bugfix Phase 6)

Bản sửa chạm vào transaction MySQL của task workflow và luật duyệt hồ sơ, nên có hai reviewer đọc độc lập: `db-tx-reviewer` và `be-security-reviewer`. Sau khi sửa theo vòng 1, `db-tx-reviewer` review lại phần thay đổi (vòng 2).

## Vòng 1

| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| DB-01 | HIGH | Task proposal không ghi `completed_at`, trong khi upload tài liệu lấy "bước hiện tại" từ step runtime. Duyệt task 1 xong thì bước 2 không upload được (`WORKFLOW_STEP_NOT_CURRENT`), còn task 2 bị cổng B3 chặn (422). Luồng bị kẹt, và đây là hồi quy do chính bản sửa WF-02 | Đã sửa. Task proposal ghi `completed_at` cho bước của nó trong cùng transaction; nếu bước đã hoàn tất trước thì giữ nguyên |
| BES-02, BES-03 | MEDIUM | Cùng gốc với DB-01. Thêm hệ quả: tài liệu của bước đã duyệt vẫn xoá được | Cùng bản sửa DB-01. Bước đã xong bị khoá upload/thay/xoá |
| DB-02 | LOW | `FOR SHARE` khi đọc bước đã xong là thừa | Đã bỏ. `completed_at` chỉ ghi một lần (không có chỗ nào gán NULL hay xoá dòng), nên đọc cũ chỉ có thể gây 409 giả |
| DB-03 | LOW | Khoá snapshot và step-state cả với bước không có tài liệu bắt buộc | Đã sửa bằng probe; vòng 2 sửa tiếp chỗ đặt probe (V2-DB-04) |
| DB-04 | INFO | Thứ tự lỗi và giới hạn của in-memory | Ghi trong `02-fix.md` |
| BES-01 | MEDIUM (plausible) | Instance không phải proposal mà không có snapshot thì cổng WF-01 không áp dụng | Không sửa code. Tạo instance bắt buộc có snapshot (`ValidateSnapshot`, "Model A invariant"), DEV có 0 instance như vậy. Đã thêm truy vấn kiểm tra cho prod |
| BES-04 | MEDIUM (có từ trước) | Người nộp tự hoàn tất các bước rồi tự duyệt | Đã hẹp hơn nhiều; follow-up WF-06 / quyết định Q5 |
| BES-05 | LOW | `stepsOpen` coi lỗi đọc instance là "không còn bước dở" | Giữ hành vi, sửa comment. Đổi sang "lỗi thì coi như còn bước dở" làm hỏng 6 test có sẵn mà không thêm an toàn: thao tác duyệt đọc lại instance và lỗi ở đó, còn repository vẫn kiểm trong transaction |
| BES-06 | INFO | In-memory không chạy B3 | Ghi trong `02-fix.md` |

## Vòng 2 (`db-tx-reviewer`, phần đã sửa)

| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| V2-DB-04 | MEDIUM (confirmed) | Probe của DB-03 là câu đọc thường đầu tiên trong transaction, nên read view REPEATABLE READ được chốt trước khi lấy khoá. Một lệnh xoá đang giữ khoá step-state (`DeleteInTx`) commit trong lúc thao tác duyệt chờ khoá, nhưng bước đếm file vẫn thấy file cũ. Cổng B3 cho qua, bước bị đánh dấu hoàn tất khi thiếu tài liệu bắt buộc | Đã sửa: probe chạy ngoài transaction, trước `BeginTx`. Snapshot tài liệu chỉ được ghi khi tạo instance và không đổi sau đó (reviewer đã kiểm mọi chỗ ghi). Test xen kẽ `TestTaskTransitionGates_GateSeesADeleteCommittedWhileItWaits`: fail 3/3 khi đặt lại câu đọc thường trong transaction, pass 5/5 với bản sửa |
| V2-DB-05 | LOW | Không test nào đi qua nhánh ODKU trên dòng placeholder (đường phổ biến nhất trên prod), nên đảo thứ tự hai phép gán cũng không bị phát hiện | Đã sửa: helper upload trong test tạo placeholder như `UploadInTx`, và test assert `completed_by` của bước 2. Đảo thứ tự phép gán thì test fail |

Reviewer xác nhận vòng 2 không có vấn đề ở các điểm sau:
- **Thứ tự khoá:** không có chu trình deadlock với `UpsertStepCompleted`, fulfillment upload/replace/delete và step evidence. Nếu gặp lỗi 1213/1205 thì transaction rollback toàn bộ; retry an toàn nhờ CAS `status = 'pending'`.
- **Khẳng định "probe không bỏ sót snapshot":** đúng.
- **Khẳng định "`completed_at` chỉ ghi một lần":** đúng.
- **Các chỗ đọc `completed_at`:** `deadlinealerts`, `workflowfulfillment`, `workflowstepevidence`, `reminder`. Với proposal, các chỗ này giờ đi theo task. Timeliness của bước chuyển từ "quá hạn mãi" sang đúng hạn/trễ theo ngày duyệt.
- **Hiệu năng:** thêm 2 query cho mỗi lần liệt kê task của một instance, có index.

## Follow-up ngoài phạm vi
- Fallback trong `lockStepCompleted` (fulfillment, evidence) và `UpsertStepCompleted` coi mọi lỗi INSERT là "insert đồng thời". Nên chỉ coi mã 1062 như vậy. Lỗi này có từ trước và bản sửa này không kích hoạt nó.
- `VALUES()` trong ODKU đã deprecated từ MySQL 8.0.20. Code cũ cũng dùng; nên chuyển cả nhóm sang alias một lần.
- Truy vấn kiểm tra prod và kế hoạch backfill: `02-fix.md`.
