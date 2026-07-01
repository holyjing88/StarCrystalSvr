param(
    [string]$AuthSmsMock = "1",
    [string]$AuthMySqlDsn = ""
)

$ErrorActionPreference = "Stop"
$scriptDir = $PSScriptRoot
. (Join-Path $scriptDir "StarcrystalConfig.ps1")
$releaseRoot = Get-StarcrystalReleaseRoot
Write-Host "==> Step 1/2: Build"
& "$scriptDir\build.ps1"
Write-Host ""
Write-Host "==> Step 2/2: Start"
& (Join-Path $releaseRoot "startsvr.ps1") -AuthSmsMock $AuthSmsMock -AuthMySqlDsn $AuthMySqlDsn
Write-Host "All steps completed."
