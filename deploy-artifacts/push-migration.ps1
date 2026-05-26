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

$plink = 'C:\Program Files\PuTTY\plink.exe'
$pscp = 'C:\Program Files\PuTTY\pscp.exe'
$pw = $env:DEV_SSH_PASSWORD
if (-not $pw) { $pw = $env:SSHPASS }
$usePlink = $pw -and (Test-Path $plink) -and (Test-Path $pscp)

if (-not $usePlink) {
  foreach ($cmd in @('scp', 'ssh')) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
      throw "Missing '$cmd'. Install OpenSSH Client or set DEV_SSH_PASSWORD with PuTTY."
    }
  }
}

$remote = "${DevUser}@${DevHost}"
$remoteDest = "${remote}:$DevPath/migrations/"

Write-Host "==> Copying $File to dev server..."
if ($usePlink) {
  & $pscp -batch -P $DevPort -pw $pw $localFile $remoteDest
} else {
  & scp -P $DevPort $localFile $remoteDest
}
if ($LASTEXITCODE -ne 0) { throw "SCP failed for $File" }

Write-Host "==> Applying migration in MySQL container..."
$remoteFile = "$DevPath/migrations/$File"
$applyCmd = "cat $remoteFile | docker exec -i cobo-iam-mysql mysql --default-character-set=utf8mb4 -uroot -proot cobo_iam"
if ($usePlink) {
  & $plink -batch -P $DevPort -pw $pw $remote $applyCmd
} else {
  & ssh -p $DevPort $remote $applyCmd
}
if ($LASTEXITCODE -ne 0) { throw "Failed to apply migration $File" }

Write-Host "==> Recording schema_migrations..."
$sqlTrack = "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('$File');"
if ($usePlink) {
  $sqlTrack | & $plink -batch -P $DevPort -pw $pw $remote "docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam"
} else {
  $sqlTrack | & ssh -p $DevPort $remote "docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam"
}
if ($LASTEXITCODE -ne 0) { throw "Failed to record schema_migrations for $File" }

Write-Host "==> Verifying..."
$sqlVerify = "SELECT file_name, executed_at FROM schema_migrations WHERE file_name='$File';"
if ($usePlink) {
  & $plink -batch -P $DevPort -pw $pw $remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e ""$sqlVerify"""
} else {
  & ssh -p $DevPort $remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e ""$sqlVerify"""
}

Write-Host "Done: $File"
