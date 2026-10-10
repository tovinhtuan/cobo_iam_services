# Vòng sửa ROLE lần lượt: sửa → check → commit → deploy BE → smoke DEV (2026-10-10)

Quy tắc của user: smoke DEV phải pass thì mới sang ROLE tiếp theo. Smoke fail thì dừng, không tự rollback.

Fixture QA mới: `scripts/devqa/provision_qa_company.py` tạo "QA Persona Company (smoke only)" qua self-service API.
- `E2E_QA_COMPANY_ID=41eab473-6cf4-4c53-aa2e-34f5d8e2495c`.
- ENT là chủ và primary admin, ENT2 là admin, MEMBER là `user_thuong`.

| # | Mục | Commit | Deploy (UTC) | Rollback point | Smoke |
|---|---|---|---|---|---|
| 1 | RP-10 / H16 (+ ROLE-26, ROLE-27, ROLE-28 từ review) | `ae58ecc` | 09:07, api `7760aaa6…` | `bin/*.rollback.20261010T090608Z` (= `c6b4ab6`) | `smoke-rp10-ownership` **17/17**; regression `smoke-pr-ab` **29/29**; log 10 phút: 0 lỗi, 0 response 5xx |
| 2 | ROLE-07 / RP-05 (+ bug SQL invite scope trưởng phòng: `ORDER BY name` → 500) | `851f45e` | 09:20, api `4cdb51eb…` | `bin/*.rollback.20261010T091858Z` (= `ae58ecc`) | `smoke-role07-operator` **10/10**; regression `smoke-rp10` **17/17**, `smoke-pr-ab` **29/29** (sau khi sửa 1 assertion sai, xem ghi chú); log 0 lỗi |

| 3 | ROLE-08 / RP-06 | `512fc04` | 09:31, api `40178c6f…` | `bin/*.rollback.20261010T093012Z` (= `851f45e`) | `smoke-role08-cms-alias` **9/9** (trên binary cũ: 6/9, tái hiện lỗi: alias tenant qua cả 3 gate); regression role07 **10/10**, rp10 **17/17**, pr-ab **29/29** |
| 4 | ROLE-25 (+ ROLE-23 phần primary; regression PR-B: owner không hạ được admin) | `1bf6318` | 09:53, api `13d4e6e1…` | `bin/*.rollback.20261010T095242Z` (= `512fc04`) | `smoke-role25-primary-guards` **12/12** (trên binary cũ: race làm ENT2 vừa là primary vừa inactive); regression role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29** |
| 5 | ROLE-09 (status allowlist) + ROLE-21 (kiểm role trước khi ghi) | `4f3a586` | 10:59, api `f9f99471…` | `bin/*.rollback.20261010T105836Z` (= `1bf6318`) | `smoke-role09-21-membership-create` **9/9** (trên binary cũ: 8/9, ROLE-09 trả 409 thay vì 400); regression role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29** |
| 6 | ROLE-10 / RP-11 | `ef58f12` | 11:35, api `481254e7…` | `bin/*.rollback.20261010T113448Z` (= `4f3a586`) | `smoke-role10-team-dept` **10/10** (trên binary cũ: thêm được member phòng X vào team phòng Y bằng body `department_id=X`); regression role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29** |
| 7 | RP-08 (một định nghĩa "cần duyệt" khi gỡ quyền) | `48bd3d0` | 11:50, api `caf0fbef…` | `bin/*.rollback.20261010T114917Z` (= `ef58f12`) | `smoke-rp08-critical-set` **7/7** (trên binary cũ: gỡ `workflow.step.override` ngay với 200); regression role10 **10/10**, role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29** |
| 8 | ROLE-20 (chỉ operator mới được làm một member mất tư cách operator) | `d23fc6f` | 12:03, api `78e09dd3…` | `bin/*.rollback.20261010T120220Z` (= `48bd3d0`) | `smoke-role20-operator-outcome` **6/6** (trên binary cũ: 4/6; owner gỡ `system.settings` của operator nhận 202 và vào hàng duyệt, duyệt xong là member mất tư cách operator); regression rp08 **7/7**, role10 **10/10**, role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29**; log 0 ERROR/panic |
| 9 | ROLE-24 / BES-28 (action `admin.*` không có policy → deny, không rơi về `system.settings`) | `fb0571b` | 12:16, api `bae94664…` | `bin/*.rollback.20261010T121502Z` (= `d23fc6f`) | `smoke-role24-admin-default-deny` **8/8** (trên binary cũ: 5/8; `admin.qa_smoke.unknown` trả allow qua `system.settings`); regression role20 **6/6**, rp08 **7/7**, role10 **10/10**, role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29**; log 0 ERROR/panic |
| 10 | ROLE-23 / BES-22 (company luôn còn admin, kể cả khi thao tác đồng thời) | `c4f2fd9` | 12:39, api `a23a6f64…` | `bin/*.rollback.20261010T123808Z` (= `fb0571b`) | `smoke-role23-last-admin` **14/14**: race 5+5 vòng, mỗi vòng đúng 1 bên thắng, bên kia 409, không 5xx. Trên binary cũ: admin duy nhất tự deactivate và tự delete được; gỡ role admin chéo nhau thì cả hai thắng 5/5 vòng, deactivate chéo thì cả hai thắng 2/5. Regression role24 **8/8**, role20 **6/6**, rp08 **7/7**, role10 **10/10**, role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29**; log 0 ERROR/panic |
| 11 | ROLE-23 role-level (approval RBAC không được lấy `rbac.manage` của admin cuối cùng) | `1479d72` | 13:05, api `879611ce…` | `bin/*.rollback.20261010T130425Z` (= `c4f2fd9`) | `smoke-role23b-rbac-approval` **11/11**. Trên binary cũ: 8/12; ca submit được xếp hàng (202) và duyệt xong thì `admins_after=[]`; ca approve (xếp hàng khi còn 2 admin, admin kia bị hạ trước lúc duyệt) duyệt 200 và cũng không còn admin. Regression role23 **14/14**, role24 **8/8**, role20 **6/6**, rp08 **7/7**, role10 **10/10**, role09-21 **9/9**, role25 **12/12**, role08 **9/9**, role07 **10/10**, rp10 **17/17**, pr-ab **29/29**; log 0 ERROR/panic |
Ghi chú từng vòng:
- **#1:**
  - Checks: tập test fail giống hệt baseline (16 test có sẵn); `-race` sạch.
  - Review admin-role: đúng. ROLE-25 (race khoá/xoá/thu hồi admin với chuyển quyền) đưa vào hàng đợi, gộp với ROLE-23.
  - Hai lần chuyển đồng thời trên DEV: bên thua nhận 403 vì đọc sau commit. Nhánh CAS 409 được unit test có barrier phủ.
