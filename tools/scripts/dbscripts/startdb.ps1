<#
.SYNOPSIS
  启动 MySQL（便携）+ Redis。不启动 starcrystalsvr。

.EXAMPLE
  .\tools\scripts\dbscripts\startdb.ps1
  .\tools\scripts\dbscripts\startdb.ps1 -MysqlPort 3307 -RedisPort 6380
#>
[CmdletBinding()]
param(
    [string]$MysqlBaseDir = "",
    [string]$MysqlDataDir = "",
    [int]$MysqlPort = 3306,
    [int]$RedisPort = 0
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'dbscripts-config.ps1')
if ([string]::IsNullOrWhiteSpace($MysqlBaseDir)) { $MysqlBaseDir = Get-DefaultMysqlPortableBaseDir }
if ([string]::IsNullOrWhiteSpace($MysqlDataDir)) { $MysqlDataDir = Get-DefaultMysqlPortableDataDir }

$scriptDir = $PSScriptRoot

Write-Host "==> [1/2] Start MySQL (port $MysqlPort)"
& "$scriptDir\mysql\mysql-start.ps1" -BaseDir $MysqlBaseDir -DataDir $MysqlDataDir -Port $MysqlPort

Write-Host ""
Write-Host "==> [2/2] Start Redis"
$redisArgs = @{}
if ($RedisPort -gt 0) { $redisArgs['Port'] = $RedisPort }
& "$scriptDir\redis\redis-start.ps1" @redisArgs

Write-Host ""
Write-Host "startdb: MySQL + Redis started."
