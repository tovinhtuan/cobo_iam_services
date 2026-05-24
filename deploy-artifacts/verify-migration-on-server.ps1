# Verify a migration file on the dev server contains expected marker text.
param(
  [string]$File = "0060_deadline_rule_catalog.up.sql",
  [string]$MustContain = "0060-deadline-v2",
  [string]$DevHost = "88.216.208.0",
  [string]$DevPort = "21239",
  [string]$DevUser = "root",
  [string]$DevPath = "/root/cobo_project"
)

$remote = "${DevUser}@${DevHost}"
$path = "$DevPath/migrations/$File"

Write-Host "==> Remote: $path"
& ssh -p $DevPort $remote "grep -n '$MustContain' '$path' || (echo 'MISSING marker $MustContain — upload migrations again' && head -n 30 '$path' && exit 1)"
Write-Host "OK: server has updated $File"
