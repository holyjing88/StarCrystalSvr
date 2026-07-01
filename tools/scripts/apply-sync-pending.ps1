# 将 sync-dev-from-d-to-y.ps1 生成的 *.sync 覆盖回正式文件（SVN 只读文件需先 svn delete）。
# Run: cd Y:\holyjing\starcrystalsvr ; .\tools\scripts\apply-sync-pending.ps1

$ErrorActionPreference = 'Stop'
$root = 'Y:\holyjing\starcrystalsvr'
$svn = 'C:\Program Files\VisualSVN Server\bin\svn.exe'
if (-not (Test-Path $svn)) { $svn = 'svn' }

Push-Location $root
try {
    $syncFiles = Get-ChildItem -Recurse -Filter '*.sync' -File |
        Where-Object { $_.FullName -notmatch '\\node_modules\\' }
    if ($syncFiles.Count -eq 0) {
        Write-Host 'No *.sync files found.'
        return
    }
    foreach ($sf in $syncFiles) {
        $target = $sf.FullName.Substring(0, $sf.FullName.Length - 5)
        $rel = $target.Substring($root.Length + 1)
        Write-Host "Apply $rel"
        & $svn delete --force $rel 2>&1 | Out-Null
        Copy-Item -LiteralPath $sf.FullName -Destination $target -Force
        & $svn add $rel 2>&1 | Out-Null
        Remove-Item -LiteralPath $sf.FullName -Force
    }
    Write-Host 'Done.'
}
finally {
    Pop-Location
}
