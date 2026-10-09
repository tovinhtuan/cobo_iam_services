# ROLE-01 follow-up plan (2026-10-09)

Nguồn: mục "Chưa làm" trong `05-completion.md` và phát hiện của vòng review (be-security BES-03/04, admin-role ROLE-14/15/16, api-compat API-03/06).

## Phát hiện mới khi lập plan (read-only DEV)
- **0/20 công ty có thành viên active giữ `system.settings`**; không có role active nào mang quyền này. 13/20 công ty có người giữ `rbac.manage`. `self_reg_company_owner` chỉ có `rbac.manage`.
- Người duyệt approval cấu hình phải có `system.settings` (`authorizeConfigApprovalDecide`). Vì vậy **mọi approval cấu hình hiện không ai duyệt được**: gỡ quyền critical khỏi role, gỡ quyền trực tiếp, sửa kênh cảnh báo, và nay là rollback critical. DEV có các approval `pending` từ tháng 7 không được xử lý.
- Cùng gốc với ROLE-04: `AssignCompanyAdmin`/`RevokeCompanyAdmin`/`TransferOwnership` dùng action `rbac.manage`, nhưng `legacyPolicy` không có case nên rơi về `system.settings`. Chủ công ty không dùng được các route này.

## Workstreams

| WS | Nội dung | Phụ thuộc | Cần quyết định |
|---|---|---|---|
| A | Ai được duyệt approval cấu hình, cộng ROLE-04 | quyết định của user | **Có** |
| B | Tính plan trong transaction (TOCTOU), lưu trạng thái thực sau apply, chụp phiên bản khi tạo/nhân bản/vô hiệu role | C (để verify SQL) | Một điểm nhỏ (B3) |
| C | Test tích hợp MySQL thật cho restore và apply approval (thay cho smoke nhánh duyệt trên DEV) | — | Không |
| D | Approval không còn tác dụng (do binary cũ tạo hoặc đã lỗi thời): chặn duyệt no-op, hiển thị thay đổi thật cho người duyệt, dọn dữ liệu pending | B | Ghi dữ liệu DEV/prod |
| E | FE: nhãn `rbac.matrix.rollback` trong ApprovalsPanel | — | Không |

Thứ tự đề xuất: **C → B → D**, **A** chạy song song ngay khi có quyết định, **E** độc lập (PR web riêng).

---

## WS-A: Mô hình người duyệt và ROLE-04 (chờ quyết định)

### Phương án người duyệt
| Phương án | Mô tả | Ưu | Nhược |
|---|---|---|---|
| **A1 (khuyến nghị)** | Người duyệt = thành viên **khác** người yêu cầu, có `rbac.manage` **hoặc** `system.settings`, cùng company | 13/20 công ty duyệt được ngay; giữ nguyên 4-eyes; không mở rộng quyền | Công ty chỉ có 1 admin vẫn không duyệt được |
| A2 | Cấp `system.settings` cho `self_reg_company_owner`/`admin_doanh_nghiep` qua migration | Không đổi code duyệt | Mở rộng quyền rộng: `system.settings` là mặc định của `legacyPolicy` cho mọi action không khai báo, nên sẽ mở nhiều route khác |
| A3 | Platform operator duyệt qua route CMS | Giải quyết công ty 1 admin | Cần route CMS mới; operator can thiệp cấu hình tenant |

**Công ty chỉ có 1 người quản trị** (kết hợp với A1), chọn một:
- (a) cho phép primary admin tự duyệt sau thời gian chờ (ví dụ 24h), có audit;
- (b) chuyển cho platform operator (A3);
- (c) giữ nguyên: phải mời thêm admin.

### Tasks (sau khi chốt A1)
- **A-T1 (test trước):** người duyệt có `rbac.manage` (khác người yêu cầu) duyệt được; người yêu cầu vẫn 403 `SELF_APPROVAL_NOT_ALLOWED`; người chỉ có quyền khác 403. Chạy cho cả 3 loại change hiện có và `rbac.matrix.rollback`.
- **A-T2:** `authorizeConfigApprovalDecide` chấp nhận `rbac.manage` OR `system.settings`. Kiểm `RejectConfigApproval` dùng cùng cổng.
- **A-T3 (ROLE-04):** `AssignCompanyAdmin`, `RevokeCompanyAdmin`, `TransferOwnership` dùng `requireRbacManage` thay vì `authorize(..., "rbac.manage")`. Test persona chỉ có `rbac.manage` chạy được; persona chỉ có `system.settings` bị 403.
- **A-T4 FE:** `ApprovalsPanel.tsx` đang dùng `useGuard(['system.settings'])` cho nút duyệt; đổi sang `['rbac.manage','system.settings']` (PR web, gộp với WS-E).
- **A-T5 hợp đồng:** `api-contracts-json.md` (người duyệt), `qa-test-matrix.csv`.
- AC: approval duyệt được bởi admin thứ hai; không có đường tự duyệt; test cũ không nới khẳng định.

