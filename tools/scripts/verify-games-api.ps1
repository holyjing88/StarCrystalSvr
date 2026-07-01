$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
$RepoRoot = Get-StarcrystalRepoRoot
$dsnCfg = Get-StarcrystalAuthMysqlDsn -ReleaseRoot $ReleaseRoot
if (-not [string]::IsNullOrWhiteSpace($dsnCfg)) {
    $env:AUTH_MYSQL_DSN = $dsnCfg.Trim()
} elseif ([string]::IsNullOrWhiteSpace($env:AUTH_MYSQL_DSN)) {
    throw "AUTH_MYSQL_DSN missing: set env or authMysqlDsn in configs/starcrystal.json"
}
$gamesConfigEnv = Join-Path $ReleaseRoot "configs\games.json"
Write-Host "Starting API server (go run from repo) ..."
Write-Host "GAMES_CONFIG=$gamesConfigEnv"
$mysqlDsnForJob = $env:AUTH_MYSQL_DSN
$job = Start-Job -ScriptBlock {
    param($configPath, $workDir, $mysqlDsn)
    Set-Location $workDir
    $env:GAMES_CONFIG = $configPath
    $env:AUTH_MYSQL_DSN = $mysqlDsn
    go run ./cmd/api
} -ArgumentList $gamesConfigEnv, $RepoRoot, $mysqlDsnForJob

try {
    Start-Sleep -Milliseconds 1200
    $baseUrl = Get-StarcrystalApiBaseUrl -ReleaseRoot $ReleaseRoot
    $health = Invoke-RestMethod -Uri "$baseUrl/healthz" -Method Get
    if ($health.code -ne 0) { throw "healthz failed" }
    $respV1 = Invoke-RestMethod -Uri "$baseUrl/api/v1/games?appVersion=1.0.0&platform=android" -Method Get
    if ($respV1.code -ne 0 -or -not $respV1.data.games) { throw "games query failed" }
    Write-Host "Protocol check passed. games(1.0.0): $($respV1.data.games.Count)"
} finally {
    Stop-Job $job -ErrorAction SilentlyContinue | Out-Null
    Remove-Job $job -ErrorAction SilentlyContinue | Out-Null
}
