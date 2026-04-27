param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$ComposeFile = "docker-compose.dev.yml",
  [string]$UserLogin = "user@example.com",
  [string]$UserPassword = "secret",
  [string]$AdminLogin = "admin.dn@example.com",
  [string]$AdminPassword = "secret",
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
    Invoke-Checked "docker compose -f $ComposeFile ps"
  }
  finally {
    Pop-Location
  }
}

Run-Step "4/6 DB-mode disclosure C1 smoke" {
  Push-Location $iamRoot
  try {
    $smokeScript = Join-Path $scriptDir "disclosure-c1-db-smoke.ps1"
    Invoke-Checked "powershell -ExecutionPolicy Bypass -File `"$smokeScript`" -BaseUrl `"$BaseUrl`" -UserLogin `"$UserLogin`" -UserPassword `"$UserPassword`" -AdminLogin `"$AdminLogin`" -AdminPassword `"$AdminPassword`" -UserCompanyId `"$UserCompanyId`""
  }
  finally {
    Pop-Location
  }
}

Run-Step "5/6 Fresh no-cache Docker rebuild" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile build --no-cache api web"
  }
  finally {
    Pop-Location
  }
}

Run-Step "6/6 Bring up rebuilt services and verify status" {
  Push-Location $iamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile up -d api web"
    Invoke-Checked "docker compose -f $ComposeFile ps"
  }
  finally {
    Pop-Location
  }
}

Write-Host ""
Write-Host "C1 cycle completed successfully." -ForegroundColor Green
