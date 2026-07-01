<#
.SYNOPSIS
  一键启动：MySQL → Redis → starcrystalsvr（在 release 目录下执行）。

.EXAMPLE
  cd server-go\release
  ..\tools\scripts\startallsvr.ps1
#>
[CmdletBinding()]
param(
    [string]$AuthSmsMock = "1",
    [string]$AuthMySqlDsn = "",
    [string]$MysqlBaseDir = "",
    [string]$MysqlDataDir = "",
    [int]$MysqlPort = 3306,
    [int]$RedisPort = 0
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
if ([string]::IsNullOrWhiteSpace($MysqlBaseDir)) { $MysqlBaseDir = Get-DefaultMysqlPortableBaseDir }
if ([string]::IsNullOrWhiteSpace($MysqlDataDir)) { $MysqlDataDir = Get-DefaultMysqlPortableDataDir }

$scriptDir = $PSScriptRoot

Write-Host "==> [1/3] Start MySQL (port $MysqlPort)"
& "$scriptDir\dbscripts\mysql\mysql-start.ps1" -BaseDir $MysqlBaseDir -DataDir $MysqlDataDir -Port $MysqlPort

Write-Host ""
Write-Host "==> [2/3] Start Redis"
$redisArgs = @{}
if ($RedisPort -gt 0) { $redisArgs["Port"] = $RedisPort }
& "$scriptDir\dbscripts\redis\redis-start.ps1" @redisArgs

Write-Host ""
Write-Host "==> [3/3] Start starcrystalsvr"
& (Join-Path $ReleaseRoot "startsvr.ps1") -AuthSmsMock $AuthSmsMock -AuthMySqlDsn $AuthMySqlDsn

Write-Host ""
Write-Host "startallsvr: all services started."
