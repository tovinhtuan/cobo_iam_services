# Onboarding doanh nghiệp qua admin platform CMS: smoke DEV và khuyến nghị (2026-10-10)

Loại task: phân tích, không sửa code.

Smoke: `docs/ai-cache/release-2026-10-10/smoke-platform-onboarding/smoke_dev_platform_onboarding.py`
- Output: `.run1.out` (có bước mời) và `.out` (không gửi mail).
- Binary api đang chạy trên DEV: `879611ce…` (commit `1479d72`).
- Chỉ dùng dữ liệu QA. Company và user tạo trong lúc chạy đã bị xoá hết (`purge`); grant tạm đã thu hồi.

## 1. Kết quả smoke

| # | Bước | Kết quả trên DEV |
|---|---|---|
| 1 | Persona CMS chỉ có `cms_operator` | Tạo company **403**, xem danh sách company **403**, vô hiệu hoá company **403**, tạo tài khoản có mật khẩu **403**. Xem danh sách user **200**. Mời user vào company bất kỳ với role admin **201** (run 1). Gán user có sẵn vào company (`assign-company`) **403**. |
| 1 | Tenant admin (ENT), member thường | 403 trên route platform. |
| 2 | Có thêm `admin.membership.invite` (cấp tạm) | Tạo company 201. Company `active` + `verified`, có role `admin_doanh_nghiep` và `user_thuong`, **0 member, không có chủ**. |
| 3 | Mời người đại diện với role `admin_doanh_nghiep` | 201. Membership active, có role admin, **`is_primary_admin = 0`**. |
| 4 | Tạo tài khoản có mật khẩu kèm `company_id` | 201. Role luôn là `user_thuong`, vì route không nhận trường role. |
| 4 | Tạo tài khoản có mật khẩu không kèm company, rồi `assign-company` với role admin | 201. Có role admin, không phải chủ. |
| 5 | Admin mới đăng nhập | Gọi API admin tenant 200. **Chuyển quyền sở hữu 403** ("only the primary admin"). **Cấp `admin.membership.invite` 403** ("only primary admin"). |
| 6 | Token CMS sửa role của member trong company khác | 404: route tenant chỉ phạm vi trong company của token; platform không có route riêng. |
| 7 | Vô hiệu hoá company | Status chuyển `inactive`, nhưng **phiên cũ vẫn chạy (200)**, **đăng nhập lại được (200)**, **API admin vẫn dùng được (200)**, **company vẫn có trong authorized-companies**. |

Nguyên nhân các mã 403 ở bước 1:
- Mọi route quản lý company (`ListPlatformCompanies`, `GetPlatformCompany`, `UpdatePlatformCompany`, `SetPlatformCompanyStatus`, `CreateCompany`) dùng `authorizePlatformCompanyAdmin`. Hàm này gọi `authorize("admin.membership.create")`, và `legacyPolicy` map action đó sang quyền `admin.membership.invite` **trong company của chính operator**.
- `CreateUser` và `AssignUserToCompany` cũng gọi cùng action đó, nên cũng bị chặn.
- Riêng `InviteUser` chỉ cần `rbac.manage`.

Phát hiện phụ:
- **DEV gửi mail thật qua `smtp.gmail.com`**, không qua mailpit. Run 1 đã gửi 1 email mời tới một địa chỉ `@cobo.test`; tên miền này không tồn tại, nên chỉ có thể sinh thư bounce. Vì vậy smoke mặc định không gửi lời mời (`SMOKE_SEND_INVITE=1` mới bật).
- Trên DEV có 11/21 company active không có chủ.
- `ValidateConfiguration` và route `/admin/company/admins` tra role code `company_admin` (đã có task riêng).

## 2. Khuyến nghị (góc nhìn PO/BA)

### Đ1. Chủ tài khoản (primary admin) cho doanh nghiệp tạo qua platform
- **Khuyến nghị:** khi mời từ platform, cho phép chỉ định "Chủ tài khoản doanh nghiệp" (cờ `as_owner`). Người đó chỉ trở thành chủ khi chấp nhận lời mời (có sự đồng ý), và chỉ khi company chưa có chủ.
- Thêm luồng **khôi phục chủ tài khoản** cho company chưa có chủ hoặc chủ đã rời đi:
  - bắt buộc nhập lý do và chứng từ yêu cầu (công văn, ticket);
  - cần người thứ hai ở platform duyệt;
  - ghi audit và gửi thông báo tới các admin của company.
- **Không** cho platform đè lên một chủ đang tồn tại. Đó là đường chiếm quyền tài khoản.
- Lý do: chuyển quyền sở hữu, cấp quyền mời, liên hệ hợp đồng/thanh toán và trách nhiệm công bố thông tin đều cần đúng một người chịu trách nhiệm. Về nghiệp vụ, người này thường là người đại diện theo pháp luật hoặc người được uỷ quyền CBTT.
- Backfill: lập danh sách company chưa có chủ (DEV 11/21; cần đếm trên PROD) và xử lý qua luồng khôi phục, không sửa thẳng trong DB.

