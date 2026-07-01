# 临时启动本机 API，请求一次发验证码，用于验证 release/configs/smtp.local.env 中 SMTP（监听端口见 starcrystal.json）
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
$envFile = Join-Path $ReleaseRoot "configs\smtp.local.env"
$exe = Join-Path $ReleaseRoot "starcrystalsvr.exe"

if (-not (Test-Path $exe)) { throw "Build first: .\scripts\build.ps1" }
if (-not (Test-Path $envFile)) { throw "Not found: $envFile" }

. (Join-Path $PSScriptRoot "Import-AuthEnvFromFile.ps1")
Import-AuthEnvFromFile -Path $envFile
if ([string]::IsNullOrWhiteSpace($env:AUTH_MYSQL_DSN)) {
    $dsnCfg = Get-StarcrystalAuthMysqlDsn -ReleaseRoot $ReleaseRoot
    if (-not [string]::IsNullOrWhiteSpace($dsnCfg)) {
        $env:AUTH_MYSQL_DSN = $dsnCfg
    }
}

$env:GAMES_CONFIG = Join-Path $ReleaseRoot "configs\games.json"
$baseUrl = Get-StarcrystalApiBaseUrl -ReleaseRoot $ReleaseRoot

Write-Host "Starting API $baseUrl (SMTP: $($env:AUTH_SMTP_ADDR)); MySQL DSN: env AUTH_MYSQL_DSN or starcrystal.json authMysqlDsn"

$p = Start-Process -FilePath $exe -WorkingDirectory $releaseDir -PassThru -WindowStyle Hidden
try {
    Start-Sleep -Seconds 3
    $line = Get-Content -LiteralPath $envFile -Encoding UTF8 -ErrorAction Stop |
        Where-Object { $_ -match "^\s*AUTH_SMTP_USER=" } | Select-Object -First 1
    if (-not $line) { throw "AUTH_SMTP_USER= not found in $envFile" }
    $acc = ($line -replace "^\s*AUTH_SMTP_USER=\s*", "").Trim().Trim([char]0x22)
    if ($acc -eq "") { throw "empty AUTH_SMTP_USER" }
    $body = @{ account = $acc } | ConvertTo-Json -Compress
    $uri = "$baseUrl/api/v1/auth/sms/send"
    Write-Host "POST $uri (account= same mailbox as smtp test)"
    $r = Invoke-RestMethod -Uri $uri -Method Post -ContentType "application/json; charset=utf-8" -Body $body
    $r | ConvertTo-Json -Depth 5
} finally {
    if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
