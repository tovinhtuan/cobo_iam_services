# Bug: trạng thái doanh nghiệp không có tác dụng lên quyền truy cập (2026-10-10)

## Intake
- **Triệu chứng (smoke DEV):** `docs/ai-cache/release-2026-10-10/smoke-platform-onboarding/smoke_dev_platform_onboarding.out`, api `879611ce…`. Platform vô hiệu hoá company (`POST /platform/cms/admin/companies/{id}/deactivate`) → `companies.status = inactive`. Sau đó admin của company vẫn:
  - dùng được phiên cũ (200);
  - đăng nhập lại vào company đó (200);
  - gọi được API admin (200);
  - thấy company trong `GET /me/authorized-companies`.
- **Mong đợi:** user chốt định nghĩa ngày 2026-10-10:
  - **Ngừng hoạt động** (`inactive`): chặn hẳn truy cập; dữ liệu giữ theo hợp đồng.
  - **Tạm ngưng** (`suspended`, ví dụ chưa thanh toán): chỉ đọc và xuất dữ liệu, vì doanh nghiệp vẫn có hạn chót CBTT theo luật.
- **Việc đã làm trước đây:**
  - `c40cf3f`: allowlist `active|inactive` khi ghi; migration `0126` thêm CHECK constraint.
  - PR-A `2996f00`: thu hồi quyền có hiệu lực ngay; `sessionbound` kiểm session mỗi request.
  - Chưa có chỗ nào đọc `companies.status` ở đường xác thực.

## Root cause (đã xác nhận bằng đọc code và smoke)
`companies.status` chỉ được ghi, không được đọc ở đường xác thực hay phân quyền:
- `MembershipQueryService.GetMembershipsByUser` / `GetActiveMembership` (`internal/companyaccess/infra/mysql/membership_query.go:20,44`) chỉ lọc theo `membership_status`.
- `Login`, `bindCompany` (select/switch), `Refresh`, auto-login sau chấp nhận lời mời (`internal/iam/app/service.go`) và `/me/companies` (`internal/iam/transport/http/me_handler.go:178`) dùng các truy vấn đó.
- Việc kiểm token mỗi request (`internal/iam/infra/token/sessionbound/manager.go:44`) chỉ kiểm session, không kiểm company.
- Tầng: service và auth (IAM). Không phải lỗi FE, cache hay config.

## Contract
### Vòng A: `inactive` = chặn hẳn
- Mã lỗi mới **`COMPANY_INACTIVE`** (HTTP 403).
- Mọi request mang token có company context, nếu company đang `inactive` → 403 `COMPANY_INACTIVE`. Áp dụng cho mọi route kiểm token qua `sessionbound`, gồm cả `/internal/v1/authorize` và route platform khi chính company của operator bị vô hiệu hoá. Phiên đang chạy bị chặn ngay ở request kế tiếp; kích hoạt lại thì có hiệu lực ngay, không cần đăng nhập lại.
- `POST /auth/login`:
  - membership thuộc company `inactive` bị bỏ qua;
  - nếu user có membership active nhưng tất cả đều ở company `inactive` → 403 `COMPANY_INACTIVE`, không tạo session;
  - user không có membership nào thì giữ nguyên hành vi `no_company_onboarding`.
- `POST /auth/select-company`, `/auth/switch-company`, `/auth/refresh` tới company `inactive` → 403 `COMPANY_INACTIVE`.
- `GET /me/companies` và `/me/authorized-companies`: bỏ company `inactive`; mỗi item có thêm `company_status` (thay đổi additive).
- Auto-login sau chấp nhận lời mời và personal ops: bỏ company `inactive`.
- Giá trị lạ hoặc legacy của `companies.status` được đọc khoan dung, chỉ `inactive` mới chặn (theo đúng nguyên tắc của gói `companystatus`).

### Vòng B: `suspended` = chỉ đọc và xuất (làm sau khi vòng A đạt trên DEV)
- Migration mới: CHECK constraint cho phép thêm `suspended`; platform có route tạm ngưng.
- Đăng nhập và chọn company vẫn được; `company_status = suspended` để FE hiện banner.
- Mọi request ghi (POST/PUT/PATCH/DELETE) trong company `suspended` → 403 **`COMPANY_SUSPENDED`**, trừ allowlist:
  - đăng nhập, đăng xuất, refresh, chọn company;
  - hồ sơ cá nhân, đổi mật khẩu, avatar (thuộc tài khoản user, không phải dữ liệu company);
  - các route xuất dữ liệu và các route POST chỉ đọc. Khảo sát ngày 2026-10-10: phần xuất/tải dữ liệu đều là `GET` (`/admin/config-export/{id}[/download]`, các route `.../files/{file_id}/content`), nên được cho qua tự nhiên. Các route POST chỉ đọc cần đưa vào allowlist:
    - `POST /api/v1/admin/config-export`;
    - `POST /api/v1/admin/configuration/validate`;
    - `POST /api/v1/admin/notification-rules/simulate`;
    - `POST /api/v1/template-builder/validate`.
- Quyền hiệu lực bỏ các quyền ghi, để FE tự ẩn nút thao tác.

### FE mapping (follow-up ở cobo_web_design)
- Xử lý toàn cục 403 `COMPANY_INACTIVE`: thông báo "Doanh nghiệp đã ngừng hoạt động", rồi về màn hình chọn company hoặc đăng xuất.
- Trang login hiển thị lỗi `COMPANY_INACTIVE`.
- Vòng B: banner "Tạm ngưng", và xử lý 403 `COMPANY_SUSPENDED`.

## Failure modes
- Thêm một truy vấn khoá chính `companies` cho mỗi request có company context, cạnh truy vấn session sẵn có.
- Lỗi DB khi đọc trạng thái → trả lỗi (fail closed), giống cách kiểm session.
- Company bị xoá cứng mà vẫn còn token → bị chặn như `inactive`.
