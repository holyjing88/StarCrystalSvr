# Dot-source: . (Join-Path $PSScriptRoot "StarcrystalConfig.ps1")
# Scripts live under server-go/tools/scripts/; runtime bundle is server-go/release/ (starcrystalsvr).
# release/startsvr.ps1 dot-sources ..\tools\scripts\StarcrystalConfig.ps1
# UTF-8 BOM recommended for this file on Windows PowerShell 5.x.

function Get-StarcrystalScriptsRoot {
    $PSScriptRoot
}

function Get-StarcrystalReleaseRoot {
    param([string]$ReleaseRoot = "")
    if (-not [string]::IsNullOrWhiteSpace($ReleaseRoot)) {
        return (Resolve-Path -LiteralPath $ReleaseRoot).Path
    }
    $cfgBesideRelease = Join-Path $PSScriptRoot "..\configs\starcrystal.json"
    if (Test-Path -LiteralPath $cfgBesideRelease) {
        return (Resolve-Path -LiteralPath (Split-Path -Parent $PSScriptRoot)).Path
    }
    $cfgUnderRelease = Join-Path $PSScriptRoot "..\..\release\configs\starcrystal.json"
    if (Test-Path -LiteralPath $cfgUnderRelease) {
        return (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..\..\release")).Path
    }
    throw "Cannot locate release/configs/starcrystal.json from $PSScriptRoot"
}

function Get-StarcrystalRepoRoot {
    param([string]$ReleaseRoot = "")
    $rel = Get-StarcrystalReleaseRoot $ReleaseRoot
    return (Split-Path -Parent $rel)
}

function Get-DefaultMysqlPortableBaseDir {
    param([string]$ReleaseRoot = "")
    $envBase = [Environment]::GetEnvironmentVariable("MYSQL_PORTABLE_BASE", "Process")
    if (-not [string]::IsNullOrWhiteSpace($envBase)) { return $envBase.Trim() }
    $repo = Get-StarcrystalRepoRoot $ReleaseRoot
    $gamesRoot = Split-Path -Parent (Split-Path -Parent $repo)
    return (Join-Path $gamesRoot "mysql-portable\mysql-8.4.8-winx64")
}

function Get-DefaultMysqlPortableDataDir {
    param([string]$ReleaseRoot = "")
    $envData = [Environment]::GetEnvironmentVariable("MYSQL_PORTABLE_DATA", "Process")
    if (-not [string]::IsNullOrWhiteSpace($envData)) { return $envData.Trim() }
    $base = Get-DefaultMysqlPortableBaseDir $ReleaseRoot
    $parent = Split-Path -Parent $base
    return (Join-Path $parent "data")
}

function Get-StarcrystalConfigJsonPath {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $root = if (-not [string]::IsNullOrWhiteSpace($ReleaseRoot)) {
        Get-StarcrystalReleaseRoot $ReleaseRoot
    } elseif (-not [string]::IsNullOrWhiteSpace($ProjectRoot)) {
        if (Test-Path -LiteralPath (Join-Path $ProjectRoot "release\configs\starcrystal.json")) {
            Join-Path $ProjectRoot "release"
        } else {
            Get-StarcrystalReleaseRoot $ProjectRoot
        }
    } else {
        Get-StarcrystalReleaseRoot
    }
    Join-Path $root "configs\starcrystal.json"
}

function Get-StarcrystalAuthMysqlDsn {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $path = Get-StarcrystalConfigJsonPath -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if (-not (Test-Path -LiteralPath $path)) { return $null }
    try {
        $raw = Get-Content -LiteralPath $path -Raw -Encoding UTF8
        $j = $raw | ConvertFrom-Json
        if ($null -eq $j.authMysqlDsn) { return $null }
        $dsn = "$($j.authMysqlDsn)".Trim()
        if ($dsn -eq "") { return $null }
        return $dsn
    } catch {
        return $null
    }
}

function Parse-AuthMysqlDsnString {
    param([Parameter(Mandatory)][string]$Dsn)
    $dsn = "$Dsn".Trim()
    if ($dsn -eq "") { return $null }
    $at = [char]64
    $re = "^([^:]+):([^$at]*)$at" + 'tcp\(([^:]+):(\d+)\)/([^?]+)'
    if (-not ($dsn -match $re)) {
        return $null
    }
    $m = $Matches
    return @{
        User     = $m[1]
        Password = $m[2]
        SqlHost  = $m[3]
        Port     = [int]$m[4]
        Database = $m[5]
    }
}

function Get-StarcrystalAuthMysqlDsnParts {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $dsn = Get-StarcrystalAuthMysqlDsn -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if (-not $dsn) { return $null }
    return Parse-AuthMysqlDsnString $dsn
}

function Get-RequiredStarcrystalAuthMysqlDsnParts {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $path = Get-StarcrystalConfigJsonPath -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if (-not (Test-Path -LiteralPath $path)) {
        throw ('Missing {0}. Add authMysqlDsn: user:pass{1}tcp(HOST:PORT)/DATABASE?...' -f $path, $([char]64))
    }
    $dsn = Get-StarcrystalAuthMysqlDsn -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if ([string]::IsNullOrWhiteSpace($dsn)) {
        throw "authMysqlDsn is missing or empty in $path"
    }
    $parts = Parse-AuthMysqlDsnString $dsn
    if (-not $parts) {
        throw ('Invalid authMysqlDsn in {0}: expected user:pass{1}tcp(HOST:PORT)/DATABASE?params' -f $path, ([char]64))
    }
    return $parts
}

function Get-StarcrystalAuthMysqlClientExe {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $rel = if ($ReleaseRoot) { Get-StarcrystalReleaseRoot $ReleaseRoot } elseif ($ProjectRoot) {
        if (Test-Path (Join-Path $ProjectRoot "release\configs\starcrystal.json")) { Join-Path $ProjectRoot "release" }
        else { Get-StarcrystalReleaseRoot $ProjectRoot }
    } else { Get-StarcrystalReleaseRoot }
    $repo = Split-Path -Parent $rel
    $path = Get-StarcrystalConfigJsonPath -ReleaseRoot $rel
    if (-not (Test-Path -LiteralPath $path)) { return $null }
    try {
        $raw = Get-Content -LiteralPath $path -Raw -Encoding UTF8
        $j = $raw | ConvertFrom-Json
        if ($null -eq $j.authMysqlClientExe) { return $null }
        $exe = "$($j.authMysqlClientExe)".Trim()
        if ($exe -eq "") { return $null }
        $full = if ([System.IO.Path]::IsPathRooted($exe)) { $exe } else { Join-Path $repo $exe }
        if (Test-Path -LiteralPath $full) { return $full }
        throw "authMysqlClientExe in starcrystal.json points to a missing file: $full"
    } catch {
        return $null
    }
}

function Resolve-AuthMysqlClientExe {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = "",
        [string]$MySqlExePathParam = ""
    )
    $rel = Get-StarcrystalReleaseRoot $ReleaseRoot
    $repo = Get-StarcrystalRepoRoot $rel
    if (-not [string]::IsNullOrWhiteSpace($MySqlExePathParam)) {
        $p = $MySqlExePathParam.Trim()
        $full = if ([System.IO.Path]::IsPathRooted($p)) { $p } else { Join-Path $repo $p }
        if (Test-Path -LiteralPath $full) { return $full }
        throw "MySqlExePath not found: $p (resolved: $full)"
    }
    $fromCfg = Get-StarcrystalAuthMysqlClientExe -ReleaseRoot $rel
    if (-not [string]::IsNullOrWhiteSpace($fromCfg)) { return $fromCfg }
    $mysqlCmd = Get-Command mysql -ErrorAction SilentlyContinue
    if ($mysqlCmd) { return $mysqlCmd.Source }
    return $null
}

