# Dot-source: . .\Import-AuthEnvFromFile.ps1
# .env 文件请使用 UTF-8（可无 BOM）。
function Import-AuthEnvFromFile {
    # KEY=value 写入进程环境；# 行注释、空行忽略；值可包在双引号中
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    Get-Content -LiteralPath $Path -Encoding UTF8 -ErrorAction Stop | ForEach-Object {
        $line = $_.Trim()
        if ($line.Length -eq 0) { return }
        if ($line.StartsWith("#")) { return }
        $eq = $line.IndexOf([char]61) # '=' 
        if ($eq -lt 1) { return }
        $k = $line.Substring(0, $eq).Trim()
        if ($k.Length -eq 0) { return }
        $v = $line.Substring($eq + 1)
        if ($v.Length -ge 2 -and $v[0] -eq [char]34 -and $v[$v.Length - 1] -eq [char]34) {
            $v = $v.Substring(1, $v.Length - 2)
        }
        $v = $v.Trim()
        [System.Environment]::SetEnvironmentVariable($k, $v, "Process")
    }
}
