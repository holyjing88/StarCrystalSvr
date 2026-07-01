# Dot-source from test-scripts.ps1
$ErrorActionPreference = 'Stop'

function New-ScriptTestResult {
    param(
        [string]$Id,
        [bool]$Passed,
        [string]$Message = '',
        [bool]$Skipped = $false
    )
    [pscustomobject]@{
        Id       = $Id
        Passed   = $Passed
        Message  = $Message
        Skipped  = $Skipped
        DurationMs = 0
    }
}

function Test-TcpPortOpen {
    param([string]$HostName = '127.0.0.1', [int]$Port = 0, [int]$TimeoutMs = 800)
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect($HostName, $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)
        if (-not $ok) { return $false }
        $client.EndConnect($iar) | Out-Null
        return $true
    } catch { return $false }
    finally { $client.Close() }
}

function Test-PowerShellScriptParses {
    param([string]$Path)
    $tokens = $null
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile($Path, [ref]$tokens, [ref]$errors)
    return @($errors.Count -eq 0), ($errors | ForEach-Object { $_.Message }) -join '; '
}

function Write-ScriptTestReport {
    param([object[]]$Results)
    $pass = ($Results | Where-Object { $_.Passed -and -not $_.Skipped }).Count
    $fail = ($Results | Where-Object { -not $_.Passed -and -not $_.Skipped }).Count
    $skip = ($Results | Where-Object { $_.Skipped }).Count
    Write-Host ''
    Write-Host '=== Script acceptance ==='
    foreach ($r in $Results) {
        $mark = if ($r.Skipped) { 'SKIP' } elseif ($r.Passed) { 'PASS' } else { 'FAIL' }
        $dur = if ($r.DurationMs -gt 0) { " ($($r.DurationMs)ms)" } else { '' }
        Write-Host ("[{0}] {1}{2} — {3}" -f $mark, $r.Id, $dur, $r.Message)
    }
    Write-Host ''
    Write-Host ("Total: {0} pass, {1} fail, {2} skip / {3}" -f $pass, $fail, $skip, $Results.Count)
    return ($fail -eq 0)
}

function Invoke-ScriptTestCase {
    param(
        [string]$Id,
        [scriptblock]$Body,
        [switch]$Skip
    )
    if ($Skip) {
        return (New-ScriptTestResult -Id $Id -Passed $true -Skipped $true -Message 'skipped')
    }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        & $Body
        $sw.Stop()
        return (New-ScriptTestResult -Id $Id -Passed $true -Message 'ok' -DurationMs $sw.ElapsedMilliseconds)
    } catch {
        $sw.Stop()
        return (New-ScriptTestResult -Id $Id -Passed $false -Message $_.Exception.Message -DurationMs $sw.ElapsedMilliseconds)
    }
}