function Get-LastAuthMysqlDsnFilePath {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $rel = Get-StarcrystalReleaseRoot $ReleaseRoot
    if (-not [string]::IsNullOrWhiteSpace($ProjectRoot) -and (Test-Path (Join-Path $ProjectRoot "release\log"))) {
        return Join-Path $ProjectRoot "release\log\last-auth-mysql-dsn.txt"
    }
    Join-Path $rel "log\last-auth-mysql-dsn.txt"
}

function Get-LastStartedAuthMysqlDsnString {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $p = Get-LastAuthMysqlDsnFilePath -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if (-not (Test-Path -LiteralPath $p)) { return $null }
    try {
        $line = @(Get-Content -LiteralPath $p -Encoding UTF8 -ErrorAction Stop)[0]
        $s = "$line".Trim()
        if ($s -eq "" -or $s.StartsWith("#")) { return $null }
        return $s
    } catch {
        return $null
    }
}

function Save-LastAuthMysqlDsnForScripts {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = "",
        [Parameter(Mandatory)][string]$Dsn
    )
    $dsn = "$Dsn".Trim()
    if ($dsn -eq "") { return }
    $rel = Get-StarcrystalReleaseRoot $ReleaseRoot
    if (-not [string]::IsNullOrWhiteSpace($ProjectRoot) -and -not $ReleaseRoot) {
        if (Test-Path (Join-Path $ProjectRoot "release\log")) {
            $dir = Join-Path $ProjectRoot "release\log"
        } else {
            $dir = Join-Path $rel "log"
        }
    } else {
        $dir = Join-Path $rel "log"
    }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $p = Join-Path $dir "last-auth-mysql-dsn.txt"
    Set-Content -LiteralPath $p -Value $dsn -Encoding UTF8
}

