# Triển khai: một DNS, prefix `public/` + `admin/`, giảm rủi ro auth/authz

Tài liệu này là **bản mô tả từng bước** để chuẩn bị go-live, áp dụng quy tắc đã thống nhất (một host API, tách bề mặt user-facing vs CMS) và gắn với **giải pháp** cho các rủi ro đã rà trong luồng `cobo_web_design` + `cobo_iam_services`.

Ưu tiên soạn thảo/đổi code: tuân theo [docs/ai-cache/README.md](../cobo_web_design/docs/ai-cache/README.md) trong repo tương ứng (`cobo_web_design` / `cobo_iam_services`) — **hợp đồng rõ, validation/ủy quyền đầy đủ, diff nhỏ, tương thích ngược khi chưa yêu cầu breaking change, không bỏ trạng thái lỗi/empty trên web, bước cuối: pre-merge / kiểm tra theo `premerge-system-review`**.

> File này nằm dưới `cobo_iam_services/docs/`. Nếu cấu trúc thư mục là cùng thư mục cha với `cobo_web_design`, dùng đường dẫn tương đối: `../cobo_web_design/docs/ai-cache/README.md`. Nếu hai repo tách ổ đĩa, mở đúng file tương ứng trong từng clone.

**Phạm vi ngoài tài liệu này (không thay đổi theo bước triển khai gateway):** định nghĩa **role/permission** trong DB, `ActionPolicy`, `Resolver` / `Checker`, ma trận quyền menu trên `cobo_web_design`, hoặc quy tắc sản phẩm (“ai được bấm nút gì”). Tài liệu chỉ đề cập **định tuyến, vận hành, toàn vẹn ngữ cảnh gọi** và **bề mặt mạng**; mọi thay đổi tại tầng đó phải **vô hiệu hóa hành vi bypass** mà **không** sửa matrix RBAC, trừ khi một task riêng có tài liệu sản phẩm rõ ràng.

---

## 1. Mục tiêu thiết kế

| Mục | Nội dung |
|-----|----------|
| Một DNS | Một `server_name` cho API, ví dụ `https://api.example.com` |
| User app | Mọi request từ `cobo_web_design` dùng prefix **`/public`** (sau khi cấu hình nginx) |
| CMS (Internet) | Request CMS dùng prefix **`/admin`**, CORS/Origin tách, rate limit chặt hơn public |
| Không bề mặt tấn công thừa | **`/internal/**` không expose ra client browser**; auth nội bộ chỉ mesh/VPC hoặc policy nginx |

**Giữ tương thích:** backend Go hiện tại vẫn lắng nghe tại `*/api/v1/...` **trong container**; nginx **rewrite** hoặc strip prefix để không phải đổi hàng trăm route cùng lúc (làm theo pha ở mục 5–6).

---

## 2. Bảng rủi ro đã xác định và cách xử lý (phải tick khi go-live)

| Rủi ro (business/transport) | Cách xử lý mục tiêu (không đổi RBAC, không đổi policy) | Ghi chú tầng |
|--------|------------------------------------------------------|--------|
| A. `/internal/v1/authorize*`: nếu client không tin cậy có thể gửi `subject` trong **body** khác **ngữ cảnh JWT** → kết quả *decision* có thể tính trên **membership khác** so với token (lỗi toàn vẹn **người gọi**, không phải thiếu rule trong `Checker`) | **Ưu tiên P0, không cần đụng code nghiệp vụ phân quyền:** không public `/internal` tới Internet; chỉ mạng nội bộ / mesh, hoặc chặn ở nginx công cộng (Bảng G). | Edge / mạng |
| A (tuỳ chọn). Hardening thêm ở transport | **Nếu** cần gọi từ mạng rộng: handler chỉ nên **gán lại** `Subject` từ **claims** của access token (bỏ qua `user_id` / `membership_id` / `company_id` từ body) **trước khi** gọi `authapp.Service.Authorize`. Điều này **không** sửa `Check`, policy, `Resolver`, permission code — chỉ bảo đảm “đang hỏi dùm ai” khớp token. | `internal/authorization/transport/http` |
| B. Access token **opaque in-memory** — restart / **scale ngang** | **Vận hành:** `ACCESS_TOKEN_MODE=jwt` hoặc `dual` (cấu hình), **không** sửa bảng role/permission. | Cấu hình token, runbook |
| C. 401 hàng loạt → nhiều lần refresh, rotation hỏng | **Client:** gom (single-flight) request refresh. | `cobo_web_design/src/services/authApi.ts` |
| D. `localStorage` + XSS | Rủi ro lộ secret tại trình duyệt; giảm: CSP, v.v. (ngoài phạm vi chính sách phân quyền nghiệp vụ). | Sản phẩm / bảo mật ứng dụng |
| E. `RequirePermission` (web) **không** tương đương 403 ở API | **Bản chất:** hành vi sản phẩm: menu/route chỉ lọc UI. **Cấm** coi bước triển khai gateway là cơ hội **đổi** ma trận `routePermissionMatrix` nếu không có yêu cầu sản phẩm. Kiểm tra: API domain vẫn gọi `Authorize` như code hiện tại. | Tách: UX vs enforcement server (RBAC ở use case không đổi) |
| F. Login/restore thất bại, state client lẫn | Ổn định state (reset session khi lỗi); theo [ai-cache] (loading/error/empty). | `App.tsx` (không thay quy tắc sử dụng quyền) |
| G. `/internal` công cộng | Nginx/edge chặn hoặc tách mạng. | File nginx / firewall |

