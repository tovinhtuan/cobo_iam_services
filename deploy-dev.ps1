#Requires -Version 5.1
<#
.SYNOPSIS
  Deploy Cobo IAM (BE + FE + migrations) từ Windows 10 lên môi trường dev.

.DESCRIPTION
  Tương đương deploy-dev.sh / make deploy-dev. Dùng OpenSSH (scp, ssh) có sẵn trên Windows 10.

.PARAMETER Mode
  all | be | fe | migrate | verify

.PARAMETER SkipTests
  Bỏ qua go build ./... và npm run lint trước deploy.

.EXAMPLE
  cd cobo_iam_services
  .\deploy-dev.ps1

.EXAMPLE
  .\deploy-dev.ps1 -Mode be -SkipTests

.NOTES
  Cấu hình: copy deploy-dev.local.env.example → deploy-dev.local.env
  SSH: nên dùng key; password nhập tương tác khi ssh/scp hỏi (không lưu trong repo).
#>

[CmdletBinding()]
param(
  [ValidateSet('all', 'be', 'fe', 'migrate', 'verify')]
  [string] $Mode = 'all',

  [switch] $SkipTests
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ── Paths ─────────────────────────────────────────────────────────────────────
$ScriptDir = $PSScriptRoot
$ArtifactsDir = Join-Path $ScriptDir 'deploy-artifacts'
$BackendBinDir = Join-Path $ArtifactsDir 'backend\bin'
$MigrationsDir = Join-Path $ScriptDir 'migrations'
$FeDir = (Resolve-Path (Join-Path $ScriptDir '..\cobo_web_design')).Path
$PushMigrationPs1 = Join-Path $ArtifactsDir 'push-migration.ps1'
$RunMigrationsSh = Join-Path $MigrationsDir 'run_dev_migrations.sh'

# ── Defaults (override bằng deploy-dev.local.env) ─────────────────────────────
$DevHost = '88.216.208.0'
$DevPort = '21239'
$DevUser = 'root'
$DevPath = '/root/cobo_project'
$DevSshIdentity = $env:DEV_SSH_IDENTITY_FILE
$DevSshPassword = $null
$UsePlink = $false
$PlinkExe = 'C:\Program Files\PuTTY\plink.exe'
$PscpExe = 'C:\Program Files\PuTTY\pscp.exe'

function Import-DeployDevLocalEnv {
  $envFile = Join-Path $ScriptDir 'deploy-dev.local.env'
  if (-not (Test-Path $envFile)) { return }

  Write-Host "    Loading $envFile"
  Get-Content $envFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    if ($line -match '^\s*([^#=]+)=(.*)$') {
      $name = $Matches[1].Trim()
      $value = $Matches[2].Trim().Trim('"').Trim("'")
      Set-Item -Path "Env:$name" -Value $value
    }
  }

  if ($env:DEV_HOST) { $script:DevHost = $env:DEV_HOST }
  if ($env:DEV_PORT) { $script:DevPort = $env:DEV_PORT }
  if ($env:DEV_USER) { $script:DevUser = $env:DEV_USER }
  if ($env:DEV_PATH) { $script:DevPath = $env:DEV_PATH }
  if ($env:DEV_SSH_IDENTITY_FILE) { $script:DevSshIdentity = $env:DEV_SSH_IDENTITY_FILE }
  if ($env:DEV_SSH_PASSWORD) { $script:DevSshPassword = $env:DEV_SSH_PASSWORD }
  elseif ($env:SSHPASS) { $script:DevSshPassword = $env:SSHPASS }
}

function Register-PlinkHostKey {
  param([string]$Target)
  & $PlinkExe -batch -P $DevPort -pw $DevSshPassword $Target 'echo ok' 2>$null | Out-Null
  if ($LASTEXITCODE -eq 0) { return }
  Write-Host '    Caching SSH host key (first connect)...'
  $prevEap = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  $null = cmd /c "echo y| `"$PlinkExe`" -P $DevPort -pw `"$DevSshPassword`" $Target exit" 2>&1
  $ErrorActionPreference = $prevEap
  & $PlinkExe -batch -P $DevPort -pw $DevSshPassword $Target 'echo ok' 2>$null | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw 'Failed to cache PuTTY host key. Run: plink -P 21239 root@88.216.208.0 exit'
  }
}

