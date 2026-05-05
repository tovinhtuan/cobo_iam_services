param(
  [ValidateSet("1", "2", "3", "all")]
  [string]$Phase = "all",
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

function Run-Phase1 {
  param(
    [string]$ScriptDir
  )
  $cycleScript = Join-Path $ScriptDir "run-c1-cycle.ps1"
  Invoke-Checked "powershell -ExecutionPolicy Bypass -File `"$cycleScript`" -BaseUrl `"$BaseUrl`" -ComposeFile `"$ComposeFile`" -UserLogin `"$UserLogin`" -UserPassword `"$UserPassword`" -AdminLogin `"$AdminLogin`" -AdminPassword `"$AdminPassword`" -CmsLogin `"$CmsLogin`" -CmsPassword `"$CmsPassword`" -UserCompanyId `"$UserCompanyId`""
}

function Run-Phase2 {
  param(
    [string]$IamRoot,
    [string]$WebRoot
  )
  Push-Location $IamRoot
  try {
    Invoke-Checked "go test ./..."
    Invoke-Checked "go test ./internal/httpserver -run TestIntegration_platformCMSPrefix -count=1"
  }
  finally {
    Pop-Location
  }

  Push-Location $WebRoot
  try {
    Invoke-Checked "npm run lint"
    Invoke-Checked "npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts src/features/cms-core/templateBlockSchema.test.ts"
    Invoke-Checked "npm run build"
  }
  finally {
    Pop-Location
  }
}

function Run-Phase3 {
  param(
    [string]$ScriptDir,
    [string]$IamRoot
  )
  Push-Location $IamRoot
  try {
    Invoke-Checked "docker compose -f $ComposeFile up -d api web"
  }
  finally {
    Pop-Location
  }

  $seedScript = Join-Path $ScriptDir "verify-seed-accounts.ps1"
  Invoke-Checked "powershell -ExecutionPolicy Bypass -File `"$seedScript`" -BaseUrl `"$BaseUrl`" -UserLogin `"$UserLogin`" -UserPassword `"$UserPassword`" -AdminLogin `"$AdminLogin`" -AdminPassword `"$AdminPassword`" -CmsLogin `"$CmsLogin`" -CmsPassword `"$CmsPassword`" -ExpectedUserCompanyId `"$UserCompanyId`""
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$iamRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$webRoot = Resolve-Path (Join-Path $iamRoot "..\cobo_web_design")

Write-Host "Week 3 phase gate start: phase=$Phase" -ForegroundColor Yellow

if ($Phase -eq "1" -or $Phase -eq "all") {
  Run-Step "Phase 1 - C2/CMS baseline cycle + docker parity" {
    Run-Phase1 -ScriptDir $scriptDir
  }
}

if ($Phase -eq "2" -or $Phase -eq "all") {
  Run-Step "Phase 2 - Full regression gate (BE+FE)" {
    Run-Phase2 -IamRoot $iamRoot -WebRoot $webRoot
  }
}

if ($Phase -eq "3" -or $Phase -eq "all") {
  Run-Step "Phase 3 - DevOps seed/account verification gate" {
    Run-Phase3 -ScriptDir $scriptDir -IamRoot $iamRoot
  }
}

Write-Host ""
Write-Host "WEEK 3 PHASE GATE PASSED." -ForegroundColor Green
