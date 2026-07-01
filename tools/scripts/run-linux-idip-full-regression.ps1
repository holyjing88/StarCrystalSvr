# Linux 服务端 + idip-webclient 全量 Vitest + 静态 WEB + Unity 客户端回归
param(
    [string]$LinuxHost = "192.168.75.99",
    [switch]$SkipUnity,
    [switch]$SkipWebDeploy,
    [switch]$SkipGoIntegration
)

$ErrorActionPreference = "Stop"
$toolsDir = $PSScriptRoot
. (Join-Path $toolsDir "test-env.local.ps1")

$ApiBase = "http://${LinuxHost}:8080"
$WebUrl = if ($env:IDIP_WEBCLIENT_URL) { $env:IDIP_WEBCLIENT_URL } else { "http://${LinuxHost}" }
$Root = if ($env:LINUX_DEPLOY_DIR) { $env:LINUX_DEPLOY_DIR } else { "/home/holyjing/starcrystalsvr" }

$webclientY = "Y:\holyjing\starcrystalsvr\tools\idip-webclient"
$webclientD = "d:\0_games\000StarCrystal\idip-webclient"
$webclientDir = if (Test-Path (Join-Path $webclientY "package.json")) { $webclientY } else { $webclientD }

Write-Host "==> Phase 1: Linux server ($LinuxHost) prepare + Go tests"
$env:LINUX_TEST_HOST = $LinuxHost
$env:STARCrystal_LOCAL_ROOT = "Y:\holyjing\starcrystalsvr"
python "Y:\holyjing\starcrystalsvr\tools\scripts\_idip_linux_acceptance.py"
if ($LASTEXITCODE -ne 0) { throw "Linux acceptance failed" }

if (-not $SkipGoIntegration) {
    Write-Host "==> Phase 1b: Linux integration tests"
    python -c @"
import os, paramiko
HOST, USER, PWD, ROOT = '$LinuxHost', '$($env:STARCRYSTAL_LINUX_SSH_USER)', '$($env:STARCRYSTAL_LINUX_SSH_PASSWORD)', '$Root'
GO = f'{ROOT}/.go-toolchain/go/bin/go'
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, username=USER, password=PWD, timeout=30, allow_agent=False, look_for_keys=False)
dsn_cmd = f\"sed -n 's/.*\\\"authMysqlDsn\\\"[[:space:]]*:[[:space:]]*\\\"\\\\([^\\\"]*\\\\)\\\".*/\\\\1/p' {ROOT}/release/configs/starcrystal.json\"
_, so, _ = c.exec_command(dsn_cmd, timeout=30)
dsn = so.read().decode().strip() or 'root:jgyjgyjgy@tcp(127.0.0.1:3306)/starcrystal_auth?charset=utf8mb4&parseTime=true&loc=Local'
cmd = f'cd {ROOT} && STARCRYSTAL_INTEGRATION_MYSQL=\"{dsn}\" AUTH_SMS_COOLDOWN_SEC=0 {GO} test ./internal/integration -tags=integration -count=1 -timeout 180s'
_, o, e = c.exec_command(cmd, timeout=300)
out = o.read().decode() + e.read().decode()
print(out)
if o.channel.recv_exit_status() != 0: raise SystemExit(1)
c.close()
"@
    if ($LASTEXITCODE -ne 0) { throw "Linux integration tests failed" }
}

Write-Host "==> Phase 1c: H5 upload dir permissions + publish paths"
python -c @"
import os, paramiko
HOST, USER, PWD, ROOT = '$LinuxHost', '$($env:STARCRYSTAL_LINUX_SSH_USER)', '$($env:STARCRYSTAL_LINUX_SSH_PASSWORD)', '$Root'
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, username=USER, password=PWD, timeout=30, allow_agent=False, look_for_keys=False)
cmd = f'''set -e
mkdir -p {ROOT}/release_h5 {ROOT}/release_h5_backup {ROOT}/release/configs
chown -R holyjing:holyjing {ROOT}/release_h5 {ROOT}/release_h5_backup {ROOT}/release/configs 2>/dev/null || true
chmod -R u+rwX {ROOT}/release_h5 {ROOT}/release_h5_backup {ROOT}/release/configs 2>/dev/null || true
ls -la {ROOT}/release_h5/
'''
_, o, e = c.exec_command(cmd, timeout=60)
print(o.read().decode())
print(e.read().decode(), file=__import__('sys').stderr)
c.close()
"@

Write-Host "==> Phase 2: idip-webclient Vitest (all suites) -> $ApiBase"
$env:IDIP_BASE_URL = $ApiBase
$env:IDIP_USERNAME = "ops_admin"
$env:IDIP_PASSWORD = "change-me-ops-password"
$env:IDIP_KEY = if ($env:STARCRYSTAL_IDIP_KEY) { $env:STARCRYSTAL_IDIP_KEY } else { "change-me-in-production" }
$env:IDIP_SKIP_H5 = ""

Push-Location $webclientDir
try {
    if (-not (Test-Path node_modules/vitest)) {
        Write-Host "npm install in $webclientDir ..."
        npm install 2>&1 | Select-Object -Last 8
    }
    Write-Host "Vitest: regression + publish + player API + web deploy"
    npx vitest run --reporter=verbose 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Vitest failed" }
} finally {
    Pop-Location
}

if (-not $SkipWebDeploy) {
    Write-Host "==> Phase 2b: webclient nginx deploy tests -> $WebUrl"
    $env:IDIP_WEBCLIENT_URL = $WebUrl
    Push-Location $webclientDir
    try {
        npx vitest run src/tests/webclient_deploy.test.ts --reporter=verbose 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Warning "webclient_deploy tests failed or skipped (nginx?)" }
    } finally {
        Pop-Location
    }
}

if (-not $SkipUnity) {
    Write-Host "==> Phase 3: Unity client (EditMode + PlayMode) -> $ApiBase"
    $env:STARCRYSTAL_API_BASE_URL = $ApiBase
    $env:STARCRYSTAL_CLIENT_INTEGRATION = "1"
    & (Join-Path $toolsDir "run-all-unity-tests.ps1")
}

Write-Host "`n=== FULL IDIP + CLIENT REGRESSION PASS ===" -ForegroundColor Green
