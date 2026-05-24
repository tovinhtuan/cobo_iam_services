# Deploy CMS Template Phase 1 to dev server 88.216.208.0
# Usage:
#   cd cobo_iam_services\deploy-artifacts
#   .\deploy-cms-template-phase1-dev.ps1
#   .\deploy-cms-template-phase1-dev.ps1 -MigrationsOnly
#   .\deploy-cms-template-phase1-dev.ps1 -SkipMigrations -SkipFE
#
# Docs: ..\docs\deploy-cms-template-phase1-88.216.208.0.md

param(
  [string]$DevHost = "88.216.208.0",
  [string]$DevPort = "21239",
  [string]$DevUser = "root",
  [string]$DevPath = "/root/cobo_project",
  [switch]$MigrationsOnly,
  [switch]$SkipMigrations,
  [switch]$SkipBE,
  [switch]$SkipFE,
  [switch]$SkipSmoke,
  [switch]$SkipRedisFlush
)

$ErrorActionPreference = "Stop"

$IamRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$FeRoot = (Resolve-Path (Join-Path $IamRoot "..\cobo_web_design")).Path
$Remote = "${DevUser}@${DevHost}"
$SshTarget = "-p $DevPort $Remote"

function Invoke-Remote([string]$Command) {
  & ssh -p $DevPort $Remote $Command
  if ($LASTEXITCODE -ne 0) { throw "Remote command failed: $Command" }
}

function Invoke-FlushEffectiveAccessCache {
  Write-Host "==> Flush Redis effective-access cache (cobo_iam:effective_access:*)" -ForegroundColor Cyan
  $flushCmd = @'
docker exec cobo-iam-redis sh -c 'keys=$(redis-cli KEYS "cobo_iam:effective_access:*"); if [ -n "$keys" ]; then echo "$keys" | xargs redis-cli DEL; fi; echo flush_done'
'@
  Invoke-Remote $flushCmd
}

function Require-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Missing '$Name'. Install OpenSSH Client (Windows Optional Features)."
  }
}

Require-Command ssh
Require-Command scp
Require-Command go

Write-Host "==> SSH test" -ForegroundColor Cyan
Invoke-Remote "echo OK && hostname"

$cmsMigrations = @(
  "0058_cms_template_permissions.up.sql",
  "0063_dev_platform_tenant_dual_admin.up.sql",
  "0069_template_display_groups_backfill.up.sql",
  "0070_prune_legacy_display_groups.up.sql",
  "0071_cms_template_write_from_platform_cms_view.up.sql",
  "0072_fix_display_groups_vietnamese_labels.up.sql"
)

if (-not $SkipMigrations) {
  Write-Host "==> Check schema_migrations on server" -ForegroundColor Cyan
  $inList = ($cmsMigrations | ForEach-Object { "'$_'" }) -join ","
  Invoke-Remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT file_name FROM schema_migrations WHERE file_name IN ($inList) ORDER BY file_name;`""

  Write-Host "==> Push CMS Phase 1 migrations (skip manually if already applied)" -ForegroundColor Cyan
  Push-Location $PSScriptRoot
  foreach ($file in $cmsMigrations) {
    Write-Host "    -> $file" -ForegroundColor DarkGray
    & .\push-migration.ps1 -File $file -DevHost $DevHost -DevPort $DevPort -DevUser $DevUser -DevPath $DevPath
  }
  Pop-Location

  Write-Host "==> Junction row count" -ForegroundColor Cyan
  Invoke-Remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT COUNT(*) AS junction_rows FROM template_display_groups;`""

  if (-not $SkipRedisFlush) {
    Invoke-FlushEffectiveAccessCache
  }
}

if ($MigrationsOnly) {
  Write-Host "Done (migrations only)." -ForegroundColor Green
  exit 0
}

if (-not $SkipBE) {
  Write-Host "==> Build Linux api/worker" -ForegroundColor Cyan
  Push-Location $IamRoot
  $env:CGO_ENABLED = "0"
  $env:GOOS = "linux"
  $env:GOARCH = "amd64"
  go build -o .\deploy-artifacts\backend\bin\api .\cmd\api
  go build -o .\deploy-artifacts\backend\bin\worker .\cmd\worker
  Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
  Pop-Location

  Write-Host "==> Upload BE + migrations, restart api/worker/migrate" -ForegroundColor Cyan
  Invoke-Remote "mkdir -p $DevPath/bin $DevPath/configs && rm -f $DevPath/bin/api $DevPath/bin/worker"
  & scp -P $DevPort (Join-Path $IamRoot "deploy-artifacts\backend\bin\api") "${Remote}:${DevPath}/bin/.api.tmp"
  & scp -P $DevPort (Join-Path $IamRoot "deploy-artifacts\backend\bin\worker") "${Remote}:${DevPath}/bin/.worker.tmp"
  & scp -P $DevPort -r (Join-Path $IamRoot "deploy-artifacts\backend\configs") "${Remote}:${DevPath}/"
  & scp -P $DevPort -r (Join-Path $IamRoot "migrations") "${Remote}:${DevPath}/"
  Invoke-Remote "mv $DevPath/bin/.api.tmp $DevPath/bin/api && mv $DevPath/bin/.worker.tmp $DevPath/bin/worker && chmod 755 $DevPath/bin/api $DevPath/bin/worker && cd $DevPath && docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker migrate"

  Write-Host "==> Health" -ForegroundColor Cyan
  Invoke-Remote "curl -sf http://localhost:8080/healthz && echo healthz_ok"
  Invoke-Remote "curl -sf http://localhost:8080/readyz && echo readyz_ok"

  if (-not $SkipRedisFlush) {
    Invoke-FlushEffectiveAccessCache
  }
}

if (-not $SkipFE) {
  if (-not (Test-Path $FeRoot)) {
    throw "Frontend repo not found at $FeRoot"
  }
  Write-Host "==> Build FE ($FeRoot)" -ForegroundColor Cyan
  Push-Location $FeRoot
  npm install
  npm run build
  Pop-Location

  $distSrc = Join-Path $FeRoot "dist"
  $distArtifact = Join-Path $IamRoot "deploy-artifacts\web\dist"
  Remove-Item $distArtifact -Recurse -Force -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Path $distArtifact -Force | Out-Null
  Copy-Item (Join-Path $distSrc "*") $distArtifact -Recurse

  Write-Host "==> Upload FE dist + restart web" -ForegroundColor Cyan
  Invoke-Remote "mkdir -p $DevPath/web/dist && rm -rf $DevPath/web/dist/*"
  & scp -P $DevPort -r (Join-Path $distArtifact "*") "${Remote}:${DevPath}/web/dist/"
  & scp -P $DevPort (Join-Path $IamRoot "deploy-artifacts\web\nginx.conf") "${Remote}:${DevPath}/web/nginx.conf"
  Invoke-Remote "cd $DevPath && docker compose -f docker-compose.artifacts.yml restart web"
}

if (-not $SkipSmoke) {
  $smokeScript = Join-Path $FeRoot "scripts\smoke-cms-template-display-groups.ps1"
  if (Test-Path $smokeScript) {
    Write-Host "==> API smoke (platform.tenant.admin)" -ForegroundColor Cyan
    Push-Location $FeRoot
    & $smokeScript -BaseUrl "http://${DevHost}:3000" -DisplayGroupCode display_groups_003 -AutoTypeIdFromName
    Pop-Location
  } else {
    Write-Warning "Smoke script not found: $smokeScript"
  }
}

Write-Host "`nDeploy complete. UI: http://${DevHost}:3000/cms/templates" -ForegroundColor Green
