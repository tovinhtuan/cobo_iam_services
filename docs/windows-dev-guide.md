# Huong dan chay `cobo_iam_services` tren Windows

> Muc tieu: thay the cac target `make` bang cac lenh chay duoc tren **Windows PowerShell**.
> Repo lien quan:
> - BE: `cobo_iam_services/`
> - FE: `../cobo_web_design/`
>
> **Khuyen nghi:** tren Windows, uu tien dung `docker compose` cho local dev. Khong can cai `make`.

---

## Tong quan 3 cach chay

### Cach 1 - Khuyen nghi nhat: Docker Compose local

Phu hop khi muon chay full stack nhanh tren Windows:

- MySQL
- Redis
- migrate
- API
- Worker
- Frontend Vite
- Mailpit

Lenh chinh:

```powershell
docker compose -f .\docker-compose.dev.yml up --build
```

### Cach 2 - Chay BE/FE truc tiep tren may

Phu hop khi ban muon debug rieng:

- Terminal 1: `go run ./cmd/api`
- Terminal 2: `go run ./cmd/worker`
- Terminal 3: `npm run dev` trong `cobo_web_design`

### Cach 3 - Deploy len dev server tu Windows

Phu hop khi ban muon thay the cac target `make deploy-*`, `make dev-*` bang `ssh`, `scp`, `go build`, `npm run build`.

Tai lieu nay co san bang mapping cuoi file.

---

## Buoc 0 - Cai dat can co tren Windows

Can co:

- Docker Desktop
- Go `1.22+`
- Node.js:
  - `18.x`, hoac
  - `20.x`, hoac
  - `22+`
- Git
- OpenSSH client (`ssh`, `scp`) neu can deploy len dev server

Kiem tra nhanh trong PowerShell:

```powershell
docker --version
docker compose version
go version
node -v
npm -v
ssh -V
scp -V
```

> `scp -V` co the khong in version tuy ban Windows, nhung neu lenh ton tai la du.

---

## Buoc 1 - Chay local full stack bang Docker Compose

### 1.1 Di chuyen vao thu muc backend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
```

### 1.2 Tao file `.env` neu can gui mail that

Mac dinh stack dung Mailpit, nen **khong bat buoc** tao `.env`.

Neu muon gui mail that, copy file mau:

```powershell
Copy-Item .\.env.example .\.env
```

Sau do sua `.env` voi SMTP that.

### 1.3 Start stack

```powershell
docker compose -f .\docker-compose.dev.yml up --build
```

Neu muon chay background:

```powershell
docker compose -f .\docker-compose.dev.yml up -d --build
```

### 1.4 Endpoint sau khi len

- API: `http://localhost:8080`
- Health: `http://localhost:8080/healthz`
- Readiness: `http://localhost:8080/readyz`
- Frontend: `http://localhost:3000`
- Mailpit: `http://localhost:8025`
- MySQL: `127.0.0.1:3306`

### 1.5 Kiem tra trang thai

```powershell
docker compose -f .\docker-compose.dev.yml ps
```

### 1.6 Xem log realtime

```powershell
docker compose -f .\docker-compose.dev.yml logs -f
```

Log rieng tung service:

```powershell
docker compose -f .\docker-compose.dev.yml logs -f api
docker compose -f .\docker-compose.dev.yml logs -f worker
docker compose -f .\docker-compose.dev.yml logs -f web
docker compose -f .\docker-compose.dev.yml logs -f migrate
```

### 1.7 Stop stack

```powershell
docker compose -f .\docker-compose.dev.yml down
```

Reset ca database volume:

```powershell
docker compose -f .\docker-compose.dev.yml down -v
```

> Tren PowerShell, `docker compose` co the in mot so status line ra stderr. Neu command exit code = `0` thi van coi la thanh cong.

---

## Buoc 2 - Chay truc tiep tren may Windows, khong qua Docker

