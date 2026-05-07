param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$PlatformLogin = "cms.operator@example.com",
  [string]$PlatformPassword = "secret",
  [string]$CompanyAdminLogin = "admin.dn@example.com",
  [string]$CompanyAdminPassword = "secret"
)

$ErrorActionPreference = "Stop"

function Login-Session {
  param(
    [string]$LoginId,
    [string]$Password
  )
  return Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body (@{
      login_id = $LoginId
      password = $Password
    } | ConvertTo-Json)
}

function Resolve-AccessToken {
  param(
    $SessionBody,
    [string]$CompanyId
  )
  $accessToken = [string]$SessionBody.session.access_token
  if (-not [string]::IsNullOrWhiteSpace($accessToken)) {
    return $accessToken
  }
  $preToken = [string]$SessionBody.session.pre_company_token
  $selected = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/select-company" -Headers @{
    Authorization = "Bearer $preToken"
  } -ContentType "application/json" -Body (@{
      company_id = $CompanyId
    } | ConvertTo-Json)
  return [string]$selected.access_token
}

function Upsert-Type {
  param(
    [string]$Token,
    [string]$TypeId,
    [string]$Name,
    [string]$Scope,
    [string]$TemplateCategory
  )
  $deadlineStrategy = "configurable"
  $periodicity = "ad_hoc"
  if ($TemplateCategory -eq "periodic") {
    $deadlineStrategy = "fixed_cycle_days"
    $periodicity = "quarterly"
  }
  $body = @{
    type_id            = $TypeId
    scope              = $Scope
    group_id           = "group-006"
    name               = $Name
    category           = "Tuy chinh"
    template_category  = $TemplateCategory
    deadline_strategy  = $deadlineStrategy
    description        = "scope smoke test"
    deadline_rule      = "Theo cau hinh"
    periodicity        = $periodicity
    change_note        = "smoke template scope"
  }
  Invoke-RestMethod -Method Put -Uri "$BaseUrl/api/v1/admin/disclosure-types/$TypeId" -Headers @{
    Authorization = "Bearer $Token"
  } -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 10) | Out-Null
}

function List-TypeIds {
  param(
    [string]$Token
  )
  $res = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/v1/disclosure-types" -Headers @{
    Authorization = "Bearer $Token"
  }
  return @($res.items | ForEach-Object { [string]$_.type_id })
}

$platformSession = Login-Session -LoginId $PlatformLogin -Password $PlatformPassword
$companyAdminSession = Login-Session -LoginId $CompanyAdminLogin -Password $CompanyAdminPassword

$platformTokenC001 = Resolve-AccessToken -SessionBody $platformSession -CompanyId "c_001"
$companyAdminTokenC001 = Resolve-AccessToken -SessionBody $companyAdminSession -CompanyId "c_001"
$companyAdminTokenC002 = Resolve-AccessToken -SessionBody $companyAdminSession -CompanyId "c_002"

$stamp = Get-Date -Format "yyyyMMddHHmmss"
$globalTypeId = "dt-scope-global-$stamp"
$companyTypeId = "dt-scope-company-$stamp"

Upsert-Type -Token $platformTokenC001 -TypeId $globalTypeId -Name "Smoke Global Template" -Scope "global" -TemplateCategory "periodic"
Upsert-Type -Token $companyAdminTokenC001 -TypeId $companyTypeId -Name "Smoke Company Template" -Scope "company" -TemplateCategory "custom"

$c001TypeIds = List-TypeIds -Token $companyAdminTokenC001
$c002TypeIds = List-TypeIds -Token $companyAdminTokenC002

$globalInC001 = $c001TypeIds -contains $globalTypeId
$globalInC002 = $c002TypeIds -contains $globalTypeId
$companyInC001 = $c001TypeIds -contains $companyTypeId
$companyInC002 = $c002TypeIds -contains $companyTypeId

[pscustomobject]@{
  global_type_id           = $globalTypeId
  company_type_id          = $companyTypeId
  platform_global_c001     = $globalInC001
  platform_global_c002     = $globalInC002
  company_scope_c001       = $companyInC001
  company_scope_c002       = $companyInC002
  condition_platform_global = ($globalInC001 -and $globalInC002)
  condition_company_scope   = ($companyInC001 -and (-not $companyInC002))
} | ConvertTo-Json -Depth 5