function Get-EffectiveAuthMysqlDsnParts {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $envDsn = [Environment]::GetEnvironmentVariable("AUTH_MYSQL_DSN", "Process")
    $fromEnv = Parse-AuthMysqlDsnString $envDsn
    if ($fromEnv) { return $fromEnv }
    $lastDsn = Get-LastStartedAuthMysqlDsnString -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    $fromLast = Parse-AuthMysqlDsnString $lastDsn
    if ($fromLast) { return $fromLast }
    return Get-StarcrystalAuthMysqlDsnParts -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
}

function Get-StarcrystalApiListenPort {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $path = Get-StarcrystalConfigJsonPath -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    if (-not (Test-Path -LiteralPath $path)) { return 8080 }
    try {
        $j = Get-Content -LiteralPath $path -Raw -Encoding UTF8 | ConvertFrom-Json
        $p = 0
        if ($null -ne $j.apiListenPort) { $p = [int]$j.apiListenPort }
        if ($p -le 0) { return 8080 }
        return $p
    } catch {
        return 8080
    }
}

function Get-StarcrystalApiBaseUrl {
    param(
        [string]$ReleaseRoot = "",
        [string]$ProjectRoot = ""
    )
    $path = Get-StarcrystalConfigJsonPath -ReleaseRoot $ReleaseRoot -ProjectRoot $ProjectRoot
    $defPort = 8080
    if (-not (Test-Path -LiteralPath $path)) {
        return ("http://0.0.0.0:{0}" -f $defPort)
    }
    try {
        $j = Get-Content -LiteralPath $path -Raw -Encoding UTF8 | ConvertFrom-Json
        $h = ([string]$j.apiListenHost).Trim()
        $p = 0
        if ($null -ne $j.apiListenPort) { $p = [int]$j.apiListenPort }
        if ($p -le 0) { $p = $defPort }
        $bindHost = if ($h -eq '') { '0.0.0.0' } else { $h }
        $scheme = if ($j.useHttps -eq $true) { 'https' } else { 'http' }
        return ('{0}://{1}:{2}' -f $scheme, $bindHost, $p)
    } catch {
        return ("http://0.0.0.0:{0}" -f $defPort)
    }
}
