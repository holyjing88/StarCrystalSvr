# dbscripts 本地配置（仅引用 dbscripts/ 目录内资源）。
# Dot-source: . (Join-Path $DbScriptsConfigDir 'dbscripts-config.ps1')
# UTF-8 BOM recommended for Windows PowerShell 5.x.

$script:DbScriptsRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

function Get-DbScriptsRoot { $script:DbScriptsRoot }

function Import-DbScriptsLocalEnv {
    $f = Join-Path $script:DbScriptsRoot 'local.env'
    if (-not (Test-Path -LiteralPath $f)) { return }
    foreach ($line in Get-Content -LiteralPath $f -Encoding UTF8) {
        $t = "$line".Trim()
        if ($t -eq '' -or $t.StartsWith('#')) { continue }
        $eq = $t.IndexOf('=')
        if ($eq -lt 1) { continue }
        $key = $t.Substring(0, $eq).Trim()
        $val = $t.Substring($eq + 1).Trim()
        if ($val.StartsWith('"') -and $val.EndsWith('"')) { $val = $val.Substring(1, $val.Length - 2) }
        if ($val.StartsWith("'") -and $val.EndsWith("'")) { $val = $val.Substring(1, $val.Length - 2) }
        if ([string]::IsNullOrWhiteSpace($key)) { continue }
        if ($null -ne [Environment]::GetEnvironmentVariable($key, 'Process')) { continue }
        [Environment]::SetEnvironmentVariable($key, $val, 'Process')
    }
}
Import-DbScriptsLocalEnv

function Get-DbScriptsConfigJsonPath {
    Join-Path $script:DbScriptsRoot 'config\starcrystal.json'
}

function Get-DbScriptsSqlDir {
    Join-Path $script:DbScriptsRoot 'sql'
}

function Get-DefaultMysqlPortableBaseDir {
    $envBase = [Environment]::GetEnvironmentVariable('MYSQL_PORTABLE_BASE', 'Process')
    if (-not [string]::IsNullOrWhiteSpace($envBase)) { return $envBase.Trim() }
    $portable = Join-Path $script:DbScriptsRoot 'mysql\portable'
    if (Test-Path -LiteralPath $portable) { return (Resolve-Path -LiteralPath $portable).Path }
    throw "MySQL portable not found. Set MYSQL_PORTABLE_BASE in local.env or place under dbscripts/mysql/portable"
}

function Get-DefaultMysqlPortableDataDir {
    foreach ($name in @('MYSQL_PORTABLE_DATA', 'MYSQL_DATA_DIR')) {
        $v = [Environment]::GetEnvironmentVariable($name, 'Process')
        if (-not [string]::IsNullOrWhiteSpace($v)) { return $v.Trim() }
    }
    Join-Path $script:DbScriptsRoot 'data\mysql'
}

function Parse-AuthMysqlDsnString {
    param([Parameter(Mandatory)][string]$Dsn)
    $dsn = "$Dsn".Trim()
    if ($dsn -eq '') { return $null }
    $at = [char]64
    $re = "^([^:]+):([^$at]*)$at" + 'tcp\(([^:]+):(\d+)\)/([^?]+)'
    if (-not ($dsn -match $re)) { return $null }
    return @{
        User     = $Matches[1]
        Password = $Matches[2]
        SqlHost  = $Matches[3]
        Port     = [int]$Matches[4]
        Database = $Matches[5]
    }
}

function Get-DbScriptsAuthMysqlDsnParts {
    $cfg = Get-DbScriptsConfigJsonPath
    if (Test-Path -LiteralPath $cfg) {
        try {
            $j = Get-Content -LiteralPath $cfg -Raw -Encoding UTF8 | ConvertFrom-Json
            if ($null -ne $j.authMysqlDsn) {
                $parts = Parse-AuthMysqlDsnString "$($j.authMysqlDsn)".Trim()
                if ($parts) { return $parts }
            }
        } catch { }
    }
    $user = [Environment]::GetEnvironmentVariable('MYSQL_AUTH_USER', 'Process')
    $pass = [Environment]::GetEnvironmentVariable('MYSQL_AUTH_PASSWORD', 'Process')
    if (-not [string]::IsNullOrWhiteSpace($user) -and $null -ne $pass) {
        return @{
            User     = $user
            Password = $pass
            SqlHost  = if ($env:MYSQL_HOST) { $env:MYSQL_HOST } else { '127.0.0.1' }
            Port     = if ($env:MYSQL_PORT) { [int]$env:MYSQL_PORT } else { 3306 }
            Database = if ($env:MYSQL_AUTH_DB) { $env:MYSQL_AUTH_DB } else { 'starcrystal_auth' }
        }
    }
    return $null
}

function Save-DbScriptsLastAuthMysqlDsn {
    param([Parameter(Mandatory)][string]$Dsn)
    $dsn = "$Dsn".Trim()
    if ($dsn -eq '') { return }
    $dir = Join-Path $script:DbScriptsRoot 'log'
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Set-Content -LiteralPath (Join-Path $dir 'last-auth-mysql-dsn.txt') -Value $dsn -Encoding UTF8
}

function Get-DbScriptsRedisSettings {
    $hostName = '127.0.0.1'
    $port = 6379
    $db = 0
    $password = ''
    $cfg = Get-DbScriptsConfigJsonPath
    if (Test-Path -LiteralPath $cfg) {
        try {
            $j = Get-Content -LiteralPath $cfg -Raw -Encoding UTF8 | ConvertFrom-Json
            if ($j.redisAddr) {
                $addr = "$($j.redisAddr)".Trim()
                if ($addr -match '^(.+):(\d+)$') {
                    $hostName = $Matches[1]
                    $port = [int]$Matches[2]
                } elseif ($addr) {
                    $hostName = $addr
                }
            }
            if ($null -ne $j.redisPassword) { $password = "$($j.redisPassword)" }
            if ($null -ne $j.redisDb) { $db = [int]$j.redisDb }
        } catch { }
    }
    if ($env:REDIS_HOST) { $hostName = $env:REDIS_HOST.Trim() }
    if ($env:REDIS_PORT) { $port = [int]$env:REDIS_PORT }
    if ($env:REDIS_DB) { $db = [int]$env:REDIS_DB }
    if ($null -ne $env:REDIS_PASSWORD) { $password = $env:REDIS_PASSWORD }
    return @{
        RedisHost     = $hostName
        RedisPort     = $port
        RedisDb       = $db
        RedisPassword = $password
    }
}
