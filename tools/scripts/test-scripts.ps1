<#
.SYNOPSIS
  tools/scripts 运维脚本验收（Windows）。静态检查默认执行；加 -Live 做启停实测。

.EXAMPLE
  cd server-go
  .\tools\scripts\test-scripts.ps1
  .\tools\scripts\test-scripts.ps1 -Live
  $env:SCRIPTS_TEST_FULL = '1'; .\scripts\test-scripts.ps1 -Live
#>
[CmdletBinding()]
param(
    [switch]$Live,
    [switch]$Build
)

$ErrorActionPreference = 'Stop'
$ScriptRoot = $PSScriptRoot
$TestDir = Join-Path $ScriptRoot 'test'
. (Join-Path $TestDir 'Test-Helpers.ps1')
. (Join-Path $ScriptRoot 'StarcrystalConfig.ps1')

$ReleaseRoot = Get-StarcrystalReleaseRoot
$RepoRoot = Get-StarcrystalRepoRoot
$liveOn = $Live -or ($env:SCRIPTS_TEST_LIVE -eq '1')
$fullOn = ($env:SCRIPTS_TEST_FULL -eq '1')
$redisTestPort = if ($env:SCRIPTS_TEST_REDIS_PORT) { [int]$env:SCRIPTS_TEST_REDIS_PORT } else { 16379 }
$results = @()

$winEntryScripts = @(
    'dbscripts\startdb.ps1', 'dbscripts\stopdb.ps1',
    'dbscripts\mysql\mysql-start.ps1', 'dbscripts\mysql\mysql-stop.ps1',
    'startallsvr.ps1', 'stopallsvr.ps1',
    'build.ps1', 'all.ps1', 'dbscripts\mysql\rebuild-auth-mysql.ps1',
    'dbscripts\redis\redis-start.ps1', 'dbscripts\redis\redis-stop.ps1'
)
$releaseApiScripts = @('startsvr.ps1', 'stopsvr.ps1')

# --- Static ---
$results += Invoke-ScriptTestCase 'SCR-S-CONFIG-W' {
    $r = Get-StarcrystalReleaseRoot
    $g = Get-StarcrystalRepoRoot
    if (-not (Test-Path -LiteralPath (Join-Path $r 'configs\starcrystal.json'))) {
        throw 'configs/starcrystal.json missing'
    }
    if ($g -ne $RepoRoot) { throw "RepoRoot mismatch: $g" }
}

$results += Invoke-ScriptTestCase 'SCR-S-ENTRY-W' {
    foreach ($name in $winEntryScripts) {
        $p = Join-Path $ScriptRoot $name
        if (-not (Test-Path -LiteralPath $p)) { throw "missing tools/scripts/$name" }
        $ok, $err = Test-PowerShellScriptParses -Path $p
        if (-not $ok) { throw "${name}: $err" }
    }
    foreach ($name in $releaseApiScripts) {
        $p = Join-Path $ReleaseRoot $name
        if (-not (Test-Path -LiteralPath $p)) { throw "missing release/$name" }
        $ok, $err = Test-PowerShellScriptParses -Path $p
        if (-not $ok) { throw "release/${name}: $err" }
    }
}

# --- Live optional ---
$redisExe = Join-Path $RepoRoot 'redis\redis-server.exe'
$hasRedis = Test-Path -LiteralPath $redisExe
$mysqlBase = Get-DefaultMysqlPortableBaseDir
$mysqlExe = Join-Path $mysqlBase 'bin\mysqld.exe'
$hasMysql = Test-Path -LiteralPath $mysqlExe
$svrExe = Join-Path $ReleaseRoot 'starcrystalsvr.exe'
$hasSvr = Test-Path -LiteralPath $svrExe

$results += Invoke-ScriptTestCase 'SCR-L-REDIS' -Skip:(-not $liveOn) {
    if (-not $hasRedis) { throw 'skip:no-redis-exe' }
    try {
        & (Join-Path $ScriptRoot 'dbscripts\redis\redis-stop.ps1') -Port $redisTestPort
    } catch { }
    & (Join-Path $ScriptRoot 'dbscripts\redis\redis-start.ps1') -Port $redisTestPort
    Start-Sleep -Seconds 1
    if (-not (Test-TcpPortOpen -Port $redisTestPort)) { throw "Redis port $redisTestPort not open" }
    & (Join-Path $ScriptRoot 'dbscripts\redis\redis-stop.ps1') -Port $redisTestPort
}