function Initialize-SshTransport {
  if ($DevSshPassword -and (Test-Path $PlinkExe) -and (Test-Path $PscpExe)) {
    $script:UsePlink = $true
    Write-Host '    SSH transport: PuTTY plink/pscp (password)'
    Register-PlinkHostKey -Target "${DevUser}@${DevHost}"
    return
  }
  if ($DevSshPassword) {
    throw 'DEV_SSH_PASSWORD set but PuTTY plink/pscp not found. Install PuTTY or use DEV_SSH_IDENTITY_FILE.'
  }
  Write-Host '    SSH transport: OpenSSH (key or interactive password)'
}

function Write-Step([string]$Message) {
  Write-Host ""
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Ok([string]$Message) {
  Write-Host "    OK $Message" -ForegroundColor Green
}

function Write-WarnMsg([string]$Message) {
  Write-Host "    WARN $Message" -ForegroundColor Yellow
}

function Write-Err([string]$Message) {
  Write-Host "    FAIL $Message" -ForegroundColor Red
}

function Assert-Command([string]$Name, [string]$Hint) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Missing '$Name'. $Hint"
  }
}

function Get-SshArgs {
  $args = @('-p', $DevPort, '-o', 'StrictHostKeyChecking=accept-new')
  if ($DevSshIdentity -and (Test-Path $DevSshIdentity)) {
    $args += @('-i', $DevSshIdentity)
  }
  return $args
}

function Invoke-DevSsh {
  param([Parameter(Mandatory)][string]$RemoteCommand)

  $target = "${DevUser}@${DevHost}"
  # Docker/compose often writes warnings to stderr; with $ErrorActionPreference=Stop,
  # PowerShell treats that as terminating even when ssh exit code is 0.
  $prevEap = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    if ($UsePlink) {
      & $PlinkExe -batch -P $DevPort -pw $DevSshPassword $target $RemoteCommand
    } else {
      $sshArgs = @(Get-SshArgs) + @($target, $RemoteCommand)
      & ssh @sshArgs
    }
    $code = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $prevEap
  }
  if ($code -ne 0) {
    throw "SSH failed (exit $code): $RemoteCommand"
  }
}

function Invoke-DevScp {
  param(
    [switch]$Recursive,
    [Parameter(Mandatory)][string]$Source,
    [Parameter(Mandatory)][string]$Destination
  )

  if ($UsePlink) {
    $pscpArgs = @('-batch', '-P', $DevPort, '-pw', $DevSshPassword)
    if ($Recursive) { $pscpArgs += '-r' }
    & $PscpExe @pscpArgs $Source $Destination
  } else {
    $base = @('-P', $DevPort, '-o', 'StrictHostKeyChecking=accept-new')
    if ($DevSshIdentity -and (Test-Path $DevSshIdentity)) {
      $base += @('-i', $DevSshIdentity)
    }
    if ($Recursive) { $base += '-r' }
    & scp @base $Source $Destination
  }
  if ($LASTEXITCODE -ne 0) {
    throw "SCP failed (exit $LASTEXITCODE): $Source -> $Destination"
  }
}

function Test-DevSsh {
  Write-Step 'Pre-flight: SSH connectivity'
  Write-Host "    Target: ${DevUser}@${DevHost}:${DevPort} -> $DevPath"
  if ($DevSshIdentity) {
    Write-Host "    SSH key: $DevSshIdentity"
  } else {
    Write-Host "    SSH key: (default) or password via DEV_SSH_PASSWORD"
  }

  try {
    Invoke-DevSsh 'echo ok' | Out-Null
  } catch {
    Write-Err 'SSH connection failed'
    Write-Host "    Thử: ssh -p $DevPort ${DevUser}@${DevHost}"
    Write-Host '    Or set DEV_SSH_IDENTITY_FILE / DEV_SSH_PASSWORD in deploy-dev.local.env'
    throw
  }
  Write-Ok "SSH OK"
}

function Test-GoToolchain {
  Assert-Command 'go' 'Install Go: https://go.dev/dl/'
}

function Test-NodeToolchain {
  Assert-Command 'node' 'Install Node.js LTS: https://nodejs.org/'
  Assert-Command 'npm' 'npm ships with Node.js'
  $major = [int](node -p "process.versions.node.split('.')[0]")
  $ok = ($major -ge 18 -and $major -lt 19) -or ($major -ge 20 -and $major -lt 21) -or ($major -ge 22)
  if (-not $ok) {
    throw "FE build needs Node ^18 || ^20 || >=22. Current: $(node -v)"
  }
}

