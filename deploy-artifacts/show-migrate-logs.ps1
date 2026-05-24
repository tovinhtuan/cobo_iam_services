# Print cobo-iam-migrate container logs (last run) — run from any directory on Windows.
param(
  [int]$Tail = 120,
  [string]$DevHost = "88.216.208.0",
  [string]$DevPort = "21239",
  [string]$DevUser = "root"
)

$remote = "${DevUser}@${DevHost}"
Write-Host "==> migrate logs (tail $Tail)..."
& ssh -p $DevPort $remote "docker logs --tail $Tail cobo-iam-migrate 2>&1"

Write-Host "`n==> last applied migrations..."
& ssh -p $DevPort $remote "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e `"SELECT file_name, executed_at FROM schema_migrations ORDER BY executed_at DESC LIMIT 15;`""
