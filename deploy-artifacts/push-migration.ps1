# Push a single migration to the dev server and apply it (Windows / PowerShell).
# Usage:
#   cd cobo_iam_services\deploy-artifacts
#   .\push-migration.ps1 -File 0063_dev_platform_tenant_dual_admin.up.sql
#
# Requires: OpenSSH client (scp, ssh) — Windows 10 optional feature "OpenSSH Client".

param(
  [Parameter(Mandatory = $true)]
  [string]$File,

  [string]$DevHost = "88.216.208.0",
  [string]$DevPort = "21239",
  [string]$DevUser = "root",
  [string]$DevPath = "/root/cobo_project"
)

$ErrorActionPreference = "Stop"

if ($File -match "'") {
  throw "Migration file name must not contain single quotes."
}

$migrationsDir = (Resolve-Path (Join-Path $PSScriptRoot "..\migrations")).Path
$localFile = Join-Path $migrationsDir $File
if (-not (Test-Path $localFile)) {
  throw "Migration not found: $localFile"
}

foreach ($cmd in @("scp", "ssh")) {
  if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
    throw "Missing '$cmd'. Install OpenSSH Client: Settings → Apps → Optional features → OpenSSH Client."
  }
}

$remote = "${DevUser}@${DevHost}"

Write-Host "==> Copying $File to dev server..."
& scp -P $DevPort $localFile "${remote}:$DevPath/migrations/"

Write-Host "==> Applying migration in MySQL container..."
Get-Content -Path $localFile -Raw -Encoding UTF8 | & ssh -p $DevPort $remote "docker exec -i cobo-iam-mysql mysql --default-character-set=utf8mb4 -uroot -proot cobo_iam"

Write-Host "==> Recording schema_migrations..."
$sqlTrack = "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('$File');"
& ssh -p $DevPort $remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e ""$sqlTrack"""

Write-Host "==> Verifying..."
$sqlVerify = "SELECT file_name, executed_at FROM schema_migrations WHERE file_name='$File';"
& ssh -p $DevPort $remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e ""$sqlVerify"""

Write-Host "Done: $File"
