# Lightweight watchdog: polls /healthz and /metrics, writes a log line and
# optionally posts to a webhook (WeCom / DingTalk / any JSON endpoint).
#
#   powershell -ExecutionPolicy Bypass -File scripts\watch.ps1                 # loop every 60s
#   powershell -ExecutionPolicy Bypass -File scripts\watch.ps1 -Once           # single check (exit 1 on alert)
#   powershell -ExecutionPolicy Bypass -File scripts\watch.ps1 -Webhook https://example/hook
#
# The plan allows replacing a monitoring platform with a probe script; this is it.

param(
    [string]$Base = 'http://127.0.0.1:8080',
    [int]$IntervalSeconds = 60,
    [string]$Webhook = '',
    [string]$LogFile = '',
    [int]$MaxFailedTasks = 5,
    [int]$MaxOverdueIssues = 5,
    [switch]$Once
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $LogFile) { $LogFile = Join-Path $root '.data\watch.log' }

function Write-Log([string]$level, [string]$message) {
    $line = '{0} [{1}] {2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $level, $message
    Write-Host $line
    $dir = Split-Path -Parent $LogFile
    if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    Add-Content -Path $LogFile -Value $line -Encoding UTF8
}

function Send-Webhook([string]$text) {
    if (-not $Webhook) { return }
    try {
        $body = @{ msgtype = 'text'; text = @{ content = $text } } | ConvertTo-Json -Depth 5
        Invoke-RestMethod -Uri $Webhook -Method Post -ContentType 'application/json; charset=utf-8' -Body $body -TimeoutSec 10 | Out-Null
    } catch {
        Write-Log 'WARN' "webhook failed: $($_.Exception.Message)"
    }
}

function Check-Once {
    $alerts = @()

    try {
        $health = Invoke-RestMethod -Uri "$Base/healthz" -TimeoutSec 8
        if ($health.status -ne 'ok') { $alerts += "health=$($health.status)" }
        foreach ($name in @('db', 'llm', 'tunnel')) {
            $c = $health.checks.$name
            if ($null -eq $c) { continue }
            if ($c.status -eq 'down') { $alerts += "$name=down" }
        }
    } catch {
        $alerts += "healthz unreachable: $($_.Exception.Message)"
    }

    try {
        $metrics = (Invoke-WebRequest -Uri "$Base/metrics" -TimeoutSec 8).Content
        $failed = 0
        $overdue = 0
        if ($metrics -match '(?m)^coalrag_tasks_failed_recent (\d+)') { $failed = [int]$Matches[1] }
        if ($metrics -match '(?m)^coalrag_issues_overdue (\d+)') { $overdue = [int]$Matches[1] }
        if ($failed -gt $MaxFailedTasks) { $alerts += "tasks_failed_recent=$failed" }
        if ($overdue -gt $MaxOverdueIssues) { $alerts += "issues_overdue=$overdue" }
    } catch {
        $alerts += "metrics unreachable: $($_.Exception.Message)"
    }

    if ($alerts.Count -eq 0) {
        Write-Log 'OK' 'all checks passed'
        return 0
    }
    $text = "CoalRAG alert: " + ($alerts -join ', ')
    Write-Log 'ALERT' $text
    Send-Webhook $text
    return 1
}

if ($Once) {
    exit (Check-Once)
}

Write-Log 'INFO' "watchdog started (interval ${IntervalSeconds}s, base $Base)"
while ($true) {
    Check-Once | Out-Null
    Start-Sleep -Seconds $IntervalSeconds
}
