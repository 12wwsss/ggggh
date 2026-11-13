# Stop the local PostgreSQL instance started by scripts\dev.ps1.

param([string]$DataDir = '')

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $DataDir) { $DataDir = Join-Path $root '.data\pg' }

& (Join-Path $root '.tools\pgsql\bin\pg_ctl.exe') -D $DataDir stop -m fast
