# Deploy CMS Template Phase 1 → Dev `88.216.208.0`

**Host:** `88.216.208.0` · **SSH:** `21239` · **User:** `root` · **Path:** `/root/cobo_project`  
**Tham chiếu:** [`windows-dev-guide.md`](./windows-dev-guide.md) §5

**Mục tiêu:** Migration quyền CMS + backfill `template_display_groups`, deploy BE (`display_group_codes`, gate D5), deploy FE (picker display groups).

---

## 0. Điều kiện trên máy Windows

- OpenSSH Client (`ssh`, `scp`)
- Go 1.22+, Node 18+
- Repo đã có code Phase 1 (BE + FE) trên nhánh đang deploy

```powershell
# Test SSH
ssh -p 21239 root@88.216.208.0 "echo OK && hostname && cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml ps"
```

---

## 1. Kiểm tra migration đã apply (trên server)

```powershell
ssh -p 21239 root@88.216.208.0 @"
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e \"
SELECT file_name FROM schema_migrations
WHERE file_name IN (
  '0053_cms_portal_template_tables.up.sql',
  '0054_cms_display_groups_po_seed.up.sql',
  '0058_cms_template_permissions.up.sql',
  '0063_dev_platform_tenant_dual_admin.up.sql',
  '0069_template_display_groups_backfill.up.sql'
) ORDER BY file_name;
\""
```

| File | Vai trò Phase 1 |
|------|-----------------|
| `0053` | Bảng `template_display_groups` |
| `0054` | Seed 7 nhóm PO (`display_groups_001`…`007`) |
| `0058` | Permission `cms.template.*` |
| `0063` | `platform.tenant.admin` + `cms.template.write` |
| `0069` | Backfill junction từ `display_group_code` |
| `0070` | Xóa legacy display groups; chỉ giữ `display_groups_001`…`007` |
| `0071` | `platform.cms.view` → `cms.template.write|activate|archive` (+ default grants) |
| `0072` | Sửa label tiếng Việt `disclosure_display_groups` (Mojibake + PO labels 001..007) |

Nếu thiếu `0053`–`0058`: chạy full stack migrate (mục 1b) hoặc `push-migration` từng file theo thứ tự số.

### 1b. (Tùy chọn) Đồng bộ cả thư mục migrations + chạy container migrate

Dùng sau khi đã `scp` migrations mới (gồm `0069` trong `run_dev_migrations.sh`):

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
scp -P 21239 -r .\migrations root@88.216.208.0:/root/cobo_project/
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d migrate"
ssh -p 21239 root@88.216.208.0 "docker logs cobo-iam-migrate --tail 80"
```

Migrate lỗi → `deploy-artifacts\show-migrate-logs.ps1` (xem §5.7 windows-dev-guide).

---

## 2. Apply migration thiếu (khuyến nghị — từng file)

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services\deploy-artifacts

# Chỉ chạy file CHƯA có trong schema_migrations (bước 1)
.\push-migration.ps1 -File 0058_cms_template_permissions.up.sql
.\push-migration.ps1 -File 0063_dev_platform_tenant_dual_admin.up.sql
.\push-migration.ps1 -File 0069_template_display_groups_backfill.up.sql
.\push-migration.ps1 -File 0070_prune_legacy_display_groups.up.sql
.\push-migration.ps1 -File 0071_cms_template_write_from_platform_cms_view.up.sql
```

### 2.2 Flush effective-access cache (sau 0071 / đổi quyền)

Script deploy gọi tự động. Thủ công:

```powershell
ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-redis sh -c 'keys=\$(redis-cli KEYS \"cobo_iam:effective_access:*\"); if [ -n \"\$keys\" ]; then echo \"\$keys\" | xargs redis-cli DEL; fi; echo flush_done'"
```

Bỏ qua trong script: `.\deploy-cms-template-phase1-dev.ps1 -SkipRedisFlush`

### 2.1 Verify sau 0063 (quyền ghi template)

```powershell
ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT login_id FROM users WHERE user_id='u_platform_tenant_admin';`""
ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT COUNT(*) AS cms_write_grants FROM role_permissions rp JOIN permissions p ON p.permission_id=rp.permission_id WHERE p.permission_code='cms.template.write';`""
```

