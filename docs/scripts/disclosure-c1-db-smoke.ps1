param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$UserLogin = "user@example.com",
  [string]$UserPassword = "secret",
  [string]$AdminLogin = "admin.dn@example.com",
  [string]$AdminPassword = "secret",
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
  $accessToken = [string]$selectRes.body.access_token
  Assert-True (-not [string]::IsNullOrWhiteSpace($accessToken)) "missing access_token after select-company for $LoginId"
  return $accessToken
}

Write-Host "[1/8] Acquire access tokens..."
$userToken = Login-And-GetAccessToken -LoginId $UserLogin -Password $UserPassword -PreferredCompanyId $UserCompanyId
$adminToken = Login-And-GetAccessToken -LoginId $AdminLogin -Password $AdminPassword

$recordTitle = "DB Smoke C1 $(Get-Date -Format 'yyyyMMdd-HHmmss')"
$createPayload = @{
  type_id       = "DISCLOSURE_FINANCIAL"
  department_id = "ou_legal"
  title         = $recordTitle
  summary       = "DB smoke summary"
  content       = "DB smoke body"
  planned_date  = "2026-05-01"
  attachments   = @(
    @{
      id          = "att-smoke-1"
      name        = "proof.pdf"
      type        = "application/pdf"
      uploaded_at = "2026-04-27T00:00:00Z"
    }
  )
  evidence_link = "https://example.com/smoke-proof"
}

Write-Host "[2/8] Create disclosure and assert contract fields..."
$createRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures" -Headers @{
  Authorization = "Bearer $userToken"
} -Body $createPayload
Assert-True ($createRes.status -eq 201) "create disclosure failed status=$($createRes.status) body=$($createRes.raw)"

$recordId = [string]$createRes.body.record_id
Assert-True (-not [string]::IsNullOrWhiteSpace($recordId)) "record_id missing in create response"
Assert-True ($createRes.body.type_id -eq "DISCLOSURE_FINANCIAL") "type_id mismatch"
Assert-True ($createRes.body.summary -eq "DB smoke summary") "summary mismatch"
Assert-True ($createRes.body.planned_date -eq "2026-05-01") "planned_date mismatch"
Assert-True ($createRes.body.evidence_link -eq "https://example.com/smoke-proof") "evidence_link mismatch"
Assert-True ($createRes.body.status -eq "Draft") "status after create must be Draft"

Write-Host "[3/8] GET disclosure and assert persisted payload..."
$getRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/disclosures/$recordId" -Headers @{
  Authorization = "Bearer $userToken"
}
Assert-True ($getRes.status -eq 200) "get disclosure failed status=$($getRes.status) body=$($getRes.raw)"
Assert-True ($getRes.body.record_id -eq $recordId) "get record_id mismatch"
Assert-True ($getRes.body.title -eq $recordTitle) "get title mismatch"

Write-Host "[4/8] Update disclosure and assert modified fields..."
$updateRes = Invoke-JsonApi -Method "PATCH" -Url "$BaseUrl/api/v1/disclosures/$recordId" -Headers @{
  Authorization = "Bearer $userToken"
} -Body @{
  type_id       = "DISCLOSURE_FINANCIAL"
  department_id = "ou_legal"
  title         = "$recordTitle updated"
  summary       = "DB smoke summary updated"
  content       = "DB smoke body updated"
  planned_date  = "2026-05-02"
  attachments   = @(
    @{
      id          = "att-smoke-2"
      name        = "proof-updated.pdf"
      type        = "application/pdf"
      uploaded_at = "2026-04-27T01:00:00Z"
    }
  )
  evidence_link = "https://example.com/smoke-proof-updated"
}
Assert-True ($updateRes.status -eq 200) "update disclosure failed status=$($updateRes.status) body=$($updateRes.raw)"
Assert-True ($updateRes.body.planned_date -eq "2026-05-02") "updated planned_date mismatch"
Assert-True ($updateRes.body.summary -eq "DB smoke summary updated") "updated summary mismatch"

Write-Host "[5/8] Submit disclosure and assert Published transition..."
$submitRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures/$recordId/submit" -Headers @{
  Authorization    = "Bearer $userToken"
  "Idempotency-Key" = "smoke-submit-$recordId"
}
Assert-True ($submitRes.status -eq 200) "submit disclosure failed status=$($submitRes.status) body=$($submitRes.raw)"
Assert-True ($submitRes.body.status -eq "Published") "status after submit must be Published"
Assert-True (-not [string]::IsNullOrWhiteSpace([string]$submitRes.body.published_date)) "published_date missing after submit"

Write-Host "[6/8] Confirm by non-admin must be forbidden..."
$confirmUserRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures/$recordId/confirm" -Headers @{
  Authorization    = "Bearer $userToken"
  "Idempotency-Key" = "smoke-confirm-user-$recordId"
}
Assert-True ($confirmUserRes.status -eq 403) "confirm by non-admin should be 403, got $($confirmUserRes.status)"

Write-Host "[7/8] Confirm by admin then re-confirm state conflict..."
$confirmAdminRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures/$recordId/confirm" -Headers @{
  Authorization    = "Bearer $adminToken"
  "Idempotency-Key" = "smoke-confirm-admin-$recordId"
}
Assert-True ($confirmAdminRes.status -eq 200) "confirm by admin failed status=$($confirmAdminRes.status) body=$($confirmAdminRes.raw)"
Assert-True ($confirmAdminRes.body.status -eq "Completed") "status after admin confirm must be Completed"

$confirmAgainRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures/$recordId/confirm" -Headers @{
  Authorization    = "Bearer $adminToken"
  "Idempotency-Key" = "smoke-confirm-admin-again-$recordId"
}
Assert-True ($confirmAgainRes.status -eq 409) "confirm again should be 409, got $($confirmAgainRes.status)"

Write-Host "[8/8] Validation error checks (400 + 401)..."
$invalidDateRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures" -Headers @{
  Authorization = "Bearer $userToken"
} -Body @{
  title        = "Invalid Date"
  content      = "Body"
  planned_date = "2026/05/01"
}
Assert-True ($invalidDateRes.status -eq 400) "invalid planned_date should be 400, got $($invalidDateRes.status)"

$missingAuthRes = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/disclosures" -Body @{
  title   = "No Auth"
  content = "No Auth"
}
Assert-True ($missingAuthRes.status -eq 401) "missing auth should be 401, got $($missingAuthRes.status)"

Write-Host ""
Write-Host "DISCLOSURE C1 DB-MODE SMOKE PASSED"
Write-Host "record_id=$recordId"
