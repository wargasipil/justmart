<#
  Justmart Windows teardown. Invoked by the Inno Setup uninstaller (as Admin)
  before files are removed. Stops + removes both services and the firewall rule.
  The PostgreSQL data dir + config are preserved unless -PurgeData 1 is passed
  (the uninstaller asks the user).
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $AppDir,
  [Parameter(Mandatory)] [string] $DataDir,
  [int] $PurgeData = 0
)

$ErrorActionPreference = "SilentlyContinue"
$pgbin  = Join-Path $AppDir "pgsql\bin"
$pgdata = Join-Path $DataDir "pgdata"
$winsw  = Join-Path $AppDir "winsw\justmart-server.exe"

# App service (WinSW)
& $winsw stop
& $winsw uninstall

# PostgreSQL service (native pg_ctl registration)
Stop-Service justmart-postgres -Force
& "$pgbin\pg_ctl.exe" unregister -N "justmart-postgres"

# Firewall rule
netsh advfirewall firewall delete rule name="Justmart Server" | Out-Null

if ($PurgeData -eq 1) {
  Remove-Item -Recurse -Force $DataDir
  Write-Host "Removed all Justmart data: $DataDir"
} else {
  Write-Host "Kept Justmart data (database + config) at $DataDir"
}