$results += Invoke-ScriptTestCase 'SCR-L-MYSQL' -Skip:(-not $liveOn) {
    if (-not $hasMysql) { throw 'skip:no-mysqld' }
    $port = if ($env:SCRIPTS_TEST_MYSQL_PORT) { [int]$env:SCRIPTS_TEST_MYSQL_PORT } else { 3306 }
    try {
        & (Join-Path $ScriptRoot 'dbscripts\mysql\mysql-stop.ps1') -Port $port
    } catch { }
    & (Join-Path $ScriptRoot 'dbscripts\mysql\mysql-start.ps1') -Port $port
    Start-Sleep -Seconds 2
    if (-not (Test-TcpPortOpen -Port $port)) { throw "MySQL port $port not open" }
    & (Join-Path $ScriptRoot 'dbscripts\mysql\mysql-stop.ps1') -Port $port
}

$results += Invoke-ScriptTestCase 'SCR-L-STARTDB' -Skip:(-not $liveOn) {
    if (-not $hasRedis) { throw 'skip:no-redis-exe' }
    try { & (Join-Path $ScriptRoot 'dbscripts\stopdb.ps1') } catch { }
    $dbArgs = @{ RedisPort = $redisTestPort }
    if ($hasMysql -and $env:SCRIPTS_TEST_MYSQL_PORT) {
        $dbArgs['MysqlPort'] = [int]$env:SCRIPTS_TEST_MYSQL_PORT
    }
    & (Join-Path $ScriptRoot 'dbscripts\startdb.ps1') @dbArgs
    Start-Sleep -Seconds 2
    if (-not (Test-TcpPortOpen -Port $redisTestPort)) { throw 'Redis not up after startdb' }
    & (Join-Path $ScriptRoot 'dbscripts\stopdb.ps1') -RedisPort $redisTestPort
}

$results += Invoke-ScriptTestCase 'SCR-L-STARTSVR' -Skip:(-not $liveOn) {
    if (-not $hasSvr) { throw 'skip:no-starcrystalsvr.exe' }
    try { & (Join-Path $ScriptRoot 'stopsvr.ps1') } catch { }
    if ($hasMysql) {
        & (Join-Path $ScriptRoot 'dbscripts\mysql\mysql-start.ps1')
        Start-Sleep -Seconds 2
    }
    & (Join-Path $ReleaseRoot 'startsvr.ps1') -AuthSmsMock 1
    Start-Sleep -Seconds 2
    $apiPort = Get-StarcrystalApiListenPort -ReleaseRoot $ReleaseRoot
    $url = "http://127.0.0.1:$apiPort/healthz"
    try {
        $resp = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 5
        if ($resp.StatusCode -ne 200) { throw "healthz status $($resp.StatusCode)" }
    } catch {
        throw "healthz failed: $($_.Exception.Message)"
    }
    & (Join-Path $ReleaseRoot 'stopsvr.ps1')
    if ($hasMysql) {
        try { & (Join-Path $ScriptRoot 'dbscripts\mysql\mysql-stop.ps1') } catch { }
    }
}

$results += Invoke-ScriptTestCase 'SCR-L-STARTALL' -Skip:(-not ($liveOn -and $fullOn)) {
    if (-not $hasSvr -or -not $hasRedis) { throw 'skip:need svr+redis' }
    try { & (Join-Path $ScriptRoot 'stopallsvr.ps1') -RedisPort $redisTestPort } catch { }
    & (Join-Path $ScriptRoot 'startallsvr.ps1') -RedisPort $redisTestPort
    Start-Sleep -Seconds 3
    $apiPort = Get-StarcrystalApiListenPort -ReleaseRoot $ReleaseRoot
    $url = "http://127.0.0.1:$apiPort/healthz"
    $resp = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 5
    if ($resp.StatusCode -ne 200) { throw "healthz $($resp.StatusCode)" }
    & (Join-Path $ScriptRoot 'stopallsvr.ps1') -RedisPort $redisTestPort
}

$results += Invoke-ScriptTestCase 'SCR-S-BUILD' -Skip:(-not $Build) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'skip:no-go' }
    & (Join-Path $ScriptRoot 'build.ps1')
    if (-not (Test-Path -LiteralPath $svrExe)) { throw 'build did not produce starcrystalsvr.exe' }
}

# Mark skip messages as Skipped not Fail
$final = foreach ($r in $results) {
    if (-not $r.Passed -and $r.Message -match '^skip:') {
        New-ScriptTestResult -Id $r.Id -Passed $true -Skipped $true -Message $r.Message
    } else { $r }
}

$ok = Write-ScriptTestReport -Results $final
if (-not $ok) { exit 1 }
exit 0
