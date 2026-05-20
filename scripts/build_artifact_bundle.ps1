param(
  [string]$OutputRoot = "deploy-artifacts",
  [string]$GoOs = "linux",
  [string]$GoArch = "amd64",
  [string]$WebApiBaseUrl = ""
)

$ErrorActionPreference = "Stop"

function Assert-LastExitCode {
  param([string]$Step)
  if ($LASTEXITCODE -ne 0) {
    throw "$Step failed with exit code $LASTEXITCODE"
  }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$workspaceRoot = Split-Path -Parent $repoRoot
$webRoot = Join-Path $workspaceRoot "cobo_web_design"
$artifactRoot = Join-Path $repoRoot $OutputRoot
$backendRoot = Join-Path $artifactRoot "backend"
$webArtifactRoot = Join-Path $artifactRoot "web"
$backendBinRoot = Join-Path $backendRoot "bin"
$backendMigrationRoot = Join-Path $backendRoot "migrations"
$backendConfigRoot = Join-Path $backendRoot "configs"

Write-Host "Preparing artifact bundle at $artifactRoot"
if (Test-Path $artifactRoot) {
  Remove-Item -Recurse -Force $artifactRoot
}

New-Item -ItemType Directory -Force -Path $backendBinRoot | Out-Null
New-Item -ItemType Directory -Force -Path $backendMigrationRoot | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $backendConfigRoot "non_trading_days") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $webArtifactRoot "dist") | Out-Null

Push-Location $repoRoot
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = $GoOs
  $env:GOARCH = $GoArch

  Write-Host "Building backend API binary..."
  go build -trimpath -ldflags "-s -w" -o (Join-Path $backendBinRoot "api") ./cmd/api
  Assert-LastExitCode "go build ./cmd/api"

  Write-Host "Building backend worker binary..."
  go build -trimpath -ldflags "-s -w" -o (Join-Path $backendBinRoot "worker") ./cmd/worker
  Assert-LastExitCode "go build ./cmd/worker"
}
finally {
  Pop-Location
}

Write-Host "Copying backend runtime files..."
Copy-Item (Join-Path $repoRoot "migrations\\*.sql") $backendMigrationRoot
Copy-Item (Join-Path $repoRoot "migrations\\run_dev_migrations.sh") $backendMigrationRoot
Copy-Item (Join-Path $repoRoot "configs\\non_trading_days\\*") (Join-Path $backendConfigRoot "non_trading_days") -Recurse

Push-Location $webRoot
try {
  Write-Host "Building frontend static bundle..."
  $env:VITE_API_BASE_URL = $WebApiBaseUrl
  npm.cmd run build
  Assert-LastExitCode "npm.cmd run build"
}
finally {
  Pop-Location
}

Write-Host "Copying frontend runtime files..."
Copy-Item (Join-Path $webRoot "dist\\*") (Join-Path $webArtifactRoot "dist") -Recurse
Copy-Item (Join-Path $repoRoot "deploy\\nginx\\cobo-web-artifact.conf") (Join-Path $webArtifactRoot "nginx.conf")

Write-Host ""
Write-Host "Artifact bundle ready:"
Write-Host "  $artifactRoot"
Write-Host ""
Write-Host "Copy these to the server:"
Write-Host "  - docker-compose.artifacts.yml"
Write-Host "  - deploy-artifacts/"
