#Requires -Version 5.1
# Save as UTF-8 with BOM for Windows PowerShell 5.x.
<#
.SYNOPSIS
  SAVE/BGSAVE then copy dump.rdb (and appendonly.aof if present) to backups.

.DESCRIPTION
  1) redis-cli SAVE, or BGSAVE with -UseBgSave (poll LASTSAVE).
  2) Copy dump.rdb into backup folder with timestamp in the file name.
  3) Prune old redis-dump-*.rdb beyond -Keep.

  Linux-aligned env: REDIS_CLI_EXE, REDIS_PORT, REDIS_DATA_DIR, REDIS_DIR, REDIS_CONF, BACKUP_ROOT, KEEP, USE_BGSAVE.

.PARAMETER Port
  0 = infer from redis.conf / REDIS_PORT (default).

.PARAMETER RedisDir
  Redis dir (folder with dump.rdb). Else REDIS_DATA_DIR, .\data, or dir from redis.conf.

.PARAMETER BackupRoot
  Backup root; default .\backups under this script folder.

.PARAMETER Keep
  Number of dump backups to keep (default 30).

.PARAMETER UseBgSave
  Use BGSAVE instead of SAVE (better for large instances).
#>
[CmdletBinding()]
param(
    [int] $Port = 0,
    [string] $RedisDir = "",
    [string] $BackupRoot = "",
    [int] $Keep = 30,
    [switch] $UseBgSave
)

$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
. "$here\redis-common.ps1"

function Get-DirFromConf {
    param([string] $ConfPath)
    if (-not (Test-Path -LiteralPath $ConfPath)) { return $null }
    foreach ($line in Get-Content -LiteralPath $ConfPath -Encoding UTF8) {
        $t = $line.Trim()
        if ($t.StartsWith("#") -or $t.Length -eq 0) { continue }
        if ($t -match '^\s*dir\s+(.+)$') {
            $d = $Matches[1].Trim().Trim('"')
            if (-not [System.IO.Path]::IsPathRooted($d)) {
                $d = Join-Path (Split-Path -Parent $ConfPath) $d
            }
            return $d
        }
    }
    return $null
}

function Resolve-RedisDir {
    param([string] $Explicit, [string] $ScriptRoot, [string] $ConfPath)
    if ($Explicit -and (Test-Path -LiteralPath $Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    $e = [Environment]::GetEnvironmentVariable("REDIS_DATA_DIR", "Process")
    if ([string]::IsNullOrWhiteSpace($e)) { $e = [Environment]::GetEnvironmentVariable("REDIS_DATA_DIR", "User") }
    if ($e -and (Test-Path -LiteralPath $e)) { return (Resolve-Path -LiteralPath $e).Path }
    $defData = Join-Path $ScriptRoot "data"
    if (Test-Path -LiteralPath (Join-Path $defData "dump.rdb")) { return (Resolve-Path -LiteralPath $defData).Path }
    if ($ConfPath -and (Test-Path -LiteralPath $ConfPath)) {
        $d = Get-DirFromConf -ConfPath $ConfPath
        if ($d -and (Test-Path -LiteralPath $d)) { return (Resolve-Path -LiteralPath $d).Path }
    }
    return $null
}

$cli = Get-ScRedisCli -ScriptRoot $here
if (-not $cli) { Write-Error "找不到 redis-cli。可设置 REDIS_CLI_EXE。" }

$conf = Get-ScRedisConfFileIfPresent -ScriptRoot $here
if (-not $conf) { $conf = Join-Path $here "redis.conf" }
$listenPort = Resolve-ScRedisPort -ExplicitPort $Port -ConfPath $conf -ScriptRoot $here

if ($UseBgSave) {
    Write-Host "[redis-backup] BGSAVE"
    $t0 = 0L
    try {
        $raw0 = (& $cli -p $listenPort LASTSAVE 2>$null | Out-String).Trim()
        if ($raw0) { $t0 = [int64][double]$raw0 }
    } catch { $t0 = 0L }
    & $cli -p $listenPort BGSAVE | Out-Host
    if ($LASTEXITCODE -ne 0) { Write-Error "[redis-backup] BGSAVE 命令失败。" }
    $dead = (Get-Date).AddMinutes(15)
    do {
        Start-Sleep -Seconds 1
        try {
            $raw1 = (& $cli -p $listenPort LASTSAVE 2>$null | Out-String).Trim()
            if ($raw1) {
                $t1 = [int64][double]$raw1
                if ($t1 -gt $t0) { break }
            }
        } catch { }
        if ((Get-Date) -gt $dead) { Write-Error "[redis-backup] BGSAVE 等待 LASTSAVE 超时。" }
    } while ($true)
} else {
    Write-Host "[redis-backup] SAVE（会短暂阻塞写入）"
    & $cli -p $listenPort SAVE | Out-Host
    if ($LASTEXITCODE -ne 0) { Write-Error "[redis-backup] SAVE 失败。" }
}

$dataDir = Resolve-RedisDir -Explicit $RedisDir -ScriptRoot $here -ConfPath $conf
if (-not $dataDir) {
    Write-Error "无法确定 Redis 数据目录。请指定 -RedisDir 或设置 REDIS_DATA_DIR，或配置 redis.conf 中 dir。"
}

$dump = Join-Path $dataDir "dump.rdb"
if (-not (Test-Path -LiteralPath $dump)) {
    Write-Error "未找到 dump.rdb: $dump"
}

$root = if ([string]::IsNullOrWhiteSpace($BackupRoot)) { Join-Path $here "backups" } else { $BackupRoot }
if (-not (Test-Path -LiteralPath $root)) { New-Item -ItemType Directory -Path $root | Out-Null }

$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$dest = Join-Path $root "redis-dump-$ts.rdb"
Copy-Item -LiteralPath $dump -Destination $dest -Force
Write-Host "[redis-backup] 已复制: $dest"

$aof = Join-Path $dataDir "appendonly.aof"
if (Test-Path -LiteralPath $aof) {
    $destAof = Join-Path $root "redis-appendonly-$ts.aof"
    Copy-Item -LiteralPath $aof -Destination $destAof -Force
    Write-Host "[redis-backup] 已复制 AOF: $destAof"
}

$old = Get-ChildItem -LiteralPath $root -Filter "redis-dump-*.rdb" -File | Sort-Object LastWriteTime -Descending | Select-Object -Skip $Keep
foreach ($f in $old) {
    Remove-Item -LiteralPath $f.FullName -Force
    Write-Host "[redis-backup] 删除旧备份: $($f.Name)"
}

Write-Host "[redis-backup] 完成。"
