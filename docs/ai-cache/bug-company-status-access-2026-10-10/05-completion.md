# Hoàn tất: trạng thái doanh nghiệp có tác dụng lên truy cập (2026-10-10)

```text
Bug: vô hiệu hoá company không chặn truy cập; chưa có trạng thái "Tạm ngưng".

Reproduction:
  - Smoke DEV trên binary cũ:
    - smoke-company-inactive/*.before-deploy.out: 10/15, phiên cũ, refresh, đăng nhập mới đều lọt.
    - smoke-company-suspended/*.before-deploy.out: 11/16, route /suspend 404, các thao tác ghi đều chạy.
  - Unit test:
    - TestInspectAccessToken_CompanyInactiveBlocksAtOnce, ..._CompanySuspendedIsReadOnly;
    - TestLogin_OnlyCompanyInactiveIsRefused, TestSelectCompany_InactiveCompanyRefused, TestRefresh_CompanyDeactivatedAfterLoginRefused;
    - TestWriteRequestMarker, TestSetPlatformCompanyStatus_*CannotBe*, TestAccessByStatus.

Root cause: companies.status chỉ được ghi; đường xác thực và kiểm token không đọc nó (00-report.md).

Fix:
  - c43b9cc (vòng A, "Ngừng hoạt động" chặn hẳn):
    - sessionbound kiểm trạng thái company mỗi request;
    - login, select/switch, refresh, /me, /me/companies, personal ops, token template-builder;
    - workflowdept giữ nguyên mã lỗi;
    - guard platform không cho tự khoá.
  - 5715d6c (vòng B, "Tạm ngưng" chỉ đọc và xuất):
    - migration 0152 và route /suspend;
    - middleware đánh dấu request ghi, sessionbound trả 403 COMPANY_SUSPENDED.

Verification:
  - go test: đúng 16 fail có sẵn; vet sạch (trừ lỗi có sẵn ở workflowfulfillment); -race sạch.
  - Reviewer bảo mật: không có HIGH; MEDIUM (tự khoá platform) và các LOW đã sửa trong c43b9cc.
  - DEV:
    - c43b9cc → api 3836551b…: smoke-company-inactive 18/18, regression 13 smoke đạt.
    - 5715d6c → migrate 0152 rồi api d4f86310…: smoke-company-suspended 16/16; company-inactive 18/18;
      12 smoke regression đạt ngay; platform-onboarding và role23 bị lỗi mạng (connection reset / timeout,
      không có lỗi upstream ở nginx, API không restart), chạy lại đạt 23/23 và 14/14.
    - Log 0 ERROR/panic.
  - Rollback point: bin/*.rollback.20261010T140422Z (= 1479d72) và bin/*.rollback.20261010T141809Z (= c43b9cc).
    Migration 0152 có down: suspended → inactive.

Blast radius / data repair:
  - DEV có 21 company, tất cả active: không ai bị khoá khi deploy.
  - Trước khi lên PROD: đếm company inactive (từ nay sẽ bị khoá thật) và báo khách hàng.

Follow-ups:
  - FE (cobo_web_design):
    - xử lý toàn cục 403 COMPANY_INACTIVE / COMPANY_SUSPENDED;
    - lỗi COMPANY_INACTIVE ở trang login;
    - banner "Tạm ngưng" theo company_status; ẩn hoặc vô hiệu nút ghi;
    - CMS: nút "Tạm ngưng" (POST /suspend) và nhãn trạng thái.
  - Worker vẫn gửi nhắc việc/thông báo cho company inactive. Cần quyết định có lọc không.
    Với suspended, việc tiếp tục nhắc hạn CBTT là hợp lý.
  - Auto-login sau khi chấp nhận lời mời không bao giờ chạy ở production: sessionbound đòi session có trước khi cấp token (có sẵn từ trước).
  - deploy-artifacts/push-migration.ps1:
    - bước verify gọi `mysql -e` bị vỡ chuỗi trên Windows, nên deploy migrate báo lỗi dù migration đã áp dụng và đã ghi;
    - script gắn cứng thông tin root MySQL trên dòng lệnh.
  - migrations/run_dev_migrations.sh thiếu 0150, dù 0150 đã áp dụng trên DEV.
  - Smoke DEV có thể dính lỗi mạng tạm thời; smoke nào dừng giữa request tạo dữ liệu có thể để sót bản ghi (đã gặp 1 user, đã dọn).
```
