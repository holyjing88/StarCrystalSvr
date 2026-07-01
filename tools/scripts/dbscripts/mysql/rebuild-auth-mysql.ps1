param(
    [string]$SqlHost = "127.0.0.1",
    [int]$Port = 3306,
    [string]$Database = "starcrystal_auth",
    [string]$User = "star_auth",
    [string]$Password = "jgyjgyjgy",
    [string]$MySqlExePath = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "..\dbscripts-config.ps1")
$sqlFile = Join-Path (Get-DbScriptsSqlDir) "starcrystal_auth_mysql.sql"
if (-not (Test-Path $sqlFile)) { throw "SQL file not found: $sqlFile" }

$mysqlPath = $MySqlExePath
if ([string]::IsNullOrWhiteSpace($mysqlPath)) {
    $mysqlCmd = Get-Command mysql -ErrorAction SilentlyContinue
    if ($mysqlCmd) { $mysqlPath = $mysqlCmd.Source }
}
if ([string]::IsNullOrWhiteSpace($mysqlPath)) {
    $portable = Join-Path (Get-DefaultMysqlPortableBaseDir) "bin\mysql.exe"
    if (Test-Path $portable) { $mysqlPath = $portable }
}
if ([string]::IsNullOrWhiteSpace($mysqlPath) -or -not (Test-Path $mysqlPath)) {
    throw "mysql client not found. Install MySQL or pass -MySqlExePath."
}

$sqlSourcePath = $sqlFile.Replace('\', '/')
Write-Host "Rebuilding auth tables in ${Database}@${SqlHost}:${Port} ..."
$rebuildSql = @"
SOURCE $sqlSourcePath;
SHOW TABLES LIKE 'auth_%';
SELECT COUNT(*) AS auth_accounts_rows FROM auth_accounts;
"@
& $mysqlPath @("--protocol=TCP", "--host=$SqlHost", "--port=$Port", "--user=$User", "--password=$Password", $Database) "-e" $rebuildSql
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Done. Auth schema rebuilt."
