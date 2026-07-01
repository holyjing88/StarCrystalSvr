#Requires -Version 5.1
# Save as UTF-8 with BOM for Windows PowerShell 5.x.
<#
.SYNOPSIS
  Register a Windows Scheduled Task to run redis-backup.ps1 daily.

.PARAMETER DailyAt
  Daily time, e.g. "03:15". Default 03:15.

.PARAMETER TaskName
  Task name; default StarCrystal-RedisBackup.

.EXAMPLE
  Run elevated PowerShell:
  .\redis-backup-register-task.ps1
  .\redis-backup-register-task.ps1 -DailyAt "02:00" -Port 0
  Port 0 lets redis-backup.ps1 infer port from redis.conf / REDIS_PORT.
#>
[CmdletBinding()]
param(
    [string] $DailyAt = "03:15",
    [string] $TaskName = "StarCrystal-RedisBackup",
    [int] $Port = 0
)

$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$backupScript = Join-Path $here "redis-backup.ps1"
if (-not (Test-Path -LiteralPath $backupScript)) {
    Write-Error "找不到 redis-backup.ps1: $backupScript"
}

$arg = "-NoProfile -ExecutionPolicy Bypass -File `"$backupScript`" -Port $Port"
$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument $arg -WorkingDirectory $here

$parts = $DailyAt -split ":"
$h = [int]$parts[0]; $m = [int]$parts[1]
$trigger = New-ScheduledTaskTrigger -Daily -At ([DateTime]::Today.AddHours($h).AddMinutes($m))

$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Force | Out-Null
Write-Host "[register] 已注册计划任务: $TaskName 每天 $DailyAt 执行备份。"
Write-Host "[register] 卸载请运行: Unregister-ScheduledTask -TaskName '$TaskName' -Confirm:`$false"
