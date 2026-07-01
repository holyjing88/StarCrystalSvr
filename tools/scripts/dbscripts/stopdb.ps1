<#
.SYNOPSIS
  停止 Redis + MySQL（便携）。不停止 starcrystalsvr。

.EXAMPLE
  .\tools\scripts\dbscripts\stopdb.ps1
#>
[CmdletBinding()]
param(
    [string]$MysqlBaseDir = "",
    [int]$MysqlPort = 3306,
    [string]$MysqlPidPath = '',
    [int]$RedisPort = 0,
    [switch]$RedisForceKill
)

$ErrorActionPreference = 'Continue'
. (Join-Path $PSScriptRoot 'dbscripts-config.ps1')
if ([string]::IsNullOrWhiteSpace($MysqlBaseDir)) { $MysqlBaseDir = Get-DefaultMysqlPortableBaseDir }

$scriptDir = $PSScriptRoot

Write-Host "==> [1/2] Stop Redis"
try {
    $redisArgs = @{}
    if ($RedisPort -gt 0) { $redisArgs['Port'] = $RedisPort }
    if ($RedisForceKill) { $redisArgs['ForceKill'] = $true }
    & "$scriptDir\redis\redis-stop.ps1" @redisArgs
} catch {
    Write-Warning "redis-stop: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "==> [2/2] Stop MySQL (port $MysqlPort)"
try {
    $mysqlArgs = @{ BaseDir = $MysqlBaseDir; Port = $MysqlPort }
    if (-not [string]::IsNullOrWhiteSpace($MysqlPidPath)) { $mysqlArgs['MysqlPidPath'] = $MysqlPidPath }
    & "$scriptDir\mysql\mysql-stop.ps1" @mysqlArgs
} catch {
    Write-Warning "mysql-stop: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "stopdb: finished."