- **#2:**
  - Chạy smoke trên binary cũ trước khi deploy: invite của người chỉ có `platform.cms.view` trả **500** do bug SQL `ListDepartmentIDsByHeadMembership` (`ORDER BY name`; cột thật là `department_name`). Lỗi này làm hỏng mọi lượt mời của trưởng phòng trên MySQL (thêm một ca RP-16). Đã sửa trong cùng commit, có thêm integration test.
  - Regression `smoke-pr-ab` ban đầu FAIL ở assertion "picker của admin có `admin_doanh_nghiep`". Nguyên nhân: role `admin_doanh_nghiep` của c_001 có `disclosure_type.manage` (`module_name = 'cms'`), nên tenant admin vốn đã không mời được với role này (403 trong `validateEnterpriseInviteRole` từ bản sửa C4). Picker giờ chỉ khớp với kết quả thật.
  - Đây là kỳ vọng sai của smoke, không phải regression; đã sửa assertion và chạy lại pass. Gốc vấn đề là ROLE-08 (mục kế tiếp).
- **#3:**
  - Kiểm dữ liệu DEV trước khi sửa: mọi người đang có `platform.cms.view` (m_101, m_106, m_107, m_108, m_qa_cms_c001) đều đã có `cms.template.write` qua role, nên bỏ alias không làm ai mất quyền.
  - Giữ alias `disclosure_type.publish` (tier HighRisk, không cấp trực tiếp được) cho activate/archive, và `rbac.manage` (TenantAdminOnly) cho read/config.
  - FE `cms-core/permissionGuards.ts:8` vẫn coi `disclosure_type.manage` là đủ để ghi CMS. Chỉ lệch UX, đưa vào PR-F.
