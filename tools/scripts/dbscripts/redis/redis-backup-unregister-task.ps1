#Requires -Version 5.1
# Save as UTF-8 with BOM for Windows PowerShell 5.x.
param(
    [string] $TaskName = "StarCrystal-RedisBackup"
)
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Write-Host "[unregister] 已移除计划任务: $TaskName（若存在）。"