function Invoke-PrecheckBe {
  if ($SkipTests) {
    Write-WarnMsg 'Skipping BE pre-check (-SkipTests)'
    return
  }
  Write-Step 'BE pre-check: go build ./...'
  Push-Location $ScriptDir
  try {
    & go build ./...
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
  } finally {
    Pop-Location
  }
  Write-Ok 'go build OK'
}

function Invoke-PrecheckFe {
  if ($SkipTests) {
    Write-WarnMsg 'Skipping FE pre-check (-SkipTests)'
    return
  }
  Write-Step 'FE pre-check: npm run lint'
  Push-Location $FeDir
  try {
    & npm run lint
    if ($LASTEXITCODE -ne 0) { throw 'npm run lint failed' }
  } finally {
    Pop-Location
  }
  Write-Ok 'npm run lint OK'
}

function Get-LocalMigrationFiles {
  if (-not (Test-Path $RunMigrationsSh)) {
    throw "Missing $RunMigrationsSh"
  }
  $raw = Get-Content $RunMigrationsSh -Raw
  if ($raw -notmatch 'MIGRATIONS="([\s\S]*?)"') {
    throw "Could not parse MIGRATIONS list in run_dev_migrations.sh"
  }
  $block = $Matches[1]
  return $block -split "`r?`n" |
    ForEach-Object { $_.Trim() } |
    Where-Object { $_ -match '\.sql$' }
}

function Sync-MigrationsDir {
  Write-Step 'Migrations: sync migrations folder to dev server'
  $remote = "${DevUser}@${DevHost}"
  Invoke-DevSsh "mkdir -p $DevPath; rm -rf ${DevPath}/migrations"
  Invoke-DevScp -Recursive -Source $MigrationsDir -Destination "${remote}:${DevPath}/"
  Write-Ok 'migrations/ synced'
}

function Get-AppliedMigrations {
  $cmd = "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse 'SELECT file_name FROM schema_migrations'"
  $out = Invoke-DevSshCapture $cmd
  return @($out | Where-Object { $_ -match '\.sql$' })
}

function Invoke-DevSshCapture {
  param([Parameter(Mandatory)][string]$RemoteCommand)
  $target = "${DevUser}@${DevHost}"
  if ($UsePlink) {
    return @(& $PlinkExe -batch -P $DevPort -pw $DevSshPassword $target $RemoteCommand)
  }
  return @(& ssh @(Get-SshArgs) $target $RemoteCommand)
}

function Invoke-DeployMigrations {
  Write-Step 'Migrations: compare local vs server'
  Sync-MigrationsDir

  $applied = @()
  try {
    $applied = Get-AppliedMigrations
  } catch {
    $msg = $_.Exception.Message
    if ($msg -match "schema_migrations' doesn't exist") {
      Write-WarnMsg 'schema_migrations missing; running bootstrap migrate on server'
      Invoke-DevSsh "cd $DevPath; docker compose -f docker-compose.artifacts.yml run --rm migrate"
      $applied = Get-AppliedMigrations
    } else {
      Write-Err $msg
      throw
    }
  }

  $localList = Get-LocalMigrationFiles
  $newFiles = @($localList | Where-Object { $applied -notcontains $_ })

  if ($newFiles.Count -eq 0) {
    Write-Ok 'No new migrations; server up to date'
    return
  }

  Write-Host "    Found $($newFiles.Count) pending migration(s):"
  $newFiles | ForEach-Object { Write-Host "      - $_" }

  if (-not (Test-Path $PushMigrationPs1)) {
    throw "Missing $PushMigrationPs1"
  }

  foreach ($f in $newFiles) {
    Write-Host "    Applying: $f"
    if ($DevSshPassword) { $env:DEV_SSH_PASSWORD = $DevSshPassword }
    & $PushMigrationPs1 -File $f -DevHost $DevHost -DevPort $DevPort -DevUser $DevUser -DevPath $DevPath
    if ($LASTEXITCODE -ne 0) {
      throw "Migration failed: $f"
    }
    Write-Ok "$f applied"
  }
  Write-Ok 'All migrations applied'
}