### Đ2. Quyền của `cms_operator` và gate các route platform
- Hiện trạng ngược đời: thao tác mạnh nhất là mời một **admin vào bất kỳ company nào**, nhưng lại được chặn lỏng nhất (chỉ cần `rbac.manage`). Trong khi đó, xem danh sách company lại bị 403. Gate đang mượn quyền tenant (`admin.membership.invite`, `rbac.manage`) của company mà operator thuộc về.
- **Khuyến nghị:** tách vai trò platform theo nhiệm vụ, kiểm bằng quyền platform riêng:
  - **Biên tập nội dung CMS:** template, record, lịch, media. Không đụng tới tài khoản doanh nghiệp.
  - **Vận hành khách hàng (CSKH/onboarding):** vòng đời company, mời chủ hoặc admin, gửi lại lời mời, yêu cầu reset mật khẩu.
  - **Super admin platform:** danh mục role/quyền, quản lý operator, khôi phục chủ, break-glass.
- Gợi ý mã quyền: `platform.company.manage`, `platform.tenant_user.manage`, `platform.owner.recover`. Mọi route `/platform/cms/admin/*` của cùng một nhóm dùng **một** gate. Bỏ sự phụ thuộc vào `admin.membership.invite` của company operator.
- **Tạo tài khoản kèm mật khẩu cho người của doanh nghiệp: không nên.** Nếu nhân viên platform biết mật khẩu thì nhật ký kiểm toán "ai đã nộp CBTT" mất giá trị. Nên chỉ cho mời, để người dùng tự đặt mật khẩu. Tài khoản có mật khẩu chỉ giữ cho demo hoặc nội bộ, dưới quyền super admin.

### Đ3. Platform có được đổi hoặc gỡ role của member trong company khác không
- **Khuyến nghị: không** làm trình sửa role xuyên tenant tổng quát. Doanh nghiệp tự quản lý nhân sự của mình, và việc phân tách trách nhiệm cũng rõ ràng hơn.
- Chỉ mở các "thao tác hỗ trợ" có giới hạn, mỗi thao tác bắt buộc nhập lý do, ghi audit và báo cho chủ doanh nghiệp:
  - khôi phục chủ (Đ1);
  - gửi lại lời mời, yêu cầu reset mật khẩu;
  - khoá hoặc mở một membership khi khẩn cấp.
- Dài hạn: "support access" theo kiểu break-glass. Operator chỉ vào được company khi chủ doanh nghiệp đồng ý, có giới hạn thời gian và ghi audit đầy đủ.

### Lỗi cần sửa (không cần quyết định sản phẩm, chỉ cần chốt định nghĩa trạng thái)
- **HIGH: vô hiệu hoá company không có tác dụng.** Cần:
  - chặn ở login, select-company, authorized-companies và resolver quyền;
  - thu hồi phiên và xoá cache khi đổi trạng thái.
- Đề xuất định nghĩa trạng thái:
  - **Tạm ngưng** (ví dụ chưa thanh toán): chỉ đọc và xuất dữ liệu, kèm banner nhắc. Lý do: nghĩa vụ CBTT có hạn chót theo luật, doanh nghiệp vẫn cần truy cập hồ sơ.
  - **Ngừng hoạt động**: không truy cập được; dữ liệu giữ N ngày theo hợp đồng.
- **Gán user có sẵn vào company (`assign-company`) hiện không cần người đó đồng ý.** Nên chuyển thành lời mời tham gia (pending cho tới khi chấp nhận). Cùng gốc với ROLE-09.
- **DEV gửi mail thật:** nên trỏ SMTP của DEV vào mailpit, hoặc chỉ cho gửi tới danh sách tên miền được phép (roadmap mục 4 trong `dev-qa/README.md`).

## 3. Luồng onboarding đề xuất
1. CSKH tạo company với dữ liệu đã xác minh (mã số thuế, tên, người đại diện). Trạng thái "chờ kích hoạt".
2. Chỉ định chủ tài khoản: email, họ tên, chức danh. Gửi lời mời "Chủ tài khoản". Người đó chấp nhận thì trở thành primary admin kèm role admin.
3. Kích hoạt gói. Company chuyển sang active.
4. Chủ doanh nghiệp tự mời admin và nhân viên trong portal.
5. Platform chỉ còn các thao tác hỗ trợ có giới hạn (Đ3). Tạm ngưng hoặc ngừng hoạt động có hiệu lực ngay.

## 4. Thứ tự đề xuất
- **P0:** sửa lỗi vô hiệu hoá company không chặn truy cập (HIGH).
- **P1:**
  - chỉ định chủ khi mời và luồng khôi phục chủ, kèm backfill;
  - gom gate các route platform về một chỗ; hạn chế quyền mời admin, chỉ cho vai trò vận hành khách hàng.
- **P2:**
  - chỉ cho mời với tài khoản doanh nghiệp;
  - bắt buộc đồng ý khi gán user có sẵn;
  - support access theo kiểu break-glass;
  - mail DEV qua mailpit.
