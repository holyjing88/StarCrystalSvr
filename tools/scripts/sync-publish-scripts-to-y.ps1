# 将发布脚本同步到 SVN 权威目录 Y:\holyjing\starcrystalsvr\tools\0publish\scripts
# Run: .\tools\scripts\sync-publish-scripts-to-y.ps1

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'starcrystal-server-root.ps1')

$srcRoot = 'd:\0_games\000StarCrystal\server'
$dstRoot = Get-StarcrystalServerRoot
$pubScripts = 'tools\0publish\scripts'

$files = @(
    @{ Src = "$pubScripts\pack-publish.ps1"; Dst = "$pubScripts\pack-publish.ps1" },
    @{ Src = "$pubScripts\pack-publish.sh"; Dst = "$pubScripts\pack-publish.sh" },
    @{ Src = "$pubScripts\pack-publish-verify.sh"; Dst = "$pubScripts\pack-publish-verify.sh" },
    @{ Src = "$pubScripts\pack-publish-linux-remote.sh"; Dst = "$pubScripts\pack-publish-linux-remote.sh" },
    @{ Src = "$pubScripts\unpack.sh"; Dst = "$pubScripts\unpack.sh" },
    @{ Src = "$pubScripts\readme.txt"; Dst = "$pubScripts\readme.txt" },
    @{ Src = 'tools\scripts\starcrystal-server-root.ps1'; Dst = 'tools\scripts\starcrystal-server-root.ps1' },
    @{ Src = 'tools\scripts\starcrystal-server-root.sh'; Dst = 'tools\scripts\starcrystal-server-root.sh' }
)

foreach ($f in $files) {
    $s = Join-Path $srcRoot $f.Src
    $d = Join-Path $dstRoot $f.Dst
    if (-not (Test-Path -LiteralPath $s)) {
        throw "Missing source: $s"
    }
    $dir = Split-Path -Parent $d
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
    Copy-Item -LiteralPath $s -Destination $d -Force
    Write-Host "  $($f.Dst)"
}

Write-Host ''
Write-Host "Done -> $dstRoot"
Write-Host 'Publish: cd Y:\holyjing\starcrystalsvr ; .\tools\0publish\scripts\pack-publish.ps1'