function Invoke-DeployBe {
  Write-Step 'Deploy BE: build Linux binary + SCP + restart api/worker'
  Test-GoToolchain

  New-Item -ItemType Directory -Force -Path $BackendBinDir | Out-Null

  Push-Location $ScriptDir
  try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'

    & go build -o (Join-Path $BackendBinDir 'api') ./cmd/api
    if ($LASTEXITCODE -ne 0) { throw 'go build api failed' }
    & go build -o (Join-Path $BackendBinDir 'worker') ./cmd/worker
    if ($LASTEXITCODE -ne 0) { throw 'go build worker failed' }
  } finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
  }

  $remote = "${DevUser}@${DevHost}"
  $apiLocal = Join-Path $BackendBinDir 'api'
  $workerLocal = Join-Path $BackendBinDir 'worker'
  $configsLocal = Join-Path $ArtifactsDir 'backend\configs'
  $rsaLocal = Join-Path $ScriptDir 'configs\login_password_rsa_dev.pem'

  Invoke-DevSsh "mkdir -p ${DevPath}/bin ${DevPath}/configs; rm -rf ${DevPath}/bin/api ${DevPath}/bin/worker ${DevPath}/migrations"

  Invoke-DevScp -Source $apiLocal -Destination "${remote}:${DevPath}/bin/.api.tmp"
  Invoke-DevScp -Source $workerLocal -Destination "${remote}:${DevPath}/bin/.worker.tmp"
  Invoke-DevScp -Recursive -Source $configsLocal -Destination "${remote}:${DevPath}/"
  if (Test-Path $rsaLocal) {
    Invoke-DevScp -Source $rsaLocal -Destination "${remote}:${DevPath}/configs/login_password_rsa_dev.pem"
  }
  Invoke-DevScp -Recursive -Source $MigrationsDir -Destination "${remote}:${DevPath}/"

  Invoke-DevSsh "mv ${DevPath}/bin/.api.tmp ${DevPath}/bin/api; mv ${DevPath}/bin/.worker.tmp ${DevPath}/bin/worker; chmod 755 ${DevPath}/bin/api ${DevPath}/bin/worker; cd ${DevPath}; docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker"

  Write-Ok 'BE deployed'
}

function Invoke-DeployFe {
  Write-Step 'Deploy FE: Vite flag preflight + npm run build + SCP dist + restart web'
  Test-NodeToolchain

  # DEV FE hardening — mirror scripts/ensure-dev-fe-vite-flags.sh (DEV-only path).
  if (-not $env:VITE_PERSONAL_OPS_V2) { $env:VITE_PERSONAL_OPS_V2 = 'true' }
  if (-not $env:VITE_DASHBOARD_OPERATIONAL_V2) { $env:VITE_DASHBOARD_OPERATIONAL_V2 = 'true' }
  if (-not $env:VITE_DASHBOARD_OVERVIEW_API_ENABLED) { $env:VITE_DASHBOARD_OVERVIEW_API_ENABLED = 'true' }
  Write-Host "[deploy-dev][fe] VITE_PERSONAL_OPS_V2=$($env:VITE_PERSONAL_OPS_V2)"
  Write-Host "[deploy-dev][fe] VITE_DASHBOARD_OPERATIONAL_V2=$($env:VITE_DASHBOARD_OPERATIONAL_V2)"
  Write-Host "[deploy-dev][fe] VITE_DASHBOARD_OVERVIEW_API_ENABLED=$($env:VITE_DASHBOARD_OVERVIEW_API_ENABLED)"
  if ($env:ALLOW_LEGACY_DEV_FLAGS -ne 'true') {
    if ($env:VITE_PERSONAL_OPS_V2 -ne 'true' -or $env:VITE_DASHBOARD_OPERATIONAL_V2 -ne 'true' -or $env:VITE_DASHBOARD_OVERVIEW_API_ENABLED -ne 'true') {
      throw 'DEV FE Vite flags must be true (or set ALLOW_LEGACY_DEV_FLAGS=true for intentional legacy rollback)'
    }
  } else {
    Write-WarnMsg 'ALLOW_LEGACY_DEV_FLAGS=true — DEV may compile Legacy Profile'
  }

  Push-Location $FeDir
  try {
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'npm run build failed' }
  } finally {
    Pop-Location
  }

  $distSrc = Join-Path $FeDir 'dist'
  if (-not (Test-Path $distSrc)) {
    throw "Missing $distSrc after build"
  }

  $artifactsWebDist = Join-Path $ArtifactsDir 'web\dist'
  if (Test-Path $artifactsWebDist) {
    Remove-Item $artifactsWebDist -Recurse -Force
  }
  New-Item -ItemType Directory -Force -Path $artifactsWebDist | Out-Null
  Copy-Item -Path (Join-Path $distSrc '*') -Destination $artifactsWebDist -Recurse -Force

  $remote = "${DevUser}@${DevHost}"
  $nginxLocal = Join-Path $ArtifactsDir 'web\nginx.conf'
  $fixPermsLocal = Join-Path $ScriptDir 'scripts\fix-dev-web-perms.sh'

  # Stop web before replacing bind-mount source (avoids stale empty mount until recreate).
  Invoke-DevSsh "cd ${DevPath}; docker compose -f docker-compose.artifacts.yml stop web 2>/dev/null || true"
  Invoke-DevSsh "mkdir -p ${DevPath}/web; rm -rf ${DevPath}/web/dist; mkdir -p ${DevPath}/web/dist"
  Invoke-DevScp -Recursive -Source (Join-Path $artifactsWebDist '*') -Destination "${remote}:${DevPath}/web/dist/"

  Invoke-DevScp -Source $nginxLocal -Destination "${remote}:${DevPath}/web/nginx.conf"
  if (Test-Path $fixPermsLocal) {
    try {
      Invoke-DevScp -Source $fixPermsLocal -Destination "${remote}:${DevPath}/fix-dev-web-perms.sh"
      Invoke-DevSsh "sed -i 's/\r$//' ${DevPath}/fix-dev-web-perms.sh; chmod +x ${DevPath}/fix-dev-web-perms.sh; sh ${DevPath}/fix-dev-web-perms.sh ${DevPath}"
    } catch {
      Write-WarnMsg 'fix-dev-web-perms.sh failed; force-recreating web container'
      Invoke-DevSsh "cd ${DevPath}; find web/dist -type d -exec chmod 755 {} \; ; find web/dist -type f -exec chmod 644 {} \; ; docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps web"
    }
  } else {
    Invoke-DevSsh "cd ${DevPath}; find web/dist -type d -exec chmod 755 {} \; ; find web/dist -type f -exec chmod 644 {} \; ; docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps web"
  }

  Write-Ok 'FE deployed'
}

