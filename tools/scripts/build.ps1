param(
    [string]$OutputName = "starcrystalsvr.exe"
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
$RepoRoot = Get-StarcrystalRepoRoot
$OutputPath = Join-Path $ReleaseRoot $OutputName
$ConfigTarget = Join-Path $ReleaseRoot "configs"
$AssetsTarget = Join-Path $ReleaseRoot "assets"

Push-Location $RepoRoot
try {
    if (-not (Test-Path -LiteralPath $ReleaseRoot)) {
        New-Item -ItemType Directory -Path $ReleaseRoot | Out-Null
    }

    if ([string]::IsNullOrWhiteSpace($env:GOPROXY)) {
        $env:GOPROXY = 'https://goproxy.cn,https://proxy.golang.org,direct'
        Write-Host 'GOPROXY was unset; using https://goproxy.cn,https://proxy.golang.org,direct'
    }

    Write-Host "go mod tidy (repo=$RepoRoot)"
    go mod tidy
    if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed with exit code $LASTEXITCODE" }

    Write-Host "go clean ./..."
    go clean ./...
    if ($LASTEXITCODE -ne 0) { throw "go clean failed with exit code $LASTEXITCODE" }

    if (Test-Path -LiteralPath $OutputPath) {
        $exeName = [System.IO.Path]::GetFileNameWithoutExtension($OutputPath)
        $running = Get-Process -Name $exeName -ErrorAction SilentlyContinue
        if ($running) {
            Write-Host "Stopping running process(es): $exeName"
            foreach ($p in $running) {
                Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
            }
            Start-Sleep -Milliseconds 300
        }
        Remove-Item -LiteralPath $OutputPath -Force
    }

    Write-Host "Building $OutputPath ..."
    go build -o $OutputPath ./cmd/api
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }

    if (-not (Test-Path $ConfigTarget)) {
        New-Item -ItemType Directory -Path $ConfigTarget | Out-Null
        Write-Warning "Created empty configs: $ConfigTarget"
    }
    if (-not (Test-Path $AssetsTarget)) {
        New-Item -ItemType Directory -Path $AssetsTarget | Out-Null
        Write-Warning "Created empty assets: $AssetsTarget"
    }

    $gamesConfigPath = Join-Path $ConfigTarget "games.json"
    if (Test-Path $gamesConfigPath) {
        try {
            $configText = Get-Content -LiteralPath $gamesConfigPath -Raw -Encoding UTF8
            $matches = [regex]::Matches($configText, '"entryUrl"\s*:\s*"([^"]+)"')
            foreach ($m in $matches) {
                $entryUrl = [string]$m.Groups[1].Value
                if ([string]::IsNullOrWhiteSpace($entryUrl) -or $entryUrl -match '^(https?:)?//') { continue }
                $pathOnly = ($entryUrl -split '[?#]')[0].Trim()
                $relativePath = $pathOnly.TrimStart('/')
                if ($relativePath.StartsWith('assets/')) { $relativePath = $relativePath.Substring(7) }
                $entryFile = Join-Path $AssetsTarget ($relativePath.Replace('/', '\'))
                if (-not (Test-Path $entryFile)) {
                    Write-Warning "Missing game entry file: $pathOnly -> $entryFile"
                }
            }
        } catch {
            Write-Warning "games.json entryUrl validation failed: $($_.Exception.Message)"
        }
    }

    Write-Host "Build complete: $OutputPath"
}
finally {
    Pop-Location
}
