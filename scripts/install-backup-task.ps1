# Register (or remove) a daily Windows Scheduled Task that runs the backup.
#
#   powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -Time 03:15 -Keep 14
#   powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -Remove
#   powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -DryRun

param(
    [string]$Time = '02:30',
    [int]$Keep = 7,
    [string]$Out = '',
    [string]$TaskName = 'CoalRAG-Backup',
    [switch]$Remove,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$script = Join-Path $root 'scripts\backup.ps1'
if (-not (Test-Path $script)) { throw "backup.ps1 not found at $script" }

if ($Remove) {
    $cmd = "schtasks /Delete /TN `"$TaskName`" /F"
    if ($DryRun) { Write-Host $cmd; exit 0 }
    & schtasks /Delete /TN "$TaskName" /F
    exit $LASTEXITCODE
}

if (-not $Out) { $Out = Join-Path $root 'backups' }
$action = "powershell -NoProfile -ExecutionPolicy Bypass -File '$script' -Keep $Keep -Out '$Out'"
$create = "schtasks /Create /TN `"$TaskName`" /SC DAILY /ST $Time /TR `"$action`" /F"

if ($DryRun) {
    Write-Host 'would run:'
    Write-Host "  $create"
    exit 0
}

& schtasks /Create /TN "$TaskName" /SC DAILY /ST $Time /TR "$action" /F
if ($LASTEXITCODE -ne 0) { throw 'schtasks failed' }
Write-Host "scheduled task '$TaskName' created: daily at $Time -> $script (keep=$Keep, out=$Out)"
Write-Host 'verify with: schtasks /Query /TN "' + $TaskName + '" /V /FO LIST'
