param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$CmsLogin = "cms.operator@example.com",
  [string]$CmsPassword = "secret"
)

$ErrorActionPreference = "Stop"

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )
  if (-not $Condition) {
    throw "ASSERT FAILED: $Message"
  }
}

function Invoke-JsonApi {
  param(
    [string]$Method,
    [string]$Url,
    [hashtable]$Headers = @{},
    $Body = $null
  )
  $params = @{
    Method      = $Method
    Uri         = $Url
    Headers     = $Headers
    ContentType = "application/json"
  }
  if ($null -ne $Body) {
    $params.Body = ($Body | ConvertTo-Json -Depth 10)
  }

  try {
    $res = Invoke-WebRequest @params
    return @{
      status = [int]$res.StatusCode
      body   = if ($res.Content) { $res.Content | ConvertFrom-Json } else { $null }
      raw    = $res.Content
    }
  }
  catch {
    $response = $_.Exception.Response
    if ($null -eq $response) {
      throw
    }
    $reader = New-Object System.IO.StreamReader($response.GetResponseStream())
    $rawBody = $reader.ReadToEnd()
    $parsed = $null
    if ($rawBody) {
      try { $parsed = $rawBody | ConvertFrom-Json } catch {}
    }
    return @{
      status = [int]$response.StatusCode
      body   = $parsed
      raw    = $rawBody
    }
  }
}

function Wait-ApiReady {
  param(
    [int]$MaxAttempts = 30,
    [int]$SleepSeconds = 2
  )
  Write-Host ">> waiting for API readiness at $BaseUrl"
  for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
    try {
      $health = Invoke-WebRequest -Method GET -Uri "$BaseUrl/healthz" -TimeoutSec 5
      $ready = Invoke-WebRequest -Method GET -Uri "$BaseUrl/readyz" -TimeoutSec 5
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
  throw "API readiness timeout after $MaxAttempts attempts: $BaseUrl"
}

function Login-And-GetAccessToken {
  param(
    [string]$LoginId,
    [string]$Password
  )

  $loginRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/auth/login" -Body @{
    login_id = $LoginId
    password = $Password
  }
  Assert-True ($loginRes.status -eq 200) "login failed for $LoginId status=$($loginRes.status) body=$($loginRes.raw)"

  if ($loginRes.body.session.access_token) {
    return [string]$loginRes.body.session.access_token
  }

  $preToken = [string]$loginRes.body.session.pre_company_token
  Assert-True (-not [string]::IsNullOrWhiteSpace($preToken)) "missing pre_company_token for $LoginId"
  $companyId = [string]$loginRes.body.memberships[0].company_id
  Assert-True (-not [string]::IsNullOrWhiteSpace($companyId)) "missing company_id in memberships for $LoginId"

  $selectRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/auth/select-company" -Headers @{
    Authorization = "Bearer $preToken"
  } -Body @{
    company_id = $companyId
  }
  Assert-True ($selectRes.status -eq 200) "select-company failed for $LoginId status=$($selectRes.status) body=$($selectRes.raw)"
  return [string]$selectRes.body.access_token
}

Wait-ApiReady

Write-Host "[1/7] Acquire CMS token..."
$cmsToken = Login-And-GetAccessToken -LoginId $CmsLogin -Password $CmsPassword

$fileName = "smoke-media-$(Get-Date -Format 'yyyyMMdd-HHmmss').png"
$sizeBytes = 1024

Write-Host "[2/7] Create media upload intent..."
$intentRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/platform/cms/media/upload" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  file_name    = $fileName
  content_type = "image/png"
  size_bytes   = $sizeBytes
  context      = "smoke"
}
Assert-True ($intentRes.status -eq 201) "create media intent failed status=$($intentRes.status) body=$($intentRes.raw)"
$assetId = [string]$intentRes.body.data.asset_id
$uploadUrl = [string]$intentRes.body.data.upload.url
Assert-True (-not [string]::IsNullOrWhiteSpace($assetId)) "asset_id missing from intent response"
Assert-True (-not [string]::IsNullOrWhiteSpace($uploadUrl)) "upload.url missing from intent response"

Write-Host "[3/7] Upload binary via signed URL..."
$tempFile = [System.IO.Path]::GetTempFileName()
try {
  $buffer = New-Object byte[] $sizeBytes
  for ($i = 0; $i -lt $sizeBytes; $i++) {
    $buffer[$i] = [byte]($i % 255)
  }
  [System.IO.File]::WriteAllBytes($tempFile, $buffer)

  $binaryRes = Invoke-WebRequest -Method PUT -Uri $uploadUrl -Headers @{
    "Content-Type" = "image/png"
  } -InFile $tempFile
  Assert-True ([int]$binaryRes.StatusCode -eq 200) "signed binary upload failed status=$([int]$binaryRes.StatusCode)"
}
finally {
  if (Test-Path $tempFile) {
    Remove-Item -Path $tempFile -Force
  }
}

Write-Host "[4/7] Complete media upload..."
$completeRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/platform/cms/media/$assetId/complete" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  size_bytes = $sizeBytes
  etag       = "smoke-etag"
  checksum   = "smoke-checksum"
}
Assert-True ($completeRes.status -eq 200) "complete media upload failed status=$($completeRes.status) body=$($completeRes.raw)"
Assert-True ($completeRes.body.data.state -eq "ready") "media state after complete should be ready"

Write-Host "[5/7] List and filter media assets..."
$listRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/media?q=$assetId&type=image/png" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($listRes.status -eq 200) "list media failed status=$($listRes.status) body=$($listRes.raw)"
Assert-True ($listRes.body.data.items.Count -ge 1) "media list should contain at least one item"
$matched = $false
foreach ($item in $listRes.body.data.items) {
  if ([string]$item.asset_id -eq $assetId) {
    $matched = $true
    Assert-True ([string]$item.state -eq "ready") "listed media state should be ready"
    break
  }
}
Assert-True $matched "created media asset not found in list"

Write-Host "[6/7] Delete media asset..."
$deleteRes = Invoke-JsonApi -Method "DELETE" -Url "$BaseUrl/api/v1/platform/cms/media/$assetId" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($deleteRes.status -eq 200) "delete media failed status=$($deleteRes.status) body=$($deleteRes.raw)"
Assert-True ($deleteRes.body.data.state -eq "deleted") "media state after delete should be deleted"

Write-Host "[7/7] Audit check for media actions..."
$auditRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/ops/audit?action=cms.media.upload.complete&resource_type=cms_media_asset&resource_id=$assetId" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($auditRes.status -eq 200) "audit query failed status=$($auditRes.status) body=$($auditRes.raw)"
Assert-True ($auditRes.body.data.items.Count -ge 1) "expected at least one media upload complete audit event"

Write-Host ""
Write-Host "CMS MEDIA DB-MODE SMOKE PASSED"
Write-Host "asset_id=$assetId"
