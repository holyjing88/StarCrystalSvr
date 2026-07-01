param(
    [string]$AuthSmsMock = "1",
    [string]$AuthMySqlDsn = "",
    [switch]$WritePidFile = $true
)

$ErrorActionPreference = "Stop"
$ToolsScripts = Join-Path (Split-Path -Parent $PSScriptRoot) "tools\scripts"
. (Join-Path $ToolsScripts "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
$exePath = Join-Path $ReleaseRoot "starcrystalsvr.exe"
if (-not (Test-Path -LiteralPath $exePath)) {
    throw "server exe not found: $exePath (run ..\tools\scripts\build.ps1 first)"
}

$env:GAMES_CONFIG = Join-Path $ReleaseRoot "configs\games.json"
$env:CHANNEL_TEXTS_CONFIG = Join-Path $ReleaseRoot "configs\channel_texts.json"
$env:STARCrystal_CONFIG = Join-Path $ReleaseRoot "configs\starcrystal.json"
$goLogDir = Join-Path $ReleaseRoot "log"
if (-not (Test-Path -LiteralPath $goLogDir)) {
    New-Item -ItemType Directory -Path $goLogDir | Out-Null
}

$oldSmsMock = $env:AUTH_SMS_MOCK
$oldMySqlDsn = $env:AUTH_MYSQL_DSN
$env:AUTH_SMS_MOCK = $AuthSmsMock

$cfgDsn = Get-StarcrystalAuthMysqlDsn -ReleaseRoot $ReleaseRoot
$resolvedDsn = if (-not [string]::IsNullOrWhiteSpace($AuthMySqlDsn)) { $AuthMySqlDsn.Trim() }
    elseif (-not [string]::IsNullOrWhiteSpace($env:AUTH_MYSQL_DSN)) { $env:AUTH_MYSQL_DSN.Trim() }
    elseif (-not [string]::IsNullOrWhiteSpace($cfgDsn)) { $cfgDsn.Trim() }
    else { "" }

if (-not [string]::IsNullOrWhiteSpace($resolvedDsn)) {
    $env:AUTH_MYSQL_DSN = $resolvedDsn
}

$listenHint = Get-StarcrystalApiBaseUrl -ReleaseRoot $ReleaseRoot
Write-Host "Starting starcrystalsvr (cwd=$ReleaseRoot; URL ~ $listenHint) AUTH_SMS_MOCK=$AuthSmsMock"
if (-not [string]::IsNullOrWhiteSpace($resolvedDsn)) {
    Write-Host "AUTH_MYSQL_DSN: env / -AuthMySqlDsn / configs authMysqlDsn"
} else {
    Write-Host "AUTH_MYSQL_DSN empty — server reads configs/starcrystal.json authMysqlDsn"
}

$dsnToRecord = if (-not [string]::IsNullOrWhiteSpace($resolvedDsn)) { $resolvedDsn.Trim() }
    elseif (-not [string]::IsNullOrWhiteSpace($cfgDsn)) { $cfgDsn.Trim() }
    else { "" }
if (-not [string]::IsNullOrWhiteSpace($dsnToRecord)) {
    Save-LastAuthMysqlDsnForScripts -ReleaseRoot $ReleaseRoot -Dsn $dsnToRecord
    Write-Host "Recorded DSN: $(Get-LastAuthMysqlDsnFilePath -ReleaseRoot $ReleaseRoot)"
}
Write-Host "Logs: log\starcrystalsvr.log + starcrystalsvr_error.log"

$smtpEnvFile = Join-Path $ReleaseRoot "configs\smtp.local.env"
if (Test-Path -LiteralPath $smtpEnvFile) {
    Get-Content -LiteralPath $smtpEnvFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $eq = $line.IndexOf("=")
        if ($eq -lt 1) { return }
        $key = $line.Substring(0, $eq).Trim()
        $val = $line.Substring($eq + 1).Trim()
        if ($key) { Set-Item -Path "Env:$key" -Value $val }
    }
}

$pidFile = Join-Path $ReleaseRoot "starcrystalsvr.pid"
if ($WritePidFile -and (Test-Path -LiteralPath $pidFile)) {
    $oldPidText = (Get-Content -Path $pidFile -Raw).Trim()
    $oldPid = 0
    if ([int]::TryParse($oldPidText, [ref]$oldPid)) {
        $oldProc = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
        if ($oldProc -and $oldProc.ProcessName -eq "starcrystalsvr") {
            Write-Host "starcrystalsvr is already running. PID=$oldPid"
            exit 0
        }
    }
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
}

$oldApiAddrShell = $env:API_ADDR
Remove-Item Env:\API_ADDR -ErrorAction SilentlyContinue

$proc = Start-Process -FilePath $exePath `
    -WorkingDirectory $ReleaseRoot `
    -WindowStyle Hidden `
    -PassThru

if ($null -ne $oldApiAddrShell) { $env:API_ADDR = $oldApiAddrShell }
if ($WritePidFile) {
    Set-Content -Path $pidFile -Value $proc.Id
}

$env:AUTH_SMS_MOCK = $oldSmsMock
$env:AUTH_MYSQL_DSN = $oldMySqlDsn

Write-Host "Started starcrystalsvr. PID=$($proc.Id)"
