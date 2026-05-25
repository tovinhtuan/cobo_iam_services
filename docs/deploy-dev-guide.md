# Hướng dẫn deploy lên môi trường dev

> **Môi trường dev:** `88.216.208.0:21239` — user `root` — path `/root/cobo_project`
> **Codebase:** `cobo_iam_services/` (BE) + `../cobo_web_design/` (FE)
> **Tất cả lệnh chạy từ thư mục `cobo_iam_services/`** (nơi chứa Makefile)

---

## Tổng quan flow

```
Máy local                          Dev server
─────────────────────────────────  ──────────────────────────────────
go build (Linux binary)      ───►  /root/cobo_project/bin/api
                                                        bin/worker
npm run build                ───►  /root/cobo_project/web/dist/
migrations/*.sql             ───►  /root/cobo_project/migrations/
                                   └── docker exec mysql apply SQL
SSH: docker compose restart  ───►  api / worker / web containers
```

---

## BƯỚC 0 — Kiểm tra SSH (làm trước mọi thứ)

### 0.1 Test kết nối

Mở terminal, chạy:

```bash
ssh -p 21239 root@88.216.208.0 "echo OK && hostname"
```

**Nếu ra `OK`** → SSH đã hoạt động, nhảy thẳng sang [Bước 1](#bước-1--kiểm-tra-code-trước-khi-deploy).

**Nếu ra `Permission denied (publickey,password)`** → Xem mục 0.2.

---

### 0.2 Fix SSH — Authorize key lên server

SSH đang fail vì public key của máy này chưa được thêm vào server.

**Bước A — Xem public key hiện có:**

```bash
cat ~/.ssh/id_rsa.pub
```

Copy toàn bộ dòng ra (bắt đầu bằng `ssh-rsa AAAA...`).

**Bước B — Login server bằng password (một lần) để thêm key:**

```bash
ssh -p 21239 -o PreferredAuthentications=password root@88.216.208.0
```

Khi vào được server, chạy lần lượt:

```bash
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# Paste public key vào đây (thay toàn bộ dòng bên dưới bằng nội dung từ Bước A)
echo "ssh-rsa AAAA...NỘI_DUNG_PUBLIC_KEY_CỦA_BẠN...== user@hostname" >> ~/.ssh/authorized_keys

chmod 600 ~/.ssh/authorized_keys
exit
```

**Bước C — Test lại:**

```bash
ssh -p 21239 root@88.216.208.0 "echo OK"
```

Phải ra `OK` trước khi tiếp tục.

> **Nếu server không cho login bằng password:**  
> Nhờ người quản lý server chạy lệnh sau trên server, paste nội dung `id_rsa.pub` vào:
> ```bash
> echo "ssh-rsa AAAA..." >> /root/.ssh/authorized_keys
> chmod 600 /root/.ssh/authorized_keys
> ```

---

### 0.3 (Tùy chọn) Thêm alias vào `~/.ssh/config` cho tiện

```
Host cobo-dev
    HostName 88.216.208.0
    Port 21239
    User root
    IdentityFile ~/.ssh/id_rsa
```

Sau đó có thể dùng `ssh cobo-dev` thay vì gõ đầy đủ mỗi lần.

---

## BƯỚC 1 — Kiểm tra code trước khi deploy

Từ `cobo_iam_services/`:

```bash
# Kiểm tra branch — phải đúng branch cần deploy
git branch
git log --oneline -3

# Kiểm tra không còn uncommitted changes
git status
```

Từ `../cobo_web_design/`:

```bash
git branch
git status
```

> Không cần commit trước khi deploy. Makefile build trực tiếp từ file local.  
> Tuy nhiên nên commit để tracking rõ ràng cái gì đang chạy trên dev.

---

## BƯỚC 2 — Deploy Backend (Go binaries)

> **Làm gì:** Cross-compile Go → Linux x86-64 binary → SCP lên server → restart container api + worker

Từ `cobo_iam_services/`:

```bash
make deploy-be
```

**Output mong đợi (các bước theo thứ tự):**
```
# 1. Cross-compile
GOOS=linux GOARCH=amd64 go build -o deploy-artifacts/backend/bin/api    ./cmd/api
GOOS=linux GOARCH=amd64 go build -o deploy-artifacts/backend/bin/worker ./cmd/worker

# 2. SCP binary + configs lên server
deploy-artifacts/backend/bin/api   → root@88.216.208.0:/root/cobo_project/bin/api
deploy-artifacts/backend/bin/worker → root@88.216.208.0:/root/cobo_project/bin/worker

# 3. Restart containers
docker compose up -d --force-recreate --no-deps api worker
```

**Nếu build thất bại** (`go build` error):
```bash
# Chạy thử local để xem lỗi rõ hơn
go build ./...
```

**Verify sau bước này:**
```bash
make dev-ps
# Kiểm tra cobo-iam-api và cobo-iam-worker đang running
```

---

## BƯỚC 3 — Deploy Frontend (React build)

> **Làm gì:** `npm run build` → copy dist → SCP lên server → restart nginx web container

Từ `cobo_iam_services/`:

```bash
make deploy-fe
```

**Output mong đợi:**
```
# 1. Build FE (từ ../cobo_web_design/)
cd ../cobo_web_design && npm run build
# → dist/ được tạo ra

# 2. Copy dist vào deploy-artifacts/web/dist/

# 3. SCP dist + nginx.conf lên server

# 4. Restart web container
docker compose restart web
```

**Nếu `npm run build` thất bại:**
```bash
cd ../cobo_web_design
npm install          # đảm bảo dependencies đủ
npm run lint         # kiểm tra TypeScript errors
npm run build        # xem lỗi cụ thể
```

**Verify sau bước này:**
```bash
# Kiểm tra web container đang running
make dev-ps
```

---

## BƯỚC 4 — Apply migrations mới

> **Làm gì:** Tìm migration nào chưa apply trên server → push file SQL → chạy trong MySQL container → ghi nhận vào schema_migrations

### Cách 1 — Tự động (dùng deploy-dev.sh)

```bash
sh deploy-dev.sh migrate
```

Script sẽ:
1. Query `schema_migrations` trên server để biết migration nào đã apply
2. So sánh với danh sách trong `migrations/run_dev_migrations.sh`
3. Push và apply từng migration chưa có

**Output mong đợi:**
```
==> Migrations: so sánh local vs server
    Phát hiện 2 migration chưa apply:
      - 0048_extend_titles.up.sql
      - 0049_seed_org_roles.up.sql
==> Đang push + apply 2 migration(s)...
    Applying: 0048_extend_titles.up.sql
    ✓ 0048_extend_titles.up.sql applied
    Applying: 0049_seed_org_roles.up.sql
    ✓ 0049_seed_org_roles.up.sql applied
    ✓ Tất cả migrations đã apply thành công
```

Nếu tất cả đã apply:
```
    ✓ Không có migration mới — server đã up to date
```

---

### Cách 2 — Push thủ công một file cụ thể

```bash
make push-migration FILE=0046_add_is_primary_admin.up.sql
```

Hoặc gọi script trực tiếp:

```bash
sh deploy-artifacts/push-migration.sh 0046_add_is_primary_admin.up.sql
```

**Xem migration nào đã apply trên server:**

```bash
ssh -p 21239 root@88.216.208.0 \
  "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam \
   -e 'SELECT file_name, executed_at FROM schema_migrations ORDER BY executed_at DESC LIMIT 10;'"
```

---

## BƯỚC 5 — Deploy tất cả cùng lúc (shortcut)

Nếu muốn làm BE + FE + migrations trong một lệnh:

```bash
sh deploy-dev.sh
# hoặc
sh deploy-dev.sh all
```

Script chạy lần lượt: pre-check → migrations → deploy-be → deploy-fe → verify.

Thêm `--skip-tests` để bỏ qua lint check (nhanh hơn ~30s):

```bash
sh deploy-dev.sh --skip-tests
```

---

## BƯỚC 6 — Verify sau deploy

### 6.1 Kiểm tra containers đang chạy

```bash
make dev-ps
```

**Output mong đợi:**
```
NAME                STATUS
cobo-iam-api        running
cobo-iam-worker     running
cobo-web-design     running   ← nginx serving FE
cobo-iam-mysql      running (healthy)
cobo-iam-redis      running (healthy)
```

### 6.2 Kiểm tra API health

```bash
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/healthz && echo"
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/readyz && echo"
```

**Output mong đợi:**
```
{"status":"ok"}
{"status":"ready"}
```

> Nếu chỉ thấy `{"status":"ok"}` nhưng không `ready` → API đang khởi động (chờ thêm 10–15s).

### 6.3 Xem log realtime (nếu cần debug)

```bash
make dev-logs
# Ctrl-C để thoát
```

Xem log riêng từng service:

```bash
ssh -p 21239 root@88.216.208.0 \
  "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml logs --tail=50 api"
```

### 6.4 Biến môi trường link email (reset password, invite, OTP)

| Biến | Giá trị trên dev server `88.216.208.0` |
|------|----------------------------------------|
| `PUBLIC_WEB_BASE_URL` | `http://88.216.208.0:3000` |
| `PUBLIC_API_BASE_URL` | `http://88.216.208.0:8080` |

Đã cấu hình trong `docker-compose.artifacts.yml` (service `api` và `worker`). Có thể override thêm trong `/root/cobo_project/.env` rồi recreate:

```bash
ssh -p 21239 root@88.216.208.0 \
  "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d --force-recreate api worker"
```

Sau `POST /api/v1/auth/forgot-password`, link trong mail phải là  
`http://88.216.208.0:3000/reset-password?token=...` — **không** dùng `localhost`.

### 6.5 Kiểm tra FE trên browser

Mở: **`http://88.216.208.0:3000`**

> Nếu không vào được: kiểm tra firewall server có mở port 3000 không.  
> API: **`http://88.216.208.0:8080`**

---

## Tóm tắt lệnh hay dùng

| Mục đích | Lệnh |
|---|---|
| Deploy đầy đủ (BE + FE + migrate) | `sh deploy-dev.sh` |
| Chỉ deploy BE | `make deploy-be` |
| Chỉ deploy FE | `make deploy-fe` |
| Chỉ apply migrations mới | `sh deploy-dev.sh migrate` |
| Push 1 migration cụ thể | `make push-migration FILE=0046_foo.up.sql` |
| Xem trạng thái containers | `make dev-ps` |
| Xem log realtime | `make dev-logs` |
| SSH vào server | `make dev-ssh` |
| Restart toàn bộ stack | `make dev-restart` |

---

## Xử lý sự cố thường gặp

### Container bị crash sau deploy

```bash
make dev-logs
# Tìm dòng ERROR hoặc panic
```

Restart thủ công:

```bash
ssh -p 21239 root@88.216.208.0 \
  "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml restart api"
```

### `make deploy-be` báo lỗi SCP

```
scp: Connection timed out
```

→ Kiểm tra SSH: `ssh -p 21239 root@88.216.208.0 "echo ok"`

### `npm run build` báo TypeScript error

```bash
cd ../cobo_web_design
npm run lint    # xem lỗi cụ thể
```

### Migration apply thất bại

Xem lỗi chi tiết:

```bash
ssh -p 21239 root@88.216.208.0 \
  "docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam 2>&1" \
  < migrations/TÊN_FILE.up.sql
```

### API không start sau deploy BE (DB migration thiếu)

Nếu log báo `column not found` hoặc `table doesn't exist` → chạy lại:

```bash
sh deploy-dev.sh migrate
```

---

## Lần đầu tiên (server chưa có gì)

Nếu đây là lần đầu deploy lên server mới:

```bash
# 1. Khởi tạo thư mục + copy docker-compose lên server
make deploy-init

# 2. SSH vào server, tạo file .env từ template
make dev-ssh
# Trên server:
cd /root/cobo_project
cp configs/config.example.env .env
# Chỉnh sửa .env nếu cần (SMTP, custom domains...)
exit

# 3. Khởi động stack lần đầu
make dev-up

# 4. Deploy code
make deploy-all

# 5. Apply toàn bộ migrations
sh deploy-dev.sh migrate
```

pass: uNf5pfg1Pu7etvp

> **Deploy với password (không dùng SSH key):** cần `sshpass` trên máy local.
>
> ```bash
> export SSHPASS="$(grep '^pass:' docs/deploy-dev-guide.md | sed 's/^pass:[[:space:]]*//')"
> sh deploy-dev.sh
> ```
>
> Hoặc: `SSHPASS='...' make deploy-all`