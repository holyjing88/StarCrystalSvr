# 在 Windows 上通过 WSL 编译并安装 Linux 版 Redis 到 dbscripts/redis/linux/。
param(
    [string] $RedisVersion = "7.2.6"
)
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$sh = Join-Path $here "install-redis-linux.sh"
if (-not (Test-Path -LiteralPath $sh)) {
    Write-Error "找不到脚本: $sh"
}
if (-not (Get-Command wsl -ErrorAction SilentlyContinue)) {
    Write-Host "未检测到 wsl.exe。请在 Linux 或 WSL 终端执行:"
    Write-Host "  bash tools/scripts/dbscripts/redis/install-redis-linux.sh"
    exit 1
}
$shWsl = (& wsl wslpath -a $sh).Trim()
if ([string]::IsNullOrWhiteSpace($shWsl)) {
    Write-Error "wslpath 转换失败: $sh"
}
Write-Host "[install-redis-linux.ps1] WSL 执行: $shWsl (REDIS_VERSION=$RedisVersion)"
$env:REDIS_VERSION = $RedisVersion
wsl -e bash -lc "export REDIS_VERSION='$RedisVersion'; bash '$shWsl'"
Remove-Item Env:REDIS_VERSION -ErrorAction SilentlyContinue
Write-Host "[install-redis-linux.ps1] 完成。"
