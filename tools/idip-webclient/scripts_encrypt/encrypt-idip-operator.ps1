# idip-webclient 自包含：生成 idip.operatorCipherKey + idip.operators[].passwordEnc。
# 默认写入 ../../../release/configs/starcrystal.json（仅 idip 段），并在 scripts_encrypt/encrypt 落盘明文/密文。
#
# 用法:
#   .\encrypt-idip-operator.ps1
#   .\encrypt-idip-operator.ps1 -Username ops_admin -Password 'secret' -CipherKeyBase64 '<b64>'
#   .\encrypt-idip-operator.ps1 -ConfigPath C:\path\to\starcrystal.json
param(
    [string]$Username,
    [string]$Password,
    [string]$CipherKeyBase64,
    [string]$ConfigPath = ''
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$DefaultConfigPath = Join-Path (Resolve-Path (Join-Path $ScriptDir '..\..\..')).Path 'release\configs\starcrystal.json'
$LocalPlainFile = Join-Path $ScriptDir 'encrypt\idip-operator.plain.txt'
$LocalEncryptedFile = Join-Path $ScriptDir 'encrypt\idip-operator.encrypted.json'

function New-IdipCipherKeyBase64 {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }
    return [Convert]::ToBase64String($bytes)
}

function New-IdipComplexPassword {
    param([int]$Length = 20)
    $charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%&*-_=+'
    $chars = $charset.ToCharArray()
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $bytes = New-Object byte[] $Length
    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }
    $sb = New-Object System.Text.StringBuilder
    foreach ($b in $bytes) {
        [void]$sb.Append($chars[$b % $chars.Length])
    }
    return $sb.ToString()
}

function Invoke-IdipPasswordEncrypt {
    param(
        [string]$PlainPassword,
        [string]$CipherKeyBase64,
        [string]$OperatorUsername
    )
    $key = [Convert]::FromBase64String($CipherKeyBase64)
    if ($key.Length -ne 32) {
        throw 'cipher key must be base64-encoded 32 bytes'
    }

    $nonce = New-Object byte[] 12
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $rng.GetBytes($nonce)
    }
    finally {
        $rng.Dispose()
    }

    $plainBytes = [System.Text.Encoding]::UTF8.GetBytes($PlainPassword)
    $cipherBytes = New-Object byte[] $plainBytes.Length
    $tag = New-Object byte[] 16

    $aesGcmType = [System.Security.Cryptography.AesGcm]
    if (-not $aesGcmType) {
        throw 'AES-GCM requires .NET Core 3+ / PowerShell 7+ (or install OpenSSL and use encrypt-idip-operator.sh on Linux)'
    }

    $aesGcm = [System.Security.Cryptography.AesGcm]::new($key)
    try {
        $aesGcm.Encrypt($nonce, $plainBytes, $cipherBytes, $tag)
    }
    finally {
        $aesGcm.Dispose()
    }

    $combined = New-Object byte[] ($nonce.Length + $cipherBytes.Length + $tag.Length)
    [Buffer]::BlockCopy($nonce, 0, $combined, 0, $nonce.Length)
    [Buffer]::BlockCopy($cipherBytes, 0, $combined, $nonce.Length, $cipherBytes.Length)
    [Buffer]::BlockCopy($tag, 0, $combined, ($nonce.Length + $cipherBytes.Length), $tag.Length)

    $enc = 'v1:' + [Convert]::ToBase64String($combined)
    return [ordered]@{
        username    = $OperatorUsername
        passwordEnc = $enc
    }
}

if ([string]::IsNullOrWhiteSpace($Username)) {
    if (-not [string]::IsNullOrWhiteSpace($env:IDIP_OPERATOR_USERNAME)) {
        $Username = $env:IDIP_OPERATOR_USERNAME
    }
    else {
        $Username = 'ops_admin'
    }
}

$_pwdGenerated = $false
if ([string]::IsNullOrWhiteSpace($Password)) {
    if (-not [string]::IsNullOrWhiteSpace($env:IDIP_OPERATOR_PASSWORD)) {
        $Password = $env:IDIP_OPERATOR_PASSWORD
    }
    else {
        $Password = New-IdipComplexPassword
        $_pwdGenerated = $true
    }
}

$_keyGenerated = $false
if ([string]::IsNullOrWhiteSpace($CipherKeyBase64)) {
    if (-not [string]::IsNullOrWhiteSpace($env:IDIP_OPERATOR_CIPHER_KEY)) {
        $CipherKeyBase64 = $env:IDIP_OPERATOR_CIPHER_KEY
    }
    else {
        $CipherKeyBase64 = New-IdipCipherKeyBase64
        $_keyGenerated = $true
    }
}

if ($_keyGenerated -or $_pwdGenerated) {
    Write-Host '=== IDIP 运营凭据（请妥善保存） ===' -ForegroundColor Yellow
    if ($_keyGenerated) {
        Write-Host "IDIP_OPERATOR_CIPHER_KEY=$CipherKeyBase64"
    }
    if ($_pwdGenerated) {
        Write-Host "IDIP_OPERATOR_PASSWORD=$Password"
    }
    Write-Host "IDIP_OPERATOR_USERNAME=$Username"
    Write-Host '=================================='
}

$op = Invoke-IdipPasswordEncrypt -PlainPassword $Password -CipherKeyBase64 $CipherKeyBase64 -OperatorUsername $Username
$out = ($op | ConvertTo-Json -Depth 3)
Write-Host $out

if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    if (-not [string]::IsNullOrWhiteSpace($env:IDIP_CONFIG_PATH)) {
        $ConfigPath = $env:IDIP_CONFIG_PATH
    }
    else {
        $ConfigPath = $DefaultConfigPath
    }
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "Config not found: $ConfigPath"
}

$encryptDir = Join-Path $ScriptDir 'encrypt'
if (-not (Test-Path -LiteralPath $encryptDir)) {
    New-Item -ItemType Directory -Path $encryptDir | Out-Null
}

@(
    '# Generated by encrypt-idip-operator.ps1 — keep secret'
    "IDIP_OPERATOR_USERNAME=$Username"
    "IDIP_OPERATOR_PASSWORD=$Password"
    "IDIP_OPERATOR_CIPHER_KEY=$CipherKeyBase64"
) | Set-Content -LiteralPath $LocalPlainFile -Encoding UTF8
$out | Set-Content -LiteralPath $LocalEncryptedFile -Encoding UTF8
Write-Host "Wrote $LocalPlainFile"
Write-Host "Wrote $LocalEncryptedFile"

$cfg = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
if (-not $cfg.idip) {
    $cfg | Add-Member -NotePropertyName idip -NotePropertyValue ([ordered]@{})
}
$cfg.idip | Add-Member -Force -NotePropertyName operatorCipherKey -NotePropertyValue $CipherKeyBase64
$list = @([ordered]@{ username = $op.username; passwordEnc = $op.passwordEnc })
$cfg.idip | Add-Member -Force -NotePropertyName operators -NotePropertyValue $list
$cfg | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $ConfigPath -Encoding UTF8
Write-Host "Updated $ConfigPath (idip.operatorCipherKey + idip.operators only)"