---

## WS-B: Tính đúng trong transaction

### B-T1 Truy vấn đọc theo transaction
- Thêm interface nội bộ `queryer` (`QueryContext`, `QueryRowContext`, `ExecContext`) trong `infra/mysql`; `*sql.DB` và `*sql.Tx` đều thoả.
- Tách các truy vấn đọc cần cho restore thành hàm nhận `queryer`: danh sách role của company, quyền của role, catalog quyền, quyền trực tiếp active theo company, dựng snapshot. Method public hiện có gọi lại với `r.db`, không đổi hành vi.
- Đồng thời xử lý PERF-15 (đọc qua pool khi đang giữ transaction).

### B-T2 Khoá và tính plan trong transaction
- Trong `RestoreRBACMatrixFromSnapshot` và `restoreRBACMatrixInTx`:
  - khoá `SELECT role_id FROM roles WHERE company_id=? AND role_type='tenant_custom' AND is_protected=0 AND status='active' FOR UPDATE`;
  - khoá `SELECT id FROM membership_direct_permissions WHERE company_id=? AND revoked_at IS NULL FOR UPDATE`;
  - đọc trạng thái **qua tx** rồi tính plan.
- Thứ tự khoá cố định: roles → direct grants, để tránh deadlock giữa hai restore.

### B-T3 Kiểm lại critical trong transaction (đổi chữ ký repo)
- `RestoreRBACMatrixFromSnapshot(ctx, companyID, actor, raw, opts RBACRestoreOptions{AllowCritical bool})`, cập nhật cả MySQL và in-memory.
- Rollback trực tiếp gọi với `AllowCritical=false`. Nếu plan trong tx có critical (do thay đổi chen giữa pre-check và restore), repo rollback tx và trả lỗi sentinel; service chuyển sang approval (202). Apply approval gọi với `AllowCritical=true`.
- **Quyết định nhỏ:** khi mismatch, (khuyến nghị) tự chuyển sang approval 202, hay trả 409 để client thử lại.
- Test: decorator repo in-memory chèn một quyền critical ngay trước restore, kỳ vọng 202 và dữ liệu không đổi.

### B-T4 Lưu trạng thái thực sau apply approval
- `ApplyPendingApprovalInTx` dựng snapshot từ DB **trong tx** sau khi apply, lọc phạm vi doanh nghiệp, rồi lưu thay cho `ProposedSnapshotJSON`.
- Catalog quyền đọc qua tx; dùng hàm lọc của `caapp`.
- Test (in-memory + tích hợp WS-C): phiên bản sau apply bằng trạng thái thực, không chứa mục role bị bỏ qua.

### B-T5 Chụp phiên bản khi tạo, nhân bản, vô hiệu role tùy chỉnh
- `CreateCustomRole`, `CloneRole`, `InactivateCustomRole` gọi `captureRBACMatrixVersion` sau khi thành công.
- Approval/rollback cũ thành stale (409) thay vì gỡ sạch quyền của role mới.
- Test: tạo role sau khi xếp approval → duyệt trả 409 `STALE_PROPOSAL`.

AC chung: Gate R (test đỏ trước), revert-check từng guard, `go test ./...` so với baseline, `-race`, test tích hợp WS-C xanh.

---

## WS-C: Test tích hợp MySQL thật (thay cho nhánh duyệt chưa chạy trên DEV)

- **C-T1:** `internal/companyaccess/infra/mysql/rbac_restore_integration_test.go`, theo mẫu có sẵn (`MYSQL_TEST_DSN`, skip nếu không có DB).
  - Seed dữ liệu với id ngẫu nhiên: company tạm, role global giả, role `tenant_default` mang `platform.cms.view`, role custom, membership, quyền trực tiếp (grantable và không grantable).
  - Dọn bằng `t.Cleanup`.
- **C-T2 case:**
  - restore xoá/thêm quyền role custom;
  - role global/mặc định và quyền platform giữ nguyên;
  - quyền trực tiếp: cấp/thu hồi chỉ mã grantable và chỉ membership của company; không tạo dòng trùng;
  - `ApplyPendingApprovalInTx` cho `rbac.matrix.rollback` và `rbac.permission.remove` (chạy thật `restoreRBACMatrixInTx`);
  - sau WS-B: snapshot sau apply = trạng thái thực; mismatch critical trong tx.
