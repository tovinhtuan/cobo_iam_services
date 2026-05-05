param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$UserLogin = "user@example.com",
  [string]$UserPassword = "secret",
  [string]$AdminLogin = "admin.dn@example.com",
  [string]$AdminPassword = "secret",
  [string]$CmsLogin = "cms.operator@example.com",
  [string]$CmsPassword = "secret",
  [string]$ExpectedUserCompanyId = "c_001"
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

function Login-Session {
  param(
    [string]$LoginId,
    [string]$Password
  )
  $res = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/auth/login" -Body @{
    login_id = $LoginId
    password = $Password
  }
  Assert-True ($res.status -eq 200) "login failed for $LoginId status=$($res.status) body=$($res.raw)"
  return $res.body
}

function Select-Company-And-GetToken {
  param(
    [string]$PreCompanyToken,
    [string]$CompanyId
  )
  $res = Invoke-JsonApi -Method "POST" -Url "$BaseUrl/api/v1/auth/select-company" -Headers @{
    Authorization = "Bearer $PreCompanyToken"
  } -Body @{
    company_id = $CompanyId
  }
  Assert-True ($res.status -eq 200) "select-company failed company=$CompanyId status=$($res.status) body=$($res.raw)"
  $token = [string]$res.body.access_token
  Assert-True (-not [string]::IsNullOrWhiteSpace($token)) "access_token missing after select-company"
  return $token
}

function Resolve-AccessToken {
  param(
    $SessionBody,
    [string]$PreferredCompanyId = ""
  )
  $accessToken = [string]$SessionBody.session.access_token
  if (-not [string]::IsNullOrWhiteSpace($accessToken)) {
    return $accessToken
  }
  $preToken = [string]$SessionBody.session.pre_company_token
  Assert-True (-not [string]::IsNullOrWhiteSpace($preToken)) "missing pre_company_token"
  $companyId = $PreferredCompanyId
  if ([string]::IsNullOrWhiteSpace($companyId)) {
    $companyId = [string]$SessionBody.memberships[0].company_id
  }
  Assert-True (-not [string]::IsNullOrWhiteSpace($companyId)) "no company id for select-company"
  return Select-Company-And-GetToken -PreCompanyToken $preToken -CompanyId $companyId
}

function Assert-CurrentUser {
  param(
    [string]$Token,
    [string]$ExpectedLogin
  )
  $me = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/me" -Headers @{
    Authorization = "Bearer $Token"
  }
  Assert-True ($me.status -eq 200) "/me failed for $ExpectedLogin status=$($me.status) body=$($me.raw)"
  $actual = [string]$me.body.user.login_id
  Assert-True ($actual -eq $ExpectedLogin) "login mismatch expected=$ExpectedLogin actual=$actual"
}

Wait-ApiReady

Write-Host "[1/4] Verify user seed account and default company membership..."
$userSession = Login-Session -LoginId $UserLogin -Password $UserPassword
$userMembershipCount = @($userSession.memberships).Count
Assert-True ($userMembershipCount -ge 1) "user should have >= 1 membership"
$userMembershipIds = @($userSession.memberships | ForEach-Object { [string]$_.company_id })
Assert-True ($userMembershipIds -contains $ExpectedUserCompanyId) "user missing expected membership company=$ExpectedUserCompanyId"
$userToken = Resolve-AccessToken -SessionBody $userSession -PreferredCompanyId $ExpectedUserCompanyId
Assert-CurrentUser -Token $userToken -ExpectedLogin $UserLogin

Write-Host "[2/4] Verify admin seed account permissions path..."
$adminSession = Login-Session -LoginId $AdminLogin -Password $AdminPassword
$adminToken = Resolve-AccessToken -SessionBody $adminSession
Assert-CurrentUser -Token $adminToken -ExpectedLogin $AdminLogin
$adminPermissions = @($adminSession.permissions)
Assert-True ($adminPermissions.Count -ge 1) "admin should have seeded permissions"

Write-Host "[3/4] Verify CMS operator seed account capabilities..."
$cmsSession = Login-Session -LoginId $CmsLogin -Password $CmsPassword
$cmsToken = Resolve-AccessToken -SessionBody $cmsSession
Assert-CurrentUser -Token $cmsToken -ExpectedLogin $CmsLogin
$cmsRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/platform/cms/collections" -Headers @{
  Authorization = "Bearer $cmsToken"
}
Assert-True ($cmsRes.status -eq 200) "cms operator cannot access CMS collections status=$($cmsRes.status) body=$($cmsRes.raw)"

Write-Host "[4/4] Verify service account baseline endpoint health..."
$companiesRes = Invoke-JsonApi -Method "GET" -Url "$BaseUrl/api/v1/disclosures?limit=1" -Headers @{
  Authorization = "Bearer $userToken"
}
Assert-True ($companiesRes.status -eq 200) "user baseline disclosure list failed status=$($companiesRes.status) body=$($companiesRes.raw)"

Write-Host ""
Write-Host "SEED ACCOUNT VERIFICATION PASSED." -ForegroundColor Green
