# PR-B: leo quyền và thao tác chéo tenant trong quản trị công ty — 2026-10-10

- Nguồn: `../risk-review-2026-10-10/10-risk-report.md`
- Kế hoạch: `../risk-review-2026-10-10/20-fix-plan.md`
- Base: commit PR-A `2996f00`
- Quyết định của user: ROLE-03 chọn **phương án A**. Người không có `rbac.manage` chỉ được cấp role/quyền nằm trong quyền hiệu lực của chính mình.

| Finding | Mức | Triệu chứng (xác nhận bằng test fail trước khi sửa) |
|---|---|---|
| ROLE-02 | HIGH | Tạo resource-scope rule hoặc workflow-assignee rule với `company_id` của tenant khác thì rule được ghi vào tenant đó |
| ROLE-03 | HIGH | Người chỉ có `admin.membership.invite` mời/tạo user hoặc gán role `admin_doanh_nghiep` (có `rbac.manage`) đều thành công |
| ROLE-13 | MEDIUM | Admin công ty (không phải platform operator) gỡ được role hoặc direct permission `platform.cms.view` của CMS operator, kể cả qua hàng chờ duyệt |
| ROLE-05 | MEDIUM | `RemoveRole` gỡ được role admin của primary admin, hoặc của admin cuối cùng |
| ROLE-12 | MEDIUM | Rollback rule `alert_channel_prefs` áp dụng ngay, không qua approval, không kiểm payload/gói. Xoá được rule prefs. |
| BES-09 | LOW | Duyệt prefs premium sau khi công ty hạ gói vẫn áp dụng |
| ROLE-14 | INFO | Target của break-glass tự duyệt được grant của mình |
| RP-07 | — | Không có guard ngăn action admin mới rơi về `system.settings` mặc định của `legacyPolicy` |