Mode nay phu hop khi muon debug code nhanh. Ban can tu quan ly MySQL/Redis/Frontend.

## 2.1 API nhanh, khong can MySQL

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
go run .\cmd\api
```

Kiem tra:

```powershell
curl.exe http://127.0.0.1:8080/healthz
curl.exe http://127.0.0.1:8080/readyz
```

Expected:

- `/healthz` -> `200`
- `/readyz` -> `503`

> Day la mode bootstrap/in-memory, khong dung DB that.

## 2.2 API + Worker day du voi MySQL

### Tao DSN trong PowerShell

```powershell
$env:MYSQL_DSN = "cobo:cobo@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false"
$env:REDIS_ADDR = "127.0.0.1:6379"
$env:ACCESS_TOKEN_MODE = "opaque"
$env:PUBLIC_WEB_BASE_URL = "http://localhost:3000"
$env:PUBLIC_API_BASE_URL = "http://localhost:8080"
```

### Chay API

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
go run .\cmd\api
```

### Chay Worker o terminal khac

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
$env:MYSQL_DSN = "cobo:cobo@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false"
$env:PUBLIC_WEB_BASE_URL = "http://localhost:3000"
go run .\cmd\worker
```

### Kiem tra readiness

```powershell
curl.exe http://127.0.0.1:8080/readyz
```

Expected:

- `{"status":"ready"}`

## 2.3 Chay Frontend tren Windows

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
npm install
npm run dev
```

Frontend se len tai:

- `http://localhost:3000`

Build production:

```powershell
npm run build
```

Lint:

```powershell
npm run lint
```

Test:

```powershell
npm test
```

---

## Buoc 3 - Chay migration tren Windows

### Cach de nhat cho local dev

Dung container `migrate` co san trong Compose:

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
docker compose -f .\docker-compose.dev.yml run --rm migrate
```

Script nay se:

1. Tao database neu chua co
2. Tao `schema_migrations` neu chua co
3. Apply cac file trong `migrations/run_dev_migrations.sh`
4. Ghi nhan migration da apply

### Kiem tra migration da apply

```powershell
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SELECT file_name, executed_at FROM schema_migrations ORDER BY executed_at DESC LIMIT 20;"
```

### Chay 1 file SQL thu cong

```powershell
Get-Content .\migrations\0044_role_default_grant_permissions.up.sql | docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam
```

> Chi dung cach nay khi ban biet ro file do chua duoc apply. Cach an toan hon van la `docker compose ... run --rm migrate`.

---

## Buoc 4 - Build va test tren Windows khong dung `make`

### Backend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
go build .\...
go test .\...
```

Build binary Windows:

```powershell
go build -o .\bin\api.exe .\cmd\api
go build -o .\bin\worker.exe .\cmd\worker
```

Build binary Linux de deploy:

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o .\deploy-artifacts\backend\bin\api .\cmd\api
go build -o .\deploy-artifacts\backend\bin\worker .\cmd\worker
Remove-Item Env:CGO_ENABLED
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
```

### Frontend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
npm install
npm run lint
npm run build
npm test
```

> `npm run clean` hien tai dung `rm -rf dist`, khong portable voi PowerShell. Neu can xoa `dist` tren Windows, dung:

```powershell
Remove-Item .\dist -Recurse -Force -ErrorAction SilentlyContinue
```

---

## Buoc 5 - Deploy len dev server tu Windows, khong dung `make`

> Chi can phan nay neu ban muon thay cho `make deploy-be`, `make deploy-fe`, `make dev-ps`, `make dev-logs`...

Thong tin dev server hien tai:

- Host: `88.216.208.0`
- Port: `21239`
- User: `root`
- Path: `/root/cobo_project`

### 5.1 Test SSH

```powershell
ssh -p 21239 root@88.216.208.0 "echo OK && hostname"
```

### 5.2 Init server lan dau