### 2.2 Verify sau 0069 (junction)

```powershell
ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT COUNT(*) AS junction_rows FROM template_display_groups;`""
```

---

## 3. Deploy Backend (Linux binary + restart api/worker/migrate)

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services

$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o .\deploy-artifacts\backend\bin\api .\cmd\api
go build -o .\deploy-artifacts\backend\bin\worker .\cmd\worker
Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue

ssh -p 21239 root@88.216.208.0 "mkdir -p /root/cobo_project/bin /root/cobo_project/configs && rm -f /root/cobo_project/bin/api /root/cobo_project/bin/worker"
scp -P 21239 .\deploy-artifacts\backend\bin\api root@88.216.208.0:/root/cobo_project/bin/.api.tmp
scp -P 21239 .\deploy-artifacts\backend\bin\worker root@88.216.208.0:/root/cobo_project/bin/.worker.tmp
scp -P 21239 -r .\deploy-artifacts\backend\configs root@88.216.208.0:/root/cobo_project/
scp -P 21239 -r .\migrations root@88.216.208.0:/root/cobo_project/

ssh -p 21239 root@88.216.208.0 "mv /root/cobo_project/bin/.api.tmp /root/cobo_project/bin/api && mv /root/cobo_project/bin/.worker.tmp /root/cobo_project/bin/worker && chmod 755 /root/cobo_project/bin/api /root/cobo_project/bin/worker && cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker migrate"
```

### 3.1 Health

```powershell
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/healthz; echo"
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/readyz; echo"
```

---

## 4. Deploy Frontend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
npm install
npm run lint
npm test
npm run build

Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
Remove-Item .\deploy-artifacts\web\dist -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path .\deploy-artifacts\web\dist -Force | Out-Null
Copy-Item ..\cobo_web_design\dist\* .\deploy-artifacts\web\dist\ -Recurse

ssh -p 21239 root@88.216.208.0 "mkdir -p /root/cobo_project/web/dist && rm -rf /root/cobo_project/web/dist/*"
scp -P 21239 -r .\deploy-artifacts\web\dist\* root@88.216.208.0:/root/cobo_project/web/dist/
scp -P 21239 .\deploy-artifacts\web\nginx.conf root@88.216.208.0:/root/cobo_project/web/nginx.conf
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml restart web"
```

---

## 5. Smoke sau deploy

### 5.1 API (PowerShell)

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
.\scripts\smoke-cms-template-display-groups.ps1 -BaseUrl http://88.216.208.0:3000 -DisplayGroupCode display_groups_003
```

**Pass:** Login → catalog → PUT template → round-trip `display_group_codes` → Portal filter.

### 5.2 UI thủ công

| URL | Kiểm tra |
|-----|----------|
| http://88.216.208.0:3000/login | `platform.tenant.admin@example.com` / `secret` |
| http://88.216.208.0:3000/cms/templates | Chip nhóm Portal, lưu template |
| http://88.216.208.0:3000/app/disclosure-types | Lọc chip, `has_workflow` |

---

## 6. Rollback (khẩn cấp)

**FE:** Deploy lại bản `web/dist` backup trên server (nếu có).  
**BE:** Khôi phục binary `api`/`worker` cũ từ backup.  
**DB:** Chỉ down migration khi chắc chắn — `0069` có file `.down.sql`; **không** down `0063` trên môi trường đã dùng.

```powershell
# Chỉ khi cần revert backfill junction (hiếm)
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services\deploy-artifacts
# Thủ công: apply 0069_template_display_groups_backfill.down.sql qua mysql exec, rồi xóa dòng schema_migrations
```

---

## 7. One-shot script (tùy chọn)

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services\deploy-artifacts
.\deploy-cms-template-phase1-dev.ps1
# Hoặc chỉ migration:
.\deploy-cms-template-phase1-dev.ps1 -MigrationsOnly
```

---

**Thứ tự an toàn:** `migration (0058→0063→0069→0070→0071)` → `flush Redis` → `BE` → `flush Redis` → `FE` → `smoke` → `UI`.
