# Release DEV 2026-10-09: candidate

| Repo | Branch | HEAD | Working tree |
|---|---|---|---|
| cobo_iam_services | improve/ui-theme | `0609fdf` | **dirty**: C2 (internal/workflowconfig, httpserver/server.go), C3 (internal/adhoc), go.mod (x/text direct), tasks/, docs/ai-cache/* |
| cobo_web_design | improve/ui-theme | `37a799db` | **dirty**: src/features/cms-core/{language.tsx, templates/workflow/*} (403 UX), docs/ai-cache (adhoc v3 banner, bug-workflow-release-403), `reusable-task-updates.md` (đã sửa từ trước session) |

## Đang chạy trên DEV (kiểm tra read-only qua SSH, 2026-10-09)
- `bin/api`, `bin/worker`: 2026-10-07 20:08. `web/dist/index.html`: 2026-10-07 19:02.
- Containers: api/worker Up 36h, web Up 37h (healthy), mysql/redis/mailpit healthy.
- Build trên DEV có trước commit `dea4e59 fix bug import` (2026-10-08 00:12). Không có SHA chính xác cho bản đang chạy (các lần deploy trước dùng working tree chưa commit).

## Nội dung bản deploy so với DEV
- `dea4e59` (template import normalizer/validator + FE import): **có regression đã ghi nhận** (offset 0 → BLOCKER; 6 test `disclosure/app` liên quan import FAIL ở HEAD; BES-09 bỏ check deadline periodic khi `use_structure_deadline=false`).
- C2: gate quyền cho `workflowconfig` (`platform.cms.view` + `cms.template.*`).
- C3: gỡ endpoint `adhoc-migrate-legacy-approvals`; `AdminApprove` từ chối identity rỗng; `json:"-"` cho `Subject`.
- FE: publish/activate 403 → lỗi tại chỗ.
- Không có migration mới.
