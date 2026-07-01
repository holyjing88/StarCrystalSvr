<#
.SYNOPSIS
  重建 MySQL auth schema + 清空 Redis 运行时键（sr:*）。

.EXAMPLE
  .\tools\scripts\dbscripts\rebuilddb.ps1
#>
[CmdletBinding()]
param(
    [string]$SqlHost = "127.0.0.1",
    [int]$MysqlPort = 3306
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'dbscripts-config.ps1')

$scriptDir = $PSScriptRoot

Write-Host '==> [1/2] MySQL rebuild (auth schema)'
& "$scriptDir\mysql\rebuild-auth-mysql.ps1" -SqlHost $SqlHost -Port $MysqlPort

Write-Host ''
Write-Host '==> [2/2] Redis rebuild (sr:*)'
& "$scriptDir\redis\rebuild-redis.ps1"

Write-Host ''
Write-Host '[rebuilddb] done'
