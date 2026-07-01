param(
    [string]$BaseDir = "",
    [int]$Port = 3306,
    [string]$MysqlPidPath = ''
)

$ErrorActionPreference = "SilentlyContinue"
. (Join-Path $PSScriptRoot "..\dbscripts-config.ps1")
if ([string]::IsNullOrWhiteSpace($BaseDir)) { $BaseDir = Get-DefaultMysqlPortableBaseDir }

if ([string]::IsNullOrWhiteSpace($MysqlPidPath)) {
    $MysqlPidPath = Join-Path (Split-Path -Parent $BaseDir) "mysqld-$Port.pid"
}

$mysqldExe = Join-Path $BaseDir "bin\mysqld.exe"
$stoppedAny = $false
$stoppedPids = @()

Write-Host "Stopping portable MySQL..."
Write-Host "BaseDir=$BaseDir Port=$Port MysqlPidPath=$MysqlPidPath"

if (Test-Path $MysqlPidPath) {
    $rawPidFromFile = (Get-Content -LiteralPath $MysqlPidPath -Raw -Encoding UTF8).Trim()
    $filePid = 0
    if ([int]::TryParse($rawPidFromFile, [ref]$filePid)) {
        $p = Get-Process -Id $filePid -ErrorAction SilentlyContinue
        if ($p) {
            Write-Host "Stopping by PID file: PID=$filePid"
            Stop-Process -Id $filePid -Force
            $stoppedAny = $true
            $stoppedPids += $filePid
        }
    }
    Remove-Item $MysqlPidPath -Force -ErrorAction SilentlyContinue
}

$procs = Get-Process -Name mysqld -ErrorAction SilentlyContinue
foreach ($p in $procs) {
    if ($stoppedPids -contains $p.Id) { continue }
    Write-Host "Stopping mysqld PID=$($p.Id)"
    Stop-Process -Id $p.Id -Force
    $stoppedAny = $true
}

if ($stoppedAny) { Write-Host "MySQL stopped." } else { Write-Host "No MySQL process found." }
