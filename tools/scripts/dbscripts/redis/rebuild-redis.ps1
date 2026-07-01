<#
.SYNOPSIS
  清空 StarCrystal Redis 运行时键（sr:*）。配置读 dbscripts/config/starcrystal.json 或 local.env。

.EXAMPLE
  .\tools\scripts\dbscripts\redis\rebuild-redis.ps1
#>
[CmdletBinding()]
param(
    [string]$RedisHost = "",
    [int]$RedisPort = 0,
    [int]$RedisDb = -1,
    [string]$RedisPassword = "",
    [string]$Pattern = "sr:*",
    [string]$RedisCliPath = ""
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\dbscripts-config.ps1')
. (Join-Path $PSScriptRoot 'redis-common.ps1')

$settings = Get-DbScriptsRedisSettings
if ([string]::IsNullOrWhiteSpace($RedisHost)) { $RedisHost = $settings.RedisHost }
if ($RedisPort -le 0) { $RedisPort = $settings.RedisPort }
if ($RedisDb -lt 0) { $RedisDb = $settings.RedisDb }
if ($RedisPassword -eq '' -and $settings.RedisPassword) { $RedisPassword = $settings.RedisPassword }
if ($env:REDIS_REBUILD_PATTERN) { $Pattern = $env:REDIS_REBUILD_PATTERN.Trim() }

$cli = Get-ScRedisCli -Explicit $RedisCliPath -ScriptRoot $PSScriptRoot
if (-not $cli) { throw '[rebuild-redis] FAIL — redis-cli not found' }

function Invoke-RedisCli {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$CliArgs)
    $base = @('-h', $RedisHost, '-p', "$RedisPort", '-n', "$RedisDb")
    if ($RedisPassword) { $base += '-a', $RedisPassword }
    & $cli @base @CliArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Get-MatchingKeyCount {
    $keys = @(Invoke-RedisCli --scan --pattern $Pattern 2>$null)
    return $keys.Count
}

$ping = Invoke-RedisCli ping 2>$null
if ("$ping" -notmatch 'PONG') {
    throw "[rebuild-redis] FAIL — cannot connect Redis ${RedisHost}:${RedisPort} db=${RedisDb}"
}

$before = Get-MatchingKeyCount
Write-Host "[rebuild-redis] connect ${RedisHost}:${RedisPort} db=${RedisDb} pattern=${Pattern}"
Write-Host "[rebuild-redis] keys before rebuild: ${before}"

if ($env:REDIS_REBUILD_FLUSHDB -eq '1') {
    Write-Host '[rebuild-redis] REDIS_REBUILD_FLUSHDB=1: FLUSHDB ...'
    Invoke-RedisCli FLUSHDB | Out-Null
} else {
    $deleted = 0
    $keys = @(Invoke-RedisCli --scan --pattern $Pattern 2>$null)
    foreach ($key in $keys) {
        if ([string]::IsNullOrWhiteSpace($key)) { continue }
        Invoke-RedisCli DEL $key | Out-Null
        $deleted++
    }
    Write-Host "[rebuild-redis] deleted ${deleted} keys"
}

$after = Get-MatchingKeyCount
Write-Host "[rebuild-redis] keys after rebuild: ${after}"
if ($after -ne 0) {
    throw "[rebuild-redis] FAIL — ${after} keys still match ${Pattern}"
}

$schemaDoc = Join-Path (Get-DbScriptsSqlDir) 'starcrystal_redis_keys.md'
if (Test-Path -LiteralPath $schemaDoc) {
    Write-Host ''
    Write-Host "[rebuild-redis] key schema doc: $schemaDoc"
    Select-String -Path $schemaDoc -Pattern '^\| `sr:' | Select-Object -First 12 | ForEach-Object { $_.Line }
}

Write-Host "[rebuild-redis] OK — Redis rebuilt (db=${RedisDb}, cleared ${Pattern})"
