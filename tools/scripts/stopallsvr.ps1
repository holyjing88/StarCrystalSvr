<#
.SYNOPSIS
  一键停止：starcrystalsvr → Redis → MySQL
#>
[CmdletBinding()]
param(
    [string]$ProcessName = "starcrystalsvr",
    [switch]$Force,
    [string]$MysqlBaseDir = "",
    [int]$MysqlPort = 3306,
    [string]$MysqlPidPath = '',
    [int]$RedisPort = 0,
    [switch]$RedisForceKill
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
if ([string]::IsNullOrWhiteSpace($MysqlBaseDir)) { $MysqlBaseDir = Get-DefaultMysqlPortableBaseDir }
$scriptDir = $PSScriptRoot
$ReleaseRoot = Get-StarcrystalReleaseRoot

Write-Host "==> [1/3] Stop starcrystalsvr"
try {
    & (Join-Path $ReleaseRoot "stopsvr.ps1") -ProcessName $ProcessName -Port 0
} catch {
    Write-Warning "stop: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "==> [2/3] Stop Redis"
try {
    $redisArgs = @{}
    if ($RedisPort -gt 0) { $redisArgs["Port"] = $RedisPort }
    if ($RedisForceKill) { $redisArgs["ForceKill"] = $true }
    & "$scriptDir\dbscripts\redis\redis-stop.ps1" @redisArgs
} catch {
    Write-Warning "redis-stop: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "==> [3/3] Stop MySQL (port $MysqlPort)"
try {
    $mysqlArgs = @{ BaseDir = $MysqlBaseDir; Port = $MysqlPort }
    if (-not [string]::IsNullOrWhiteSpace($MysqlPidPath)) { $mysqlArgs["MysqlPidPath"] = $MysqlPidPath }
    & "$scriptDir\dbscripts\mysql\mysql-stop.ps1" @mysqlArgs
} catch {
    Write-Warning "mysql-stop: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "stopallsvr: finished."
