#Requires -Version 5.1
# Save as UTF-8 with BOM for Windows PowerShell 5.x.
<#
.SYNOPSIS
  Gracefully stop local Redis (SHUTDOWN via redis-cli).

.DESCRIPTION
  Port: -Port if positive, else REDIS_PORT / PORT, else redis.conf in this folder, else 6379.
  Linux-aligned env: REDIS_CLI_EXE, REDIS_PORT, FORCE_KILL (1 = taskkill after failed SHUTDOWN).

.PARAMETER Port
  0 = auto-detect (default).

.PARAMETER ForceKill
  If set, taskkill /F redis-server.exe after SHUTDOWN fails (kills all redis-server processes).
#>
[CmdletBinding()]
param(
    [int] $Port = 0,
    [switch] $ForceKill
)

$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
. "$here\redis-common.ps1"

$conf = Get-ScRedisConfFileIfPresent -ScriptRoot $here
if (-not $conf) { $conf = Join-Path $here "redis.conf" }
$listenPort = Resolve-ScRedisPort -ExplicitPort $Port -ConfPath $conf -ScriptRoot $here

$cli = Get-ScRedisCli -ScriptRoot $here
if (-not $cli) {
    Write-Error "找不到 redis-cli，无法发送 SHUTDOWN。可设置 REDIS_CLI_EXE。"
}

Write-Host "[redis-stop] redis-cli -p $listenPort SHUTDOWN"
& $cli -p $listenPort SHUTDOWN 2>&1 | Out-Host
if ($LASTEXITCODE -eq 0) {
    Write-Host "[redis-stop] 已发送关闭指令。"
    exit 0
}

Write-Warning "[redis-stop] SHUTDOWN 返回非零。若 Redis 已停止可忽略。"

$envForce = [Environment]::GetEnvironmentVariable("FORCE_KILL", "Process")
if ([string]::IsNullOrWhiteSpace($envForce)) { $envForce = [Environment]::GetEnvironmentVariable("FORCE_KILL", "User") }

if ($ForceKill -or ($envForce -eq "1")) {
    Write-Warning "[redis-stop] ForceKill：结束所有 redis-server 进程。"
    Get-Process redis-server -ErrorAction SilentlyContinue | Stop-Process -Force
}
exit 0