function Invoke-Verify {
  Write-Step 'Verify: containers on dev server'
  try {
    Invoke-DevSsh "cd $DevPath; docker compose -f docker-compose.artifacts.yml ps"
  } catch {
    Write-WarnMsg 'dev-ps failed'
  }

  Write-Step 'Verify: /healthz and /readyz'
  $healthz = ''
  try {
    $healthz = (Invoke-DevSshCapture 'curl -sf http://localhost:8080/healthz 2>/dev/null') -join ''
  } catch { $healthz = 'FAIL' }

  if ($healthz -match 'ok') {
    Write-Ok '/healthz ok'
  } else {
    Write-WarnMsg "/healthz: $healthz"
  }

  $readyz = ''
  try {
    $readyz = (Invoke-DevSshCapture 'curl -sf http://localhost:8080/readyz 2>/dev/null') -join ''
  } catch { $readyz = 'FAIL' }

  if ($readyz -match 'ready') {
    Write-Ok '/readyz ready'
  } else {
    Write-WarnMsg "/readyz: $readyz"
  }

  Write-Host ""
  Write-Host "    Portal: http://${DevHost}:3000"
  Write-Host "    API:    http://${DevHost}:8080"
}

# ── Main ──────────────────────────────────────────────────────────────────────
Import-DeployDevLocalEnv
Initialize-SshTransport

if (-not $UsePlink) {
  Assert-Command 'ssh' 'Install OpenSSH Client (Settings -> Apps -> Optional features)'
  Assert-Command 'scp' 'Install OpenSSH Client (scp bundled)'
}

Write-Host ""
Write-Host 'deploy-dev.ps1 - Cobo Dev Deploy (Windows)' -ForegroundColor White
Write-Host "Mode: $Mode   SkipTests: $($SkipTests.IsPresent)"
Write-Host "Target: ${DevUser}@${DevHost}:${DevPort} -> $DevPath"
Write-Host ""

switch ($Mode) {
  'be' {
    Test-DevSsh
    Invoke-PrecheckBe
    Invoke-DeployBe
    Invoke-Verify
  }
  'fe' {
    Test-DevSsh
    Invoke-PrecheckFe
    Invoke-DeployFe
    Invoke-Verify
  }
  'migrate' {
    Test-DevSsh
    Invoke-DeployMigrations
    Invoke-Verify
  }
  'verify' {
    Test-DevSsh
    Invoke-Verify
  }
  default {
    Test-DevSsh
    Invoke-PrecheckBe
    Invoke-PrecheckFe
    Invoke-DeployMigrations
    Invoke-DeployBe
    Invoke-DeployFe
    Invoke-Verify
  }
}

Write-Host ""
Write-Host '==> Deploy complete.' -ForegroundColor Green
Write-Host "    SSH: ssh -p $DevPort ${DevUser}@${DevHost}"
Write-Host ""
