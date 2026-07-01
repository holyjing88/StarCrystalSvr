# 验收 g001 整包：/healthz、GET /api/v1/games 三字段、tar 可下载且 sha256 一致。
param(
    [string]$BaseUrl = $env:STARCHRYSTAL_API_BASE_URL,
    [string]$GameId = "g001"
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $BaseUrl = "http://127.0.0.1:8080"
}
$BaseUrl = $BaseUrl.Trim().TrimEnd("/")

function Fail($msg) {
    Write-Error $msg
    exit 1
}

Write-Host "API base: $BaseUrl"

try {
    $health = Invoke-WebRequest -Uri "$BaseUrl/healthz" -UseBasicParsing -TimeoutSec 8
    if ($health.StatusCode -lt 200 -or $health.StatusCode -ge 300) {
        Fail "healthz HTTP $($health.StatusCode)"
    }
    Write-Host "OK healthz"
}
catch {
    Fail "healthz unreachable: $_"
}

$gamesUrl = "$BaseUrl/api/v1/games?appVersion=1.0.0.0&platform=android&lang=zh&channel=ChannelType_GooglePlay"
try {
    $resp = Invoke-WebRequest -Uri $gamesUrl -UseBasicParsing -TimeoutSec 15
}
catch {
    Fail "GET /api/v1/games failed: $_"
}

$json = $resp.Content | ConvertFrom-Json
if ($null -eq $json.data -or $null -eq $json.data.games) {
    Fail "games list missing in response"
}

$game = $json.data.games | Where-Object { $_.gameId -eq $GameId } | Select-Object -First 1
if ($null -eq $game) {
    Fail "gameId $GameId not found in games list"
}

Write-Host "Found $GameId entryUrl=$($game.entryUrl) downloadUrl=$($game.downloadUrl) bytes=$($game.packageBytes)"

if ([string]::IsNullOrWhiteSpace($game.downloadUrl)) { Fail "downloadUrl empty" }
if ($game.packageBytes -le 0) { Fail "packageBytes invalid" }
if ($game.downloadSha256 -notmatch '^[a-f0-9]{64}$') { Fail "downloadSha256 invalid" }

$tarUrl = $game.downloadUrl
if ($tarUrl -notmatch '^https?://') {
    $tarUrl = "$BaseUrl/$($tarUrl.TrimStart('/'))"
}

$tmp = Join-Path $env:TEMP ("g001_pkg_" + [Guid]::NewGuid().ToString("N") + ".tar.gz")
try {
    Invoke-WebRequest -Uri $tarUrl -OutFile $tmp -UseBasicParsing -TimeoutSec 60
}
catch {
    Fail "download tar failed: $_"
}

$len = (Get-Item $tmp).Length
if ($len -ne [long]$game.packageBytes) {
    Fail "size mismatch: file=$len expected=$($game.packageBytes)"
}
Write-Host "OK tar size $len bytes"

$hash = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLowerInvariant()
if ($hash -ne $game.downloadSha256.ToLowerInvariant()) {
    Fail "sha256 mismatch: got=$hash expected=$($game.downloadSha256)"
}
Write-Host "OK sha256 $hash"
Write-Host "PASS g001 H5 package verification on $BaseUrl"
