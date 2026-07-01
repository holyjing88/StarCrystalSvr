# dbscripts/redis — dot-sourced by redis-start|stop|backup.ps1.
# UTF-8: save *.ps1 as UTF-8 with BOM for Windows PowerShell 5.x.

function Get-ScDbscriptsRedisBinDir {
    param([string]$ScriptRoot)
    if ([string]::IsNullOrWhiteSpace($ScriptRoot)) { return $null }
    foreach ($sub in @('bin', 'linux')) {
        $candidate = Join-Path $ScriptRoot $sub
        if (Test-Path -LiteralPath $candidate) { return (Resolve-Path -LiteralPath $candidate).Path }
    }
    return $null
}

function Get-ScRedisConfFileIfPresent {
    param([string]$ScriptRoot)
    foreach ($src in @("Process", "User")) {
        $e = [Environment]::GetEnvironmentVariable("REDIS_CONF", $src)
        if ($e -and (Test-Path -LiteralPath $e)) { return (Resolve-Path -LiteralPath $e).Path }
    }
    $a = Join-Path $ScriptRoot "redis.conf"
    if (Test-Path -LiteralPath $a) { return (Resolve-Path -LiteralPath $a).Path }
    return $null
}

function Get-ScRedisCli {
    param([string]$Explicit = "", [string]$ScriptRoot = "")
    if ($Explicit -and (Test-Path -LiteralPath $Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    foreach ($src in @("Process", "User")) {
        $e = [Environment]::GetEnvironmentVariable("REDIS_CLI_EXE", $src)
        if ($e -and (Test-Path -LiteralPath $e)) { return (Resolve-Path -LiteralPath $e).Path }
    }
    $c = Get-Command redis-cli.exe -ErrorAction SilentlyContinue
    if ($c) { return $c.Source }
    $c2 = Get-Command redis-cli -ErrorAction SilentlyContinue
    if ($c2) { return $c2.Source }
    if ($ScriptRoot) {
        $pd = Get-ScDbscriptsRedisBinDir -ScriptRoot $ScriptRoot
        if ($pd) {
            foreach ($name in @("redis-cli.exe", "redis-cli")) {
                $fp = Join-Path $pd $name
                if (Test-Path -LiteralPath $fp) { return (Resolve-Path -LiteralPath $fp).Path }
            }
        }
    }
    return $null
}

function Get-ScRedisServer {
    param([string]$Explicit = "", [string]$ScriptRoot = "")
    if ($Explicit -and (Test-Path -LiteralPath $Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    foreach ($src in @("Process", "User")) {
        $e = [Environment]::GetEnvironmentVariable("REDIS_SERVER_EXE", $src)
        if ($e -and (Test-Path -LiteralPath $e)) { return (Resolve-Path -LiteralPath $e).Path }
    }
    $cmd = Get-Command redis-server.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $cmd2 = Get-Command redis-server -ErrorAction SilentlyContinue
    if ($cmd2) { return $cmd2.Source }
    if ($ScriptRoot) {
        $pd = Get-ScDbscriptsRedisBinDir -ScriptRoot $ScriptRoot
        if ($pd) {
            foreach ($name in @("redis-server.exe", "redis-server")) {
                $fp = Join-Path $pd $name
                if (Test-Path -LiteralPath $fp) { return (Resolve-Path -LiteralPath $fp).Path }
            }
        }
    }
    return $null
}

function Get-PortFromRedisConf {
    param([string]$ConfPath)
    if (-not (Test-Path -LiteralPath $ConfPath)) { return 0 }
    foreach ($line in Get-Content -LiteralPath $ConfPath -Encoding UTF8) {
        $t = $line.Trim()
        if ($t.StartsWith("#") -or $t.Length -eq 0) { continue }
        if ($t -match '^port\s+(\d+)') { return [int]$Matches[1] }
    }
    return 0
}

# Port resolution: explicit arg > PORT > REDIS_PORT > redis.conf > 6379
function Resolve-ScRedisPort {
    param(
        [int]$ExplicitPort = 0,
        [string]$ConfPath = "",
        [string]$ScriptRoot = ""
    )
    if ($ExplicitPort -gt 0) { return $ExplicitPort }
    foreach ($src in @("Process", "User")) {
        foreach ($name in @("PORT", "REDIS_PORT")) {
            $e = [Environment]::GetEnvironmentVariable($name, $src)
            if (-not [string]::IsNullOrWhiteSpace($e)) {
                try {
                    $v = [int]$e
                    if ($v -gt 0) { return $v }
                } catch { }
            }
        }
    }
    if ($ConfPath -and (Test-Path -LiteralPath $ConfPath)) {
        $v2 = Get-PortFromRedisConf -ConfPath $ConfPath
        if ($v2 -gt 0) { return $v2 }
    }
    if ($ScriptRoot) {
        $c2 = Join-Path $ScriptRoot "redis.conf"
        if (Test-Path -LiteralPath $c2) {
            $v3 = Get-PortFromRedisConf -ConfPath $c2
            if ($v3 -gt 0) { return $v3 }
        }
    }
    return 6379
}

function Test-ScRedisPong {
    param([int]$Port, [string]$CliExplicit = "", [string]$ScriptRoot = "")
    try {
        $cli = Get-ScRedisCli -Explicit $CliExplicit -ScriptRoot $ScriptRoot
        if (-not $cli) { return $false }
        $out = & $cli -p $Port ping 2>$null
        return ($null -ne $out -and "$out" -match "PONG")
    } catch { return $false }
}