Chay tu thu muc `cobo_iam_services`:

```powershell
scp -P 21239 .\docker-compose.artifacts.yml root@88.216.208.0:/root/cobo_project/docker-compose.artifacts.yml
ssh -p 21239 root@88.216.208.0 "mkdir -p /root/cobo_project/bin /root/cobo_project/configs /root/cobo_project/web /root/cobo_project/migrations"
scp -P 21239 -r .\migrations root@88.216.208.0:/root/cobo_project/
```

### 5.3 Deploy Backend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o .\deploy-artifacts\backend\bin\api .\cmd\api
go build -o .\deploy-artifacts\backend\bin\worker .\cmd\worker
Remove-Item Env:CGO_ENABLED
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

ssh -p 21239 root@88.216.208.0 "mkdir -p /root/cobo_project/bin /root/cobo_project/configs && rm -rf /root/cobo_project/bin/api /root/cobo_project/bin/worker /root/cobo_project/migrations"
scp -P 21239 .\deploy-artifacts\backend\bin\api root@88.216.208.0:/root/cobo_project/bin/.api.tmp
scp -P 21239 .\deploy-artifacts\backend\bin\worker root@88.216.208.0:/root/cobo_project/bin/.worker.tmp
scp -P 21239 -r .\deploy-artifacts\backend\configs root@88.216.208.0:/root/cobo_project/
scp -P 21239 -r .\migrations root@88.216.208.0:/root/cobo_project/
ssh -p 21239 root@88.216.208.0 "mv /root/cobo_project/bin/.api.tmp /root/cobo_project/bin/api && mv /root/cobo_project/bin/.worker.tmp /root/cobo_project/bin/worker && chmod 755 /root/cobo_project/bin/api /root/cobo_project/bin/worker && cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker"
```

### 5.4 Deploy Frontend

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
npm install
npm run build

Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
Remove-Item .\deploy-artifacts\web\dist -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item ..\cobo_web_design\dist .\deploy-artifacts\web\dist -Recurse
ssh -p 21239 root@88.216.208.0 "mkdir -p /root/cobo_project/web && rm -rf /root/cobo_project/web/dist && mkdir -p /root/cobo_project/web/dist"
scp -P 21239 -r .\deploy-artifacts\web\dist\* root@88.216.208.0:/root/cobo_project/web/dist/
scp -P 21239 .\deploy-artifacts\web\nginx.conf root@88.216.208.0:/root/cobo_project/web/nginx.conf
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml restart web"
```

### 5.5 Xem trang thai va log tren dev server

```powershell
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml ps"
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml logs -f"
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml restart"
ssh -p 21239 root@88.216.208.0
```

### 5.6 Health check sau deploy

```powershell
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/healthz && echo"
ssh -p 21239 root@88.216.208.0 "curl -sf http://localhost:8080/readyz && echo"
```

---

## Bang mapping `make` -> Windows PowerShell