- **C-T3:** hướng dẫn chạy local:
  - `docker compose -f docker-compose.dev.yml up -d mysql`;
  - chạy `migrations/run_dev_migrations.sh`; lưu ý H18: 4 migration chưa có trong danh sách, kiểm bảng cần dùng;
  - `MYSQL_TEST_DSN=... go test ./internal/companyaccess/infra/mysql/ -run Integration`.
- **C-T4 (tuỳ chọn, sau WS-A):** smoke DEV nhánh duyệt thật khi đã có người duyệt hợp lệ (admin thứ hai có `rbac.manage`); cần duyệt ghi DEV.
- AC: các case chạy xanh trên MySQL 8 local; CI/dev không có DB thì skip, không fail.

---

## WS-D: Approval không còn tác dụng

- **D-T1 (test trước):** `ApproveConfigApproval` cho `rbac_matrix`: tính tác động của bản đề xuất so với trạng thái hiện tại (cùng hàm plan). Nếu không có thay đổi nào trong phạm vi được phép → 409 `APPROVAL_NOTHING_TO_APPLY` (mã mới), giữ `pending` để người dùng huỷ/từ chối. Không còn "approved nhưng không làm gì".
- **D-T2:** `CompareConfigApproval` và summary của approval `rbac_matrix` trả danh sách thay đổi thật (role, quyền, thêm/gỡ; quyền trực tiếp) từ plan, thay vì chỉ số đếm; người duyệt thấy đúng thứ sẽ áp dụng.
- **D-T3 dữ liệu:**
  - query read-only liệt kê approval `pending` của `rbac_matrix` mà plan rỗng hoặc đã lỗi thời (DEV hiện có 1 `rbac.direct_permission.remove` ở `144ca32b…` và các approval khác từ tháng 7);
  - sau khi user duyệt ghi, huỷ/từ chối bằng API (không sửa SQL tay);
  - prod: chạy cùng query trước khi deploy bản mới.
- **D-T4 hợp đồng:** mã lỗi mới, dạng `changes` trong compare (cộng FE nếu hiển thị).

---

## WS-E: FE (cobo_web_design, PR riêng)
- **E-T1:** `ApprovalsPanel.tsx` `changeTypeLabel`: thêm `rbac.matrix.rollback` → "Khôi phục ma trận phân quyền (critical)"; test trong `ApprovalsPanel.test.tsx`.
- **E-T2 (cùng A-T4):** guard nút duyệt theo mô hình WS-A.
- **E-T3 (sau D-T2):** hiển thị danh sách thay đổi trong màn compare.
- Verify: `npm run lint`, `npm test` (focused), `npm run build`, `npm run check:mojibake`.

---

## Ngoài phạm vi (ghi nhận)
- Repo `AddRolePermission`/`RemoveRolePermission` chưa tự kiểm `company_id`/`role_type` (hardening, gộp PR-A của risk review).
- `MaxBytesReader` cho handler `companyaccess` (chung).
- Cache quyền chưa invalidate sau các thay đổi khác (H17), membership inactive vẫn giữ quyền (H3), là PR-B của risk review.

## Quyết định cần user
1. **Mô hình người duyệt:** A1 (khuyến nghị) / A2 / A3; và công ty chỉ có 1 admin: (a) tự duyệt sau thời gian chờ / (b) platform operator / (c) giữ nguyên.
2. **Gộp ROLE-04 vào WS-A** (khuyến nghị: có, cùng gốc "system.settings không ai có").
3. **B-T3** khi phát hiện critical trong transaction: tự chuyển sang approval 202 (khuyến nghị) hay 409.
4. ~~D-T3: cho phép huỷ/từ chối approval pending lỗi thời trên DEV~~ → **đã chốt (user, 2026-10-09):** DEV chỉ là dữ liệu mẫu, được phép sửa. Áp dụng cho D-T3, C-T4 (smoke nhánh duyệt) và dọn dư lượng smoke; vẫn ưu tiên thao tác qua API và ghi lại trong ai-cache.

## Quyết định đã chốt (user, 2026-10-09)
1. **Người duyệt: A1.** Thành viên khác người yêu cầu, cùng company, có `rbac.manage` **hoặc** `system.settings`.
2. **Công ty chỉ có 1 admin: giữ nguyên (c).** Không có đường tự duyệt; approval chờ tới khi có admin thứ hai. Ghi rõ trong hợp đồng và (nếu cần) thông báo trên FE.
3. **ROLE-04 gộp vào WS-A** (A-T3).
4. **B-T3: mismatch trong transaction → tự chuyển sang approval 202.**
5. **DEV là dữ liệu mẫu:** được phép ghi/dọn (D-T3, C-T4, dư lượng smoke).

WS-A không còn chờ quyết định; mọi workstream có thể bắt đầu.
