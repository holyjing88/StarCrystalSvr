# Dot-source: . (Join-Path $PSScriptRoot "starcrystal-server-root.ps1")
# 服务器仓库根目录（SVN 工作副本）。可用环境变量 STARCrystalSVR_ROOT 覆盖。

function Get-StarcrystalServerRootDefault {
    'Y:\holyjing\starcrystalsvr'
}

function Test-StarcrystalServerRootIsCanonical {
    param([Parameter(Mandatory)][string]$Path)
    $p = $Path.TrimEnd('\')
    return ($p -ieq 'Y:\holyjing\starcrystalsvr')
}

function Get-StarcrystalServerRoot {
    param([string]$Override = "")
    if (-not [string]::IsNullOrWhiteSpace($Override)) {
        $resolved = (Resolve-Path -LiteralPath $Override.Trim()).Path
        if (-not (Test-StarcrystalServerRootIsCanonical $resolved)) {
            throw "Server root must be Y:\holyjing\starcrystalsvr (got: $resolved). Do not use d:\0_games\...\server."
        }
        return $resolved
    }
    $envRoot = [Environment]::GetEnvironmentVariable('STARCrystalSVR_ROOT', 'Process')
    if ([string]::IsNullOrWhiteSpace($envRoot)) {
        $envRoot = [Environment]::GetEnvironmentVariable('STARCrystalSVR_ROOT', 'User')
    }
    if (-not [string]::IsNullOrWhiteSpace($envRoot)) {
        $resolved = (Resolve-Path -LiteralPath $envRoot.Trim()).Path
        if (-not (Test-StarcrystalServerRootIsCanonical $resolved)) {
            throw "STARCrystalSVR_ROOT must be Y:\holyjing\starcrystalsvr (got: $resolved)"
        }
        return $resolved
    }
    $def = Get-StarcrystalServerRootDefault
    if (-not (Test-Path -LiteralPath $def)) {
        throw @"
StarCrystal server root not found: $def
  - Map/checkout SVN to Y:\holyjing\starcrystalsvr, or
  - Set STARCrystalSVR_ROOT to your starcrystalsvr path, or
  - Pass -RepoRoot to the script.
"@
    }
    return (Resolve-Path -LiteralPath $def).Path
}