- **#4:**
  - Trên binary cũ (`512fc04`), smoke tái hiện ROLE-25 ở vòng race thứ 2: transfer và deactivate cùng trả 200, khiến ENT2 vừa là primary vừa inactive. Fixture được khôi phục bằng `provision_qa_company.py`.
  - Cũng phát hiện một **regression do PR-B gây ra**:
    - Guard ROLE-13 phía gỡ role đánh giá tầng platform theo `module_name`, mà `disclosure_type.manage` (quyền tenant) bị gắn module `cms`.
    - Hệ quả: owner không hạ được admin khác qua `PUT /primary-role` (403). Xác nhận trên DEV bằng dữ liệu QA.
    - Đã sửa: guard phía gỡ chỉ xét mã quyền (`platform.*`, `cms.*`, `EnterpriseDenyCodes`).
  - Bất đối xứng còn lại (có từ bản sửa C4, chưa đổi):
    - Tenant admin vẫn không **gán** được `admin_doanh_nghiep` qua AssignRole/primary-role, vì phía gán vẫn phân loại theo module.
    - **Cần quyết định:** phân loại lại `disclosure_type.manage` thành quyền tenant ở mọi nơi.
  - Parity in-memory (RP-16), đã sửa: `RemoveRole` hiểu key `r_invite_<code>`; `ListRolePermissions` trả `module_name` theo catalog.
- **#5:**
  - ROLE-09: chỉ làm phần chắc chắn (status allowlist). Phần "tenant gắn một user có sẵn mà người đó không đồng ý" (bỏ route khỏi tenant, hay chuyển sang luồng mời) **cần quyết định sản phẩm**. Các test C4 hiện vẫn cho tenant admin tạo membership trong chính company của mình.
  - ROLE-21: trên MySQL, bản cũ tạo membership trước rồi xoá khi `AddRole` lỗi; bản mới kiểm role trước khi ghi.
  - Fixture smoke: route platform assign-company còn đòi `admin.membership.invite` trong company của operator, nên cấp tạm cho CMS persona qua `qa_rw` (đã thu hồi).
  - Parity in-memory: `GetCompanyRoleByID` giờ ẩn role của company khác.
- **#7:**
  - Có 8 mã mới phải qua duyệt khi gỡ khỏi role hoặc gỡ trực tiếp: `admin.role.permission.assign` và `admin.role.permission.remove`, `ad_hoc_alert.process_control`, `company.ownership.transfer`, `disclosure_type.publish`, `workflow.step.override`, `template.workflow.override.reset`, `alert.channels.manage`.
  - FE coi 202 là thành công mà không báo "đang chờ duyệt". Hành vi này đã có cho các mã critical cũ; ghi vào PR-F.