---

## 3. Chuỗi bước triển khai (theo thứ tự thực hiện)

### Bước 0 — Chuẩn bị tài khoản và tài liệu

- [ ] Xác định URL cuối: `API_PUBLIC_BASE=https://api.example.com/public`, `API_ADMIN_BASE=https://api.example.com/admin` (cùng host, khác path).
- [ ] Liệt kê Origin hợp lệ: SPA end-user, SPA CMS.
- [ ] Môi trường: `ACCESS_TOKEN_MODE` (nên `jwt`/`dual` nếu ≥2 replica API).
- [ ] MySQL/Redis/DSN đã có từ `docker-compose.dev.yml` chuyển lên môi trường thật theo `cobo_iam_services/README.md` (hoặc thủy trình nội bộ).

### Bước 1 — Nginx: một `server` cho API

1. Tạo file cấu hình (ví dụ) `conf.d/cobo-api.conf` trên host nginx.
2. Cấu hình TLS (`listen 443 ssl`), `client_max_body_size` phù hợp.
3. Thêm **`/public/`**:
   - `location /public/` với `proxy_pass` tới upstream API (cùng socket/port Go).
   - Cơ chế strip prefix: backend nhận `/api/v1/...` còn client gọi `https://api.../public/api/v1/...` — tùy chọn:
   - **Cách A (ít sửa app):** không strip; toàn bộ path tới app là `.../public/...` và ứng dụng Go mount thêm path prefix (đổi nhiều ở Go) **hoặc**
   - **Cách B (khuyến nghị):** `rewrite ^/public/(.*)$ /$1 break;` (hoặc tương đương) trước `proxy_pass` để phía app vẫn thấy `/api/v1/...`.
4. Thêm **`/admin/`** tương tự với bước rewrite, hoặc sau này trỏ upstream khác nếu tách dịch vụ CMS.
5. **Chặn `/internal`** trên cùng vhost công cộng (Bảng, mục G), trừ khi có listener riêng mạng nội bộ.
6. `limit_req` cho `location /admin/` (và tùy chọn endpoint auth sau rewrite, ví dụ cùng host dưới `/public/api/v1/auth/...`) mạnh hơn khu public — dùng **mẫu path ngoài edge** tương ứng cấu hình bạn chọn, không sửa logic auth trong app.
7. Gửi `proxy_set_header Host $host;`, `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Request-Id` nếu ứng dụng cần log.

**Kiểm tra sau bước 1:** `curl -k https://api.../public/healthz` (nếu health đặt dưới `/` sau rewrite) tương đương lệnh health hiện có; điều chỉnh path theo cách strip bạn chọn.

### Bước 2 — `cobo_iam_services` (hành vi & vận hành, **không** mở task sửa RBAC)

1. Bật biến môi trường **JWT** (hoặc tài liệu hóa sticky) theo bảng rủi ro B — thay *cách phát/kiểm tra* token, **không** đổi permission/role trên DB.
2. [ ] Bảng A **P0 (edge):** xác minh **không** có bản công cộng cho `/internal` tới Internet (Bước 1 + mục G). Nếu đáp ứng, **không bắt buộc** đổi code handler.
3. [ ] (Tuỳ chọn) Chỉ khi `/internal` **phải** tới từ mạng rộng hơn: xem cột A (tuỳ chọn) — sửa **chỉ** tầng transport, giữ nguyên `authapp.Service` + checker/policy.
4. Rà: audit, không log secret; theo [`.cursor/rules/cobo-iam-architecture.mdc`](../.cursor/rules/cobo-iam-architecture.mdc) nếu có trong clone (hoặc quy tắc tương đương).
5. [ ] `go test ./...` trước khi gắn image.

### Bước 3 — `cobo_web_design`

