param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$UserLogin = "user@example.com",
  [string]$UserPassword = "secret",
  [string]$CmsLogin = "cms.operator@example.com",
  [string]$CmsPassword = "secret",
  [string]$UserCompanyId = "c_001"
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
    [string]$Password,
    [string]$PreferredCompanyId = ""
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

  $companyId = $PreferredCompanyId
  if ([string]::IsNullOrWhiteSpace($companyId)) {
    $companyId = [string]$loginRes.body.memberships[0].company_id
  }
  $selectRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/auth/select-company" -Headers @{
    Authorization = "Bearer $preToken"
  } -Body @{
    company_id = $companyId
  }
  Assert-True ($selectRes.status -eq 200) "select-company failed for $LoginId status=$($selectRes.status) body=$($selectRes.raw)"
  return [string]$selectRes.body.access_token
}

Wait-ApiReady

Write-Host "[1/9] Acquire tokens..."
$null = Login-And-GetAccessToken -LoginId $UserLogin -Password $UserPassword -PreferredCompanyId $UserCompanyId
$cmsToken = Login-And-GetAccessToken -LoginId $CmsLogin -Password $CmsPassword

Write-Host "[2/9] Create entry via CMS prefix..."
$title = "CMS Prefix Smoke $(Get-Date -Format 'yyyyMMdd-HHmmss')"
$createEntryRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/platform/cms/entries" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  type_id      = "DISCLOSURE_FINANCIAL"
  title        = $title
  summary      = "prefix smoke summary"
  content      = "prefix smoke content"
  planned_date = "2026-05-20"
}
Assert-True ($createEntryRes.status -eq 201) "create entry failed status=$($createEntryRes.status) body=$($createEntryRes.raw)"
$entryId = [string]$createEntryRes.body.data.entry_id
Assert-True (-not [string]::IsNullOrWhiteSpace($entryId)) "missing data.entry_id in create response"

Write-Host "[3/9] Get entry detail..."
$entryDetailRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/entries/$entryId" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($entryDetailRes.status -eq 200) "entry detail failed status=$($entryDetailRes.status) body=$($entryDetailRes.raw)"
Assert-True ($entryDetailRes.body.data.entry_id -eq $entryId) "entry detail id mismatch"

Write-Host "[4/9] Update entry via CMS prefix..."
$updateEntryRes = Invoke-JsonApi -Method "PUT" -Url "$BaseUrl/api/v1/platform/cms/entries/$entryId" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  type_id      = "DISCLOSURE_FINANCIAL"
  title        = "$title updated"
  summary      = "prefix smoke summary updated"
  content      = "prefix smoke content updated"
  planned_date = "2026-05-21"
}
Assert-True ($updateEntryRes.status -eq 200) "update entry failed status=$($updateEntryRes.status) body=$($updateEntryRes.raw)"

Write-Host "[5/9] Read collection detail..."
$collectionRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/collections/DISCLOSURE_FINANCIAL" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($collectionRes.status -eq 200) "collection detail failed status=$($collectionRes.status) body=$($collectionRes.raw)"
Assert-True ($collectionRes.body.data.items.Count -ge 1) "collection items should not be empty"

Write-Host "[6/9] Schedule create/list/delete..."
$createScheduleRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/platform/cms/schedules" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  entry_id   = $entryId
  publish_at = "2026-05-22"
}
Assert-True ($createScheduleRes.status -eq 201) "create schedule failed status=$($createScheduleRes.status) body=$($createScheduleRes.raw)"

$listScheduleRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/schedules" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($listScheduleRes.status -eq 200) "list schedules failed status=$($listScheduleRes.status) body=$($listScheduleRes.raw)"

$deleteScheduleRes = Invoke-JsonApi -Method "DELETE" -Url "$BaseUrl/api/v1/platform/cms/schedules/$entryId" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($deleteScheduleRes.status -eq 200) "delete schedule failed status=$($deleteScheduleRes.status) body=$($deleteScheduleRes.raw)"

Write-Host "[7/9] Submit entry then list reviews..."
$submitRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures/$entryId/submit" -Headers @{
  Authorization     = "Bearer $cmsToken"
  "Idempotency-Key" = "cms-prefix-submit-$entryId"
}
Assert-True ($submitRes.status -eq 200) "submit failed status=$($submitRes.status) body=$($submitRes.raw)"

$reviewListRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/reviews" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($reviewListRes.status -eq 200) "list reviews failed status=$($reviewListRes.status) body=$($reviewListRes.raw)"

Write-Host "[8/9] Approve review action via prefix..."
$approveRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/platform/cms/reviews/$entryId" -Headers @{
  Authorization = "Bearer $cmsToken"
} -Body @{
  decision = "approve"
}
Assert-True ($approveRes.status -eq 200) "approve review failed status=$($approveRes.status) body=$($approveRes.raw)"

Write-Host "[9/9] Validate envelope shape on key endpoints..."
Assert-True ($null -ne $createEntryRes.body.data) "create entry missing data envelope"
Assert-True ($null -ne $collectionRes.body.data) "collection detail missing data envelope"
Assert-True ($null -ne $reviewListRes.body.data) "reviews list missing data envelope"

Write-Host ""
Write-Host "CMS CORE PREFIX SMOKE PASSED"
Write-Host "entry_id=$entryId"
