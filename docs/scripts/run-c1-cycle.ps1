param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$ComposeFile = "docker-compose.dev.yml",
  [string]$UserLogin = "user@example.com",
  [string]$UserPassword = "secret",
  [string]$AdminLogin = "admin.dn@example.com",
  [string]$AdminPassword = "secret",
  [string]$CmsLogin = "cms.operator@example.com",
  [string]$CmsPassword = "secret",
  [string]$UserCompanyId = "c_001"
)

$ErrorActionPreference = "Stop"

function Run-Step {
  param(
    [string]$Title,
    [scriptblock]$Action
  )
  Write-Host ""
  Write-Host "=== $Title ===" -ForegroundColor Cyan
  & $Action
}

function Invoke-Checked {
  param(
    [string]$Command
  )
  Write-Host ">> $Command"
  Invoke-Expression $Command
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed (exit $LASTEXITCODE): $Command"
  }
}

function Wait-ApiReady {
  param(
    [string]$ApiBaseUrl,
    [int]$MaxAttempts = 30,
    [int]$SleepSeconds = 2
  )
  Write-Host ">> waiting for API readiness at $ApiBaseUrl"
  for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
    try {
      $health = Invoke-WebRequest -Method GET -Uri "$ApiBaseUrl/healthz" -TimeoutSec 5
      $ready = Invoke-WebRequest -Method GET -Uri "$ApiBaseUrl/readyz" -TimeoutSec 5
      if ($health.StatusCode -eq 200 -and $ready.StatusCode -eq 200) {
        Write-Host ">> API ready (attempt $attempt/$MaxAttempts)"
        return
      }
    }
    catch {
      Write-Host ">> API not ready yet (attempt $attempt/$MaxAttempts): $($_.Exception.Message)"
    }
    Start-Sleep -Seconds $SleepSeconds
  }
  throw "API readiness timeout after $MaxAttempts attempts: $ApiBaseUrl"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$iamRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$webRoot = Resolve-Path (Join-Path $iamRoot "..\cobo_web_design")

Run-Step "1/6 Backend disclosure integration tests" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "go test ./internal/httpserver -run disclosureC1 -v"
    Invoke-Checked "go test ./..."
  }
  finally {
    Pop-Location
  }
}

Run-Step "2/6 Frontend contract tests + lint" {
  Push-Location $webRoot
  try {
    Invoke-Checked "npm run lint"
    Invoke-Checked "npm run test -- src/services/normalizers.disclosure.test.ts src/services/disclosureApi.contract.test.ts"
  }
  finally {
    Pop-Location
  }
}

Run-Step "3/6 Restart compose services before DB smoke" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile up -d api web"
    Wait-ApiReady -ApiBaseUrl $BaseUrl
    Invoke-Checked "docker compose -f $ComposeFile ps"
  }
  finally {
    Pop-Location
  }
}

Run-Step "4/7 DB-mode disclosure C1 smoke" {
  Push-Location $iamRoot
  try {
    $smokeScript = Join-Path $scriptDir "disclosure-c1-db-smoke.ps1"
    Invoke-Checked "powershell -ExecutionPolicy Bypass -File `"$smokeScript`" -BaseUrl `"$BaseUrl`" -UserLogin `"$UserLogin`" -UserPassword `"$UserPassword`" -AdminLogin `"$AdminLogin`" -AdminPassword `"$AdminPassword`" -UserCompanyId `"$UserCompanyId`""
  }
  finally {
    Pop-Location
  }
}

Run-Step "5/7 DB-mode CMS prefix smoke" {
  Push-Location $iamRoot
  try {
    $cmsSmokeScript = Join-Path $scriptDir "cms-core-prefix-smoke.ps1"
    Invoke-Checked "powershell -ExecutionPolicy Bypass -File `"$cmsSmokeScript`" -BaseUrl `"$BaseUrl`" -UserLogin `"$UserLogin`" -UserPassword `"$UserPassword`" -CmsLogin `"$CmsLogin`" -CmsPassword `"$CmsPassword`" -UserCompanyId `"$UserCompanyId`""
  }
  finally {
    Pop-Location
  }
}

Run-Step "6/7 Fresh no-cache Docker rebuild" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile build --no-cache api web"
  }
  finally {
    Pop-Location
  }
}

Run-Step "7/7 Bring up rebuilt services and verify status" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile up -d api web"
    Wait-ApiReady -ApiBaseUrl $BaseUrl
    Invoke-Checked "docker compose -f $ComposeFile ps"
  }
  finally {
    Pop-Location
  }
}

Write-Host ""
Write-Host "C1 cycle completed successfully." -ForegroundColor Green
