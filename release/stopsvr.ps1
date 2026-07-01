param(
    [string]$ProcessName = "starcrystalsvr",
    [int]$Port = 0,
    [string]$PidFile = "",
    [switch]$Force
)

$ErrorActionPreference = "SilentlyContinue"
$ToolsScripts = Join-Path (Split-Path -Parent $PSScriptRoot) "tools\scripts"
. (Join-Path $ToolsScripts "StarcrystalConfig.ps1")
$ReleaseRoot = Get-StarcrystalReleaseRoot
if ($Port -le 0) {
    $Port = Get-StarcrystalApiListenPort -ReleaseRoot $ReleaseRoot
}
if ([string]::IsNullOrWhiteSpace($PidFile)) {
    $PidFile = Join-Path $ReleaseRoot "starcrystalsvr.pid"
}

Write-Host "Stopping starcrystalsvr..."
Write-Host "ProcessName=$ProcessName Port=$Port PidFile=$PidFile"

$stoppedAny = $false
$stoppedPids = @()

if (Test-Path $PidFile) {
    $pidText = (Get-Content -Path $PidFile -Raw).Trim()
    $filePid = 0
    if ([int]::TryParse($pidText, [ref]$filePid)) {
        $p = Get-Process -Id $filePid -ErrorAction SilentlyContinue
        if ($p) {
            Write-Host "Stopping by PID file: PID=$filePid"
            if ($Force) { Stop-Process -Id $filePid -Force } else { Stop-Process -Id $filePid }
            $stoppedAny = $true
            $stoppedPids += $filePid
        }
    }
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
}

foreach ($p in (Get-Process -Name $ProcessName -ErrorAction SilentlyContinue)) {
    if ($stoppedPids -contains $p.Id) { continue }
    Write-Host "Stopping by name: PID=$($p.Id)"
    if ($Force) { Stop-Process -Id $p.Id -Force } else { Stop-Process -Id $p.Id }
    $stoppedAny = $true
    $stoppedPids += $p.Id
}

$pidsByPort = @()
try {
    $conn = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction Stop
    foreach ($c in $conn) {
        if ($c.OwningProcess -and ($pidsByPort -notcontains $c.OwningProcess)) {
            $pidsByPort += $c.OwningProcess
        }
    }
} catch {
    foreach ($line in (netstat -ano -p tcp)) {
        if ($line -match "LISTENING\s+(\d+)$" -and $line -match "[:\.]$Port\s+") {
            $pid = [int]$Matches[1]
            if ($pid -and ($pidsByPort -notcontains $pid)) { $pidsByPort += $pid }
        }
    }
}

foreach ($pid in $pidsByPort) {
    if ($stoppedPids -contains $pid) { continue }
    $p = Get-Process -Id $pid -ErrorAction SilentlyContinue
    if ($p) {
        Write-Host "Stopping by port: PID=$pid"
        Stop-Process -Id $pid -Force
        $stoppedAny = $true
    }
}

if ($stoppedAny) { Write-Host "starcrystalsvr stopped."; exit 0 }
Write-Host "No starcrystalsvr process found."
exit 0
