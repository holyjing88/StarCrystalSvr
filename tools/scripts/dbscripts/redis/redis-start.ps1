#Requires -Version 5.1
# Save as UTF-8 with BOM for Windows PowerShell 5.x.
<#
.SYNOPSIS
  Start local Redis (Windows: redis-server.exe).

.DESCRIPTION
  Redis server: -RedisServer, else env REDIS_SERVER_EXE, else PATH.
  Config: -Config, else redis.conf next to this script if present.
  Port: -Port if positive, else REDIS_PORT / PORT, else redis.conf port, else 6379.
  Data dir defaults to .\data under this script; -DataDir writes a minimal generated redis.conf.

  Env vars (aligned with Linux scripts): REDIS_SERVER_EXE, REDIS_CLI_EXE, REDIS_CONF, REDIS_PORT.

.PARAMETER Port
  Listen port; 0 means infer from config / REDIS_PORT (default 0).

.PARAMETER Config
  Path to redis.conf (absolute or relative to current directory).

.PARAMETER DataDir
  RDB/AOF directory; used with generated minimal config when -Config is not set.

.EXAMPLE
  .\redis-start.ps1
  .\redis-start.ps1 -Port 6380 -DataDir D:\redis-data\starcrystal
#>
[CmdletBinding()]
param(
    [int] $Port = 0,
    [string] $Config = "",
    [string] $DataDir = "",
    [string] $RedisServer = ""
)

$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
. "$here\redis-common.ps1"

$configPath = $null
if (-not [string]::IsNullOrWhiteSpace($Config)) {
    $configPath = if ([System.IO.Path]::IsPathRooted($Config)) { $Config } else { Join-Path (Get-Location) $Config }
    if (-not (Test-Path -LiteralPath $configPath)) { Write-Error "配置文件不存在: $configPath" }
} elseif ($dc = Get-ScRedisConfFileIfPresent -ScriptRoot $here) {
    $configPath = $dc
}

if (-not $configPath) {
    $dd = if ([string]::IsNullOrWhiteSpace($DataDir)) { Join-Path $here "data" } else { $DataDir }
    if (-not (Test-Path -LiteralPath $dd)) { New-Item -ItemType Directory -Path $dd | Out-Null }
    $configPath = Join-Path $here "redis.generated.conf"
    $listenPort = Resolve-ScRedisPort -ExplicitPort $Port -ConfPath "" -ScriptRoot $here
    @"
port $listenPort
bind 127.0.0.1
dir $((Resolve-Path -LiteralPath $dd).Path -replace '\\','/')
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000
appendonly no
loglevel notice
"@ | Set-Content -LiteralPath $configPath -Encoding UTF8
    Write-Host "[redis-start] 已生成临时配置: $configPath"
}

$listenPort = Resolve-ScRedisPort -ExplicitPort $Port -ConfPath $configPath -ScriptRoot $here

$exe = Get-ScRedisServer -Explicit $RedisServer -ScriptRoot $here
if (-not $exe) {
    Write-Error "找不到 redis-server。请将 Windows 版 Redis 解压到 server-go\redis（含 redis-server.exe、redis-cli.exe），或安装 Memurai / 加入 PATH，或设置 REDIS_SERVER_EXE，或使用 -RedisServer 指定完整路径。"
}

if (Test-ScRedisPong -Port $listenPort -ScriptRoot $here) {
    Write-Host "[redis-start] 端口 $listenPort 已有实例响应 PING，跳过启动。"
    exit 0
}

$logDir = Join-Path $here "logs"
if (-not (Test-Path -LiteralPath $logDir)) { New-Item -ItemType Directory -Path $logDir | Out-Null }

$workDir = Split-Path -Parent $exe
Write-Host "[redis-start] 启动: $exe"
Write-Host "[redis-start] 配置: $configPath （探测端口 $listenPort）"
Start-Process -FilePath $exe -ArgumentList "`"$configPath`"" -WorkingDirectory $workDir -WindowStyle Minimized
Start-Sleep -Seconds 1
if (Test-ScRedisPong -Port $listenPort -ScriptRoot $here) {
    Write-Host "[redis-start] 已启动，端口 $listenPort 。"
    exit 0
}
Write-Warning "[redis-start] 已尝试启动，但 PING 未通过。请查看 Redis 日志或是否缺少 MSVCRT/权限。"
