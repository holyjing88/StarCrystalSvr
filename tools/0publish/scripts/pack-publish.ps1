# StarCrystal 发布打包（权威脚本：Y:\holyjing\starcrystalsvr\tools\0publish\scripts\pack-publish.ps1）
# 用法: cd Y:\holyjing\starcrystalsvr ; .\tools\0publish\scripts\pack-publish.ps1
param(
    [switch]$SkipBuild,
    [switch]$BuildIdip,
    [string]$PublishDir = "",
    [string]$RepoRoot = ""
)

$ErrorActionPreference = "Stop"

$Y_SVR_ROOT = 'Y:\holyjing\starcrystalsvr'
$CanonicalScript = Join-Path $Y_SVR_ROOT 'tools\0publish\scripts\pack-publish.ps1'

if ($PSScriptRoot -notlike "$Y_SVR_ROOT*") {
    if (-not (Test-Path -LiteralPath $CanonicalScript)) {
        throw @"
Publish script must run from Y:\holyjing\starcrystalsvr\tools\0publish\scripts\
  Missing: $CanonicalScript
  Sync: .\tools\scripts\sync-publish-scripts-to-y.ps1
"@
    }
    Write-Host "==> delegate to $CanonicalScript"
    & $CanonicalScript @PSBoundParameters
    exit $LASTEXITCODE
}

. (Join-Path $Y_SVR_ROOT 'tools\scripts\starcrystal-server-root.ps1')
$RepoRoot = Get-StarcrystalServerRoot -Override $RepoRoot
if (-not (Test-StarcrystalServerRootIsCanonical $RepoRoot)) {
    throw "pack-publish requires repo Y:\holyjing\starcrystalsvr"
}

$PublishRoot = Join-Path $RepoRoot 'tools\0publish'
$ToolsRoot = Join-Path $RepoRoot 'tools'
$ReleaseSrc = Join-Path $RepoRoot 'release'
$ReleaseH5Src = Join-Path $RepoRoot 'release_h5'
$DbscriptsSrc = Join-Path $ToolsRoot 'scripts\dbscripts'
$IdipSrc = Join-Path $ToolsRoot 'idip-webclient'
$BuildScript = Join-Path $ToolsRoot 'scripts\build.ps1'

if (-not $PublishDir) {
    $PublishDir = Get-Date -Format 'yyyyMMdd-HHmmss'
}
if ($PublishDir -notmatch '^\d{8}(-\d{6})?$') {
    throw "Invalid -PublishDir '$PublishDir'; use yyyyMMdd-HHmmss"
}

$OutputDir = Join-Path $PublishRoot $PublishDir
$StagingRoot = Join-Path $OutputDir '_staging'
$ZipRelease = Join-Path $OutputDir 'release.zip'
$ZipDbscripts = Join-Path $OutputDir 'dbscripts.zip'
$ZipIdip = Join-Path $OutputDir 'idip-webclient.zip'
$ZipReleaseH5 = Join-Path $OutputDir 'release_h5.zip'
$ManifestPath = Join-Path $OutputDir 'pack-manifest.txt'
$BundleZip = Join-Path $PublishRoot "$PublishDir.zip"

function Remove-TreeIfExists {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return }
    cmd /c "attrib -r `"$Path\*`" /s /d" 2>$null | Out-Null
    try {
        Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
    } catch {
        throw "Cannot remove $Path ($($_.Exception.Message)). Close programs using these files and retry."
    }
}

function Invoke-RobocopyFiltered {
    param(
        [Parameter(Mandatory)][string]$Source,
        [Parameter(Mandatory)][string]$Destination,
        [string[]]$DirExclude = @(),
        [string[]]$FileExclude = @()
    )
    if (-not (Test-Path -LiteralPath $Source)) {
        throw "Source not found: $Source"
    }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null

    $xd = @()
    foreach ($d in $DirExclude) { if ($d) { $xd += '/XD'; $xd += $d } }
    $xf = @()
    foreach ($f in $FileExclude) { if ($f) { $xf += '/XF'; $xf += $f } }

    $args = @(
        $Source, $Destination,
        '/E', '/NFL', '/NDL', '/NJH', '/NJS', '/NC', '/NS', '/NP'
    ) + $xd + $xf

    $code = (Start-Process -FilePath 'robocopy.exe' -ArgumentList $args -Wait -PassThru -NoNewWindow).ExitCode
    if ($code -ge 8) {
        throw "robocopy failed ($code): $Source -> $Destination"
    }
}

function Copy-DbscriptsBundle {
    param([Parameter(Mandatory)][string]$Destination)
    Invoke-RobocopyFiltered -Source $DbscriptsSrc -Destination $Destination `
        -DirExclude @('data\mysql', 'data\mysql_backup', 'redis\linux') `
        -FileExclude @('*.pid')
    foreach ($sub in @('data\mysql', 'data\mysql_backup')) {
        New-Item -ItemType Directory -Force -Path (Join-Path $Destination $sub) | Out-Null
    }
}

function Copy-IdipBundle {
    param([Parameter(Mandatory)][string]$Destination)
    Invoke-RobocopyFiltered -Source $IdipSrc -Destination $Destination `
        -DirExclude @('node_modules', '.tmp-regression', 'tmp-regression') `
        -FileExclude @('*.local', 'tsconfig.tsbuildinfo')
}

function Copy-ReleaseH5Bundle {
    param([Parameter(Mandatory)][string]$Destination)
    if (-not (Test-Path -LiteralPath $ReleaseH5Src)) {
        Write-Warning "release_h5 not found: $ReleaseH5Src (packing empty tree)"
        New-Item -ItemType Directory -Force -Path $Destination | Out-Null
        return
    }
    Invoke-RobocopyFiltered -Source $ReleaseH5Src -Destination $Destination `
        -DirExclude @('.upload-*') `
        -FileExclude @('.upload-*')
}

function Copy-ReleaseBundle {
    param([Parameter(Mandatory)][string]$Destination)
    Invoke-RobocopyFiltered -Source $ReleaseSrc -Destination $Destination `
        -FileExclude @('*.pid', '.dockerignore')
    $logDir = Join-Path $Destination 'log'
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    Get-ChildItem -LiteralPath $logDir -File -ErrorAction SilentlyContinue | Remove-Item -Force
    $keep = Join-Path $logDir '.gitkeep'
    if (-not (Test-Path -LiteralPath $keep)) {
        New-Item -ItemType File -Path $keep -Force | Out-Null
    }
    foreach ($bin in @('starcrystalsvr', 'starcrystalsvr.exe')) {
        $p = Join-Path $Destination $bin
        if (-not (Test-Path -LiteralPath $p)) {
            Write-Warning "Missing release binary: $p"
        }
    }
}

