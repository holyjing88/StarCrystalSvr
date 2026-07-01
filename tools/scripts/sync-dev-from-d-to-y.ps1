# 将误在 D:\0_games\000StarCrystal\server 上的开发同步到 SVN 权威目录 Y:\holyjing\starcrystalsvr
# 策略：robocopy /XO；若目标为 SVN 受控只读文件则 copy 到 *.sync 后提示运行 apply-sync-pending.ps1
#
# Run:
#   .\tools\scripts\sync-dev-from-d-to-y.ps1
#   .\tools\scripts\apply-sync-pending.ps1   # 若有 *.sync

param(
    [switch]$WhatIf
)

$ErrorActionPreference = 'Stop'

$srcRoot = 'D:\0_games\000StarCrystal\server'
$dstRoot = 'Y:\holyjing\starcrystalsvr'
$svn = 'C:\Program Files\VisualSVN Server\bin\svn.exe'

if (-not (Test-Path -LiteralPath $srcRoot)) { throw "Source not found: $srcRoot" }
if (-not (Test-Path -LiteralPath $dstRoot)) { throw "Destination not found: $dstRoot" }

function Sync-RobocopyDir {
    param(
        [Parameter(Mandatory)][string]$RelativeDir,
        [string[]]$ExcludeDirs = @('node_modules', '.git', 'dist', '.docker-data')
    )
    $src = Join-Path $srcRoot $RelativeDir
    $dst = Join-Path $dstRoot $RelativeDir
    if (-not (Test-Path -LiteralPath $src)) {
        Write-Warning "Skip missing source dir: $RelativeDir"
        return
    }
    if (-not (Test-Path -LiteralPath $dst)) {
        New-Item -ItemType Directory -Force -Path $dst | Out-Null
    }
    $xd = if ($ExcludeDirs.Count -gt 0) { @('/XD') + $ExcludeDirs } else { @() }
    $args = @($src, $dst, '/E', '/XO', '/R:2', '/W:2', '/NFL', '/NDL', '/NJH', '/NJS', '/NC', '/NS') + $xd
    if ($WhatIf) {
        Write-Host "[WhatIf] robocopy $($args -join ' ')"
        return
    }
    & robocopy @args | Out-Null
    $code = $LASTEXITCODE
    if ($code -ge 8) {
        Write-Warning "robocopy $RelativeDir exit $code (some files may be SVN locked; see *.sync)"
    }
    else {
        Write-Host "  synced dir: $RelativeDir (exit $code)"
    }
}

function Sync-File {
    param([Parameter(Mandatory)][string]$RelativePath)
    $s = Join-Path $srcRoot $RelativePath
    $d = Join-Path $dstRoot $RelativePath
    if (-not (Test-Path -LiteralPath $s)) {
        Write-Warning "Skip missing source: $RelativePath"
        return
    }
    $dir = Split-Path -Parent $d
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
    if ($WhatIf) {
        Write-Host "[WhatIf] copy $RelativePath"
        return
    }
    try {
        Copy-Item -LiteralPath $s -Destination $d -Force
        Write-Host "  synced file: $RelativePath"
    }
    catch {
        $sync = "$d.sync"
        Copy-Item -LiteralPath $s -Destination $sync -Force
        Write-Warning "  locked -> $sync (run apply-sync-pending.ps1)"
    }
}

Write-Host 'Sync D -> Y (canonical: Y:\holyjing\starcrystalsvr)'
Write-Host "  from: $srcRoot"
Write-Host "  to:   $dstRoot"
Write-Host ''

Sync-RobocopyDir 'internal\api'
Sync-RobocopyDir 'internal\service'
Sync-RobocopyDir 'internal\config'
Sync-RobocopyDir 'cmd'
Sync-File 'go.mod'
Sync-File 'go.sum'
Sync-RobocopyDir 'tools\idip-webclient\src'
Sync-RobocopyDir 'tools\idip-webclient\doc'
Sync-File 'tools\scripts\verify-g001-h5-package.ps1'
Sync-RobocopyDir 'doc'

$gamesSrc = Join-Path $srcRoot 'release\configs\games.json'
$gamesDst = Join-Path $dstRoot 'release\configs\games.json'
if ((Test-Path $gamesSrc) -and (Test-Path $gamesDst) -and (Get-Item $gamesSrc).LastWriteTime -gt (Get-Item $gamesDst).LastWriteTime) {
    Sync-File 'release\configs\games.json'
}

$pending = @(Get-ChildItem -Path $dstRoot -Recurse -Filter '*.sync' -File -ErrorAction SilentlyContinue)
if ($pending.Count -gt 0) {
    Write-Host ''
    Write-Host "Pending $($pending.Count) file(s). Run: .\tools\scripts\apply-sync-pending.ps1"
}

Write-Host ''
Write-Host 'Done. Develop only under Y:\holyjing\starcrystalsvr (see tools\scripts\starcrystal-server-root.ps1).'