| Make target | Lenh PowerShell tuong duong |
|---|---|
| `make be-build` | `go build -o .\bin\api.exe .\cmd\api` va `go build -o .\bin\worker.exe .\cmd\worker` |
| `make be-build-linux` | set `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, sau do `go build` vao `.\deploy-artifacts\backend\bin\` |
| `make be-run` | `go run .\cmd\api` |
| `make be-run-worker` | `go run .\cmd\worker` |
| `make be-test` | `go test .\...` |
| `make fe-install` | `Set-Location ..\cobo_web_design; npm install` |
| `make fe-dev` | `Set-Location ..\cobo_web_design; npm run dev` |
| `make fe-build` | `Set-Location ..\cobo_web_design; npm run build` |
| `make fe-test` | `Set-Location ..\cobo_web_design; npm test` |
| `make fe-clean` | `Remove-Item ..\cobo_web_design\dist -Recurse -Force -ErrorAction SilentlyContinue` |
| `make dc-up` | `docker compose -f .\docker-compose.dev.yml up -d` |
| `make dc-down` | `docker compose -f .\docker-compose.dev.yml down` |
| `make dc-build` | `docker compose -f .\docker-compose.dev.yml build` |
| `make dc-rebuild` | `docker compose -f .\docker-compose.dev.yml down`; `docker compose -f .\docker-compose.dev.yml build --no-cache`; `docker compose -f .\docker-compose.dev.yml up -d` |
| `make dc-logs` | `docker compose -f .\docker-compose.dev.yml logs -f` |
| `make dc-ps` | `docker compose -f .\docker-compose.dev.yml ps` |
| `make dc-restart` | `docker compose -f .\docker-compose.dev.yml restart` |
| `make deploy-init` | dung bo `scp` + `ssh mkdir -p` trong muc 5.2 |
| `make deploy-be` | dung chuoi lenh trong muc 5.3 |
| `make deploy-fe` | dung chuoi lenh trong muc 5.4 |
| `make deploy-all` | chay muc 5.3 roi 5.4 |
| `make push-migration FILE=...` | `scp` file SQL len server, roi `docker exec ... mysql` nhu script `deploy-artifacts/push-migration.sh` |
| `make dev-up` | `ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d"` |
| `make dev-down` | `ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml down"` |
| `make dev-ps` | `ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml ps"` |
| `make dev-logs` | `ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml logs -f"` |
| `make dev-restart` | `ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml restart"` |
| `make dev-ssh` | `ssh -p 21239 root@88.216.208.0` |

---

## Xu ly su co thuong gap tren Windows

### `make` khong ton tai

Ban bo qua `make`, dung truc tiep cac lenh trong tai lieu nay.

### `npm run build` loi quyen ghi `dist`

Neu truoc do `dist` bi tao boi user/container khac:

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_web_design
Remove-Item .\dist -Recurse -Force -ErrorAction SilentlyContinue
npm run build
```

### `curl` trong PowerShell hanh xu khac Linux

Tren Windows, uu tien dung `curl.exe` thay vi alias `curl` cua PowerShell:

```powershell
curl.exe http://127.0.0.1:8080/healthz
```

### Port `8080` hoac `3000` da bi chiem

Kiem tra:

```powershell
netstat -ano | Select-String ":8080"
netstat -ano | Select-String ":3000"
```

Neu bi chiem, dung service dang chiem port hoac doi port trong `docker-compose.dev.yml`.

### Frontend khong refresh dung sau khi pull code moi

Trong mode Docker Compose, co the can recreate web:

```powershell
Set-Location C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services
docker compose -f .\docker-compose.dev.yml up -d --force-recreate web
```

Neu van loi cache Vite:

```powershell
docker compose -f .\docker-compose.dev.yml exec web sh -c "rm -rf node_modules/.vite"
```

### `/readyz` tra `503`

Kiem tra:

- MySQL da len chua
- `MYSQL_DSN` da set chua
- migration da chay chua
- API va Worker co dang dung cung DSN khong

Lenh nhanh:

```powershell
docker compose -f .\docker-compose.dev.yml ps
docker compose -f .\docker-compose.dev.yml logs --tail=100 api
docker compose -f .\docker-compose.dev.yml run --rm migrate
```

---

## Lenh nen dung hang ngay tren Windows

### Start full stack

```powershell
docker compose -f .\docker-compose.dev.yml up -d --build
```

### Xem log API

```powershell
docker compose -f .\docker-compose.dev.yml logs -f api
```

### Chay lai migration

```powershell
docker compose -f .\docker-compose.dev.yml run --rm migrate
```

### Build backend

```powershell
go build .\...
```

### Build frontend

```powershell
Set-Location ..\cobo_web_design
npm run build
```

### Stop full stack

```powershell
Set-Location ..\cobo_iam_services
docker compose -f .\docker-compose.dev.yml down
```