- **#8:**
  - Guard xét theo kết quả:
    - Áp dụng khi người gọi không phải operator.
    - Tính tập quyền của member trước và sau thay đổi (role + quyền trực tiếp đang hiệu lực). Nếu trước là operator mà sau không còn → 403.
    - Áp dụng cho RemoveRole, RemoveDirectPermission, `PUT /primary-role` và lượt gửi duyệt direct-remove, nên lượt gỡ không còn vào hàng chờ duyệt.
    - Gỡ mà tư cách operator vẫn giữ được thì vẫn cho phép. Smoke kiểm bằng cách gỡ role `user_thuong` → 200.
  - Fixture: MEMBER được cấp tạm `platform.cms.view` + `system.settings` qua `qa_rw` (đã thu hồi; `provision_qa_company.py` trả lại role `user_thuong`).
  - Checks:
    - `go test ./...`: đúng 16 fail có sẵn như baseline.
    - `go vet ./...`: báo lỗi có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go:326-327` (copy `atomic.Int64`, file sửa lần cuối ở `2f1a784b`, ngoài diff). Các package khác vet sạch.
- **#9:**
  - DEV không có bảng `action_policy_matrix` (lỗi 1146), nên `legacyPolicy` là nguồn duy nhất quyết định action trên DEV.
  - Phạm vi ảnh hưởng:
    - Mọi action literal mà companyaccess, disclosure, workflow, notification, adhoc, deadlinealerts, portaldashboard truyền vào authorize đều đã có case, nên không route hiện tại nào đổi hành vi.
    - Đường còn hở là endpoint `/internal/v1/authorize` (+ `/batch`): bind theo token, nhận action bất kỳ.
  - Action không phải `admin.*` vẫn rơi về `system.settings` (giữ nguyên, ngoài phạm vi). Dòng matrix tường minh cho action `admin.*` chưa có case vẫn được giữ.
  - Nginx của portal (:3000) không proxy `/internal/*` (405). API :8080 thì vẫn mở trực tiếp ra ngoài, nên smoke gọi thẳng API origin (`E2E_API_BASE_URL` hoặc host:8080).
  - Parity in-memory: repo authz in-memory vẫn mặc định `system.settings` cho mọi action lạ (test dựa vào điều này). Thuộc nhóm RP-16, chưa đổi.
- **#10:**
  - Phạm vi thật rộng hơn "residual":
    - Quy tắc "còn ít nhất một admin" (ROLE-05) chỉ được kiểm ở RemoveRole và `PUT /primary-role`, và kiểm trước khi ghi mà không khoá.
    - Deactivate, delete, revoke-company-admin không kiểm gì cả.
    - Trên DEV có 11/21 company active không có primary admin đang active (gồm `c_001`, `c_002`), nên lỗi có thật.
  - Cách sửa:
    - Thêm `LockCompanyAdmins`: MySQL `GET_LOCK` trên connection riêng, chờ 5s, quá hạn thì 409; nếu release lỗi thì session bị bỏ khỏi pool. In-memory dùng mutex theo company.
    - Năm đường trên chạy bước kiểm và bước ghi trong lock. Deactivate/delete thêm kiểm admin cuối cùng.
    - Không có lời gọi lồng nhau. Pool 25 connection; trường hợp xấu nhất là chờ 5s rồi nhận 409, không deadlock.
  - **Sự cố khi tái hiện trên binary cũ (chỉ dữ liệu QA, đã khôi phục):**
    - Lần chạy đầu: smoke tiếp tục sau khi ca "deactivate" lọt guard, và ca "delete" đã **xoá cứng membership QA của ENT**. `provision_qa_company.py` khi đó chỉ UPDATE ENT nên không khôi phục được.
    - Đã khôi phục ENT với đúng membership id cũ (`01a12508-…`, grant trực tiếp mặc định vẫn còn) và kiểm lại: ENT là owner/primary, gọi admin API 200.
    - Cải tiến `provision_qa_company.py`:
      - tạo lại membership ENT nếu thiếu, theo id ghi trong env `E2E_QA_ENT_MEMBERSHIP_ID`;
      - thêm chế độ `--no-primary`;
      - xoá generation cache effective-access của QA company sau mỗi lần sync.
    - Smoke giờ chạy ca ít phá huỷ trước và dừng ngay ở guard đầu tiên bị lọt. Output lần đầu giữ ở `smoke_dev_role23.before-deploy.run1.out`.
  - **Lỗi riêng, có sẵn (chưa sửa):** `DELETE /api/v1/admin/company/admins/{id}` (và có thể cả `POST /company/admins`) tra role code `company_admin`, nhưng company tenant dùng `admin_doanh_nghiep`, nên route này trả **500** "company_admin role not found". Log API không ghi lại lỗi 500 cấp app này.
  - **Role-level:** đã xử lý ở vòng #11.
  - Integration test MySQL cho lock (`admin_lock_integration_test.go`) bị skip vì máy local không có MySQL. Hành vi trên MySQL thật đã được smoke race trên DEV kiểm.
- **#11:**
  - Các đường role-level có thể làm mất `rbac.manage`: gỡ quyền khỏi role tenant_custom, gỡ grant trực tiếp, rollback ma trận RBAC.
    - Cả ba đều thuộc nhóm critical nên luôn đi qua approval (`queueConfigApproval` → `ApproveConfigApproval` → `ApplyPendingApprovalInTx`).
    - InactivateCustomRole đã an toàn sẵn: role đang gán cho member thì không vô hiệu hoá được.
    - Role tenant_default/protected (như `admin_doanh_nghiep`) không bị restore đụng tới.
  - Cách sửa: `assertRBACPlanKeepsAdmin` tính lại, trên chính plan sẽ apply, xem từng member active còn `rbac.manage` không (qua role hoặc grant trực tiếp).
    - Gọi lúc gửi approval (báo sớm, 409).
    - Gọi lúc duyệt, giữ lock admin của company tới hết bước apply, nên không chạy xen được với một thay đổi membership (unit test race).
    - Company vốn đã không có admin thì không bị chặn thêm.
  - Fixture smoke: QA company `--no-primary`. MEMBER được cấp `rbac.manage` trực tiếp, ENT2 được cấp `system.settings` trực tiếp, qua `qa_rw` với `granted_by=qa_smoke_role23b`. ENT2 làm người duyệt, bị hạ xuống `user_thuong` nên không tính là admin. Đã dọn sạch: không còn grant fixture, không còn approval chờ.
  - Sửa thêm `provision_qa_company.py`: ENT giờ giữ **đúng** role `admin_doanh_nghiep` (trước đây không gỡ role thừa nên cleanup của smoke này fail trên binary cũ; đã chạy lại script để khôi phục).
  - Chưa phủ (ngoài phạm vi): grant tạm thời hết hạn (break-glass, delegation) không đi qua đường này.
  - Phát hiện phụ: `ValidateConfiguration` đếm admin theo role code `company_admin` nên company tenant luôn bị cảnh báo `business.admin.no_primary` (chỉ ở mức warning, cùng nhóm với lỗi 500 của `/admin/company/admins`).