function Write-ZipFromFolder {
    param(
        [Parameter(Mandatory)][string]$FolderPath,
        [Parameter(Mandatory)][string]$ZipPath
    )
    if (Test-Path -LiteralPath $ZipPath) {
        Remove-Item -LiteralPath $ZipPath -Force
    }
    Compress-Archive -Path $FolderPath -DestinationPath $ZipPath -CompressionLevel Optimal
    $sizeMb = [math]::Round((Get-Item -LiteralPath $ZipPath).Length / 1MB, 2)
    Write-Host "    -> $ZipPath ($sizeMb MB)"
}

function Write-Manifest {
    $lines = @(
        'StarCrystal pack-publish',
        "generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
        "publishDir: $PublishDir",
        "repoRoot: $RepoRoot",
        "outputDir: $OutputDir",
        '',
        'archives (in publish subdir):',
        '  release.zip',
        '  dbscripts.zip',
        '  idip-webclient.zip',
        '  release_h5.zip',
        '  unpack.sh             (deploy: run inside this subdir)',
        '',
        "bundle (tools/0publish/): $BundleZip"
    )
    Set-Content -LiteralPath $ManifestPath -Value $lines -Encoding UTF8
}

Write-Host "==> StarCrystal pack-publish ($PublishDir)"
Write-Host "    repo: $RepoRoot"
Write-Host "    out:  $OutputDir"

if (-not $SkipBuild) {
    Write-Host '==> build (tools/scripts/build.ps1)'
    Push-Location $RepoRoot
    try { & $BuildScript } finally { Pop-Location }
} else {
    Write-Host '==> skip build (-SkipBuild)'
}

if ($BuildIdip) {
    Write-Host '==> idip-webclient npm run build'
    Push-Location $IdipSrc
    try {
        if (-not (Test-Path 'node_modules')) {
            npm ci 2>$null
            if ($LASTEXITCODE -ne 0) { npm install }
        }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw 'npm run build failed' }
    } finally {
        Pop-Location
    }
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
Write-Host "==> stage -> $StagingRoot"
Remove-TreeIfExists $StagingRoot
New-Item -ItemType Directory -Force -Path $StagingRoot | Out-Null

$stageRelease = Join-Path $StagingRoot 'release'
$stageDbscripts = Join-Path $StagingRoot 'dbscripts'
$stageIdip = Join-Path $StagingRoot 'idip-webclient'
$stageReleaseH5 = Join-Path $StagingRoot 'release_h5'

Write-Host '==> pack release'
Copy-ReleaseBundle -Destination $stageRelease
Write-ZipFromFolder -FolderPath $stageRelease -ZipPath $ZipRelease

Write-Host '==> pack dbscripts'
Copy-DbscriptsBundle -Destination $stageDbscripts
Write-ZipFromFolder -FolderPath $stageDbscripts -ZipPath $ZipDbscripts

Write-Host '==> pack idip-webclient'
Copy-IdipBundle -Destination $stageIdip
Write-ZipFromFolder -FolderPath $stageIdip -ZipPath $ZipIdip

Write-Host '==> pack release_h5'
Copy-ReleaseH5Bundle -Destination $stageReleaseH5
Write-ZipFromFolder -FolderPath $stageReleaseH5 -ZipPath $ZipReleaseH5

Remove-TreeIfExists $StagingRoot
Write-Manifest

$UnpackSrc = Join-Path $PSScriptRoot 'unpack.sh'
$UnpackDst = Join-Path $OutputDir 'unpack.sh'
if (Test-Path -LiteralPath $UnpackSrc) {
    $unpackText = [IO.File]::ReadAllText($UnpackSrc) -replace "`r`n", "`n" -replace "`r", "`n"
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [IO.File]::WriteAllText($UnpackDst, $unpackText, $utf8NoBom)
    Write-Host "==> copy unpack.sh (LF) -> $UnpackDst"
} else {
    Write-Warning "Missing unpack.sh: $UnpackSrc"
}

Write-Host "==> pack bundle (publish subdir -> 0publish root)"
if (Test-Path -LiteralPath $BundleZip) {
    Remove-Item -LiteralPath $BundleZip -Force
}
Compress-Archive -Path $OutputDir -DestinationPath $BundleZip -CompressionLevel Optimal
$bundleMb = [math]::Round((Get-Item -LiteralPath $BundleZip).Length / 1MB, 2)
Write-Host "    -> $BundleZip ($bundleMb MB)"

Write-Host ''
Write-Host "Done."
Write-Host "  subdir:  $OutputDir"
Write-Host "    $ZipRelease"
Write-Host "    $ZipDbscripts"
Write-Host "    $ZipIdip"
Write-Host "    $ZipReleaseH5"
Write-Host "    $ManifestPath"
Write-Host "  bundle: $BundleZip"
