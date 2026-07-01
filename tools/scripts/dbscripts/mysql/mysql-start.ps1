# Portable mysqld only; sets AUTH_MYSQL_DSN when the port listens.
param(
    [string]$BaseDir = "",
    [string]$DataDir = "",
    [int]$Port = 3306
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\dbscripts-config.ps1')
if ([string]::IsNullOrWhiteSpace($BaseDir)) { $BaseDir = Get-DefaultMysqlPortableBaseDir }
if ([string]::IsNullOrWhiteSpace($DataDir)) { $DataDir = Get-DefaultMysqlPortableDataDir }

$script:DefaultSqlHost = '127.0.0.1'
$script:DefaultDatabase = 'starcrystal_auth'
$script:DefaultAppUser = 'star_auth'
$script:DefaultAppPassword = 'jgyjgyjgy'

function Publish-SessionAuthMysqlDsn {
    param([int]$PortNumber)
    $cfgParts = Get-DbScriptsAuthMysqlDsnParts
    $dsn = $null
    if ($cfgParts) {
        $dsn = '{0}:{1}@tcp({2}:{3})/{4}?charset=utf8mb4&parseTime=true&loc=Local' -f `
            $cfgParts.User, $cfgParts.Password, $cfgParts.SqlHost, $PortNumber, $cfgParts.Database
        Write-Host ''
        Write-Host 'AUTH_MYSQL_DSN set from dbscripts/config/starcrystal.json port=' $PortNumber
    }
    if (-not $dsn) {
        $dsn = '{0}:{1}@tcp({2}:{3})/{4}?charset=utf8mb4&parseTime=true&loc=Local' -f `
            $script:DefaultAppUser, $script:DefaultAppPassword, $script:DefaultSqlHost, $PortNumber, $script:DefaultDatabase
        Write-Host ''
        Write-Host 'AUTH_MYSQL_DSN set (defaults; edit dbscripts/config/starcrystal.json or local.env).'
    }
    $env:AUTH_MYSQL_DSN = $dsn
    Save-DbScriptsLastAuthMysqlDsn -Dsn $dsn
    Write-Host $dsn
    Write-Host ('Schema: mysql\rebuild-auth-mysql.ps1 -SqlHost {0} -Port {1}' -f $script:DefaultSqlHost, $PortNumber)
}

$mysqld = Join-Path $BaseDir 'bin\mysqld.exe'
$MysqlRootParent = Split-Path -Parent $BaseDir
$logFile = Join-Path $MysqlRootParent 'mysql.log'
$stdoutLog = Join-Path $MysqlRootParent 'mysql.stdout.log'
$stderrLog = Join-Path $MysqlRootParent 'mysql.stderr.log'
$MysqlPidPath = Join-Path $MysqlRootParent "mysqld-$Port.pid"

function Test-TcpPortOpen {
    param([string]$HostName, [int]$PortNumber, [int]$TimeoutMs = 800)
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect($HostName, $PortNumber, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)
        if (-not $ok) { return $false }
        $client.EndConnect($iar) | Out-Null
        return $true
    } catch { return $false }
    finally { $client.Close() }
}

if (-not (Test-Path -LiteralPath $mysqld)) {
    throw "mysqld not found: $mysqld (set MYSQL_PORTABLE_BASE in local.env or place under dbscripts/mysql/portable)"
}
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

if (-not (Test-Path -LiteralPath (Join-Path $DataDir 'mysql'))) {
    Write-Host 'Initializing data directory...'
    $initArgs = @('--initialize-insecure', "--basedir=$BaseDir", "--datadir=$DataDir")
    $init = Start-Process -FilePath $mysqld -ArgumentList $initArgs -Wait -PassThru -NoNewWindow
    if ($init.ExitCode -ne 0) { throw "mysqld --initialize-insecure failed (exit $($init.ExitCode))" }
}

if (Test-TcpPortOpen -HostName '127.0.0.1' -PortNumber $Port) {
    Write-Host "MySQL is already running on port $Port."
    Publish-SessionAuthMysqlDsn -PortNumber $Port
    exit 0
}

Write-Host "Starting portable MySQL on port $Port ..."
$startArgs = @(
    "--basedir=$BaseDir", "--datadir=$DataDir", "--port=$Port",
    '--bind-address=127.0.0.1', "--log-error=$logFile"
)
$proc = Start-Process -FilePath $mysqld -ArgumentList $startArgs -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutLog -RedirectStandardError $stderrLog -PassThru
if ($proc) { Set-Content -LiteralPath $MysqlPidPath -Value $proc.Id -Encoding UTF8 }

$isUp = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    if (Test-TcpPortOpen -HostName '127.0.0.1' -PortNumber $Port) { $isUp = $true; break }
}
if (-not $isUp) { throw "MySQL failed to open TCP port $Port within 30s. See $logFile" }
Write-Host "MySQL started successfully. PID=$($proc.Id)"
Publish-SessionAuthMysqlDsn -PortNumber $Port
