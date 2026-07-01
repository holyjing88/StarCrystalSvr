# Clears StarCrystal game-list encrypted cache (PlayerPrefs) on Windows so the client
# refetches /api/v1/games and picks up new entryUrl (e.g. game2.html?v=...).
# Unity PlayerPrefs (Editor / standalone): HKCU\Software\<CompanyName>\<ProductName>
# ProjectSettings: companyName DefaultCompany, productName StarCrystal

$ErrorActionPreference = "Continue"
$keyNames = @(
    "starcrystal.game_list.encrypted.v4"
)
$roots = @(
    "HKCU:\Software\DefaultCompany\StarCrystal",
    "HKCU:\Software\Unity Technologies\StarCrystal"
)

$removed = $false
foreach ($root in $roots) {
    if (-not (Test-Path -LiteralPath $root)) { continue }
    foreach ($name in $keyNames) {
        if (Get-ItemProperty -LiteralPath $root -Name $name -ErrorAction SilentlyContinue) {
            Remove-ItemProperty -LiteralPath $root -Name $name -ErrorAction Stop
            Write-Host "Removed $name from $root"
            $removed = $true
        }
    }
}

if (-not $removed) {
    Write-Host "No game-list PlayerPrefs keys found under common paths (already clean or app never ran on this account)."
    Write-Host "Paths checked: $($roots -join ', ')"
}

Write-Host ""
Write-Host "Android device: Settings -> Apps -> StarCrystal -> Storage -> Clear cache (or Clear data)."
Write-Host "Chrome desktop test: DevTools -> Application -> Clear site data for your API host."