1. Cấu hình **`VITE_API_BASE_URL`**: dạng `https://api.example.com/public` nếu dùng cách B (rewrite) hoặc base đã bao hết prefix. **Không** cần sửa từng dòng nếu `baseUrl` + mọi path bắt đầu bằng `/api/v1/...` thành URL hợp: `https://.../public/api/v1/...` khi `base` kết thúc bằng `/public`.
2. (CMS sau này) biến riêng, ví dụ `VITE_CMS_API_BASE_URL` trỏ `.../admin`.
3. Rà theo bảng C, F: refresh single-flight; ổn định session khi lỗi; trạng thái theo [ai-cache]. **Không** mở rộng thay đổi `routePermissionMatrix` / catalog permission trong cùng PR triển khai gateway, trừ yêu cầu sản phẩm tách bản ghi thay đổi.
4. [ ] `npm run build` (và `npm test` nếu có) trước khi gắn bản.

### Bước 4 — Hợp đồng nội bộ / QA

- [ ] **Nếu** đã bật hardening A (tuỳ chọn) — gán `Subject` từ claims token: cập nhật tài liệu/QA (ví dụ `api-contracts`, `qa-test-matrix`) theo cách thử: **chủ yếu** `action` + `resource`; nếu body còn field `subject` thì server bỏ qua — **chỉ** cập nhật hợp đồng, **không** sửa mã permission hay policy.
- [ ] **Nếu** chỉ dùng lớp bảo vệ P0 (edge: `/internal` không tới Internet) và **chưa** sửa handler: giữ kịch bản gọi API/QA **theo bản hợp đồng hiện hành**; không mặc định rewrite ma trận thử theo cột A (tuỳ chọn).
- [ ] Regression: login, refresh, chọn công ty, `/me*` với base có `/public/`. Cùng user / cùng bối cảnh membership: mọi **200/403** phải **trùng hành vi** trước triển khai gateway (không đổi rule sản phẩm/permission trong cùng đợt này, trừ tài liệu PR riêng).

### Bước 5 — Cán mốc tương thích ngược

- Giữ song song: **có thể** để tạm route cũ (không prefix) proxy giống prefix trong giai đoạn chuyển; tắt khi 100% client đổi base.
- Tài liệu hóa kỳ **tắt** bản public “direct `/api`” trên cùng host công cộng.

### Bước 6 — Trước khi go-live (pre-merge theo dự án)

Theo [docs/ai-cache/README.md](../cobo_web_design/docs/ai-cache/README.md) (bản tương ứng trong từng repo): kiểm thử tối thiểu, điền checklist **premerge-system-review** — đặc biệt: validation, auth, contract JSON, hành vi 401/403, migration, gap test.

---

## 4. Ma trận xác minh nhanh (mẫu)

| Kiểm tra | Kỳ vọng |
|----------|--------|
| Gọi API từ SPA với base `.../public` | 200/401/403 hợp hợp đồng, không 404 do sai strip |
| Hai tab, nhiều 401, một refresh | Không tốn 4–5 lệnh refresh xung đột (C) |
| 2+ replica, không sticky | Dùng JWT, không 401 tại ngẫu nhiên (B) |
| `GET /.../public/...` tới `.../internal/...` | 404/444 ở edge (G) nếu không có internal vhost |
| Tác vụ cần quyền nghiệp vụ (server) | **Cùng** tài khoản / cùng token: 403/200 **trùng** môi trường cũ; UI có thể ẩn route nhưng API vẫn nguồn sự thật. **Không** xem bước này là cơ hội chỉnh ma trận `RequirePermission` hoặc permission DB (E) — chỉ hậu kiểm hồi quy |

---

## 5. Tham chiếu mã (trong từng clone)

- IAM handler: `cobo_iam_services/internal/iam/transport/http/`
- Authorize: `cobo_iam_services/internal/authorization/transport/http/`
- Cấu hình token: `cobo_iam_services/internal/httpserver/token_builder.go`
- Web: `cobo_web_design/src/services/authApi.ts`, `cobo_web_design/src/App.tsx` (session bootstrap / login)

---

## 6. Phiên bản tài liệu

- **0.1** — Bước triển khai + bảng rủi ro tích hợp với mô hình **một DNS, `/public` + `/admin`**, bám [docs/ai-cache/README.md] và bảo mật IAM.
- **0.2** — Rà soát phân biệt lỗi toàn vẹn **ngữ cảnh gọi** (transport/edge) vs nghiệp vụ **phân quyền** (RBAC không đổi trong phạm vi tài liệu); tách P0 mạng vs tùy chọn handler; bước 3–4/ma trận: không tự mở scope đổi `routePermissionMatrix` hay policy.
