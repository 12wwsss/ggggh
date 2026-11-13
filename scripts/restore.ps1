# Restore a backup created by scripts\backup.ps1.
#
#   powershell -ExecutionPolicy Bypass -File scripts\restore.ps1 -From backups\coalrag-20261112-140000
#   powershell -ExecutionPolicy Bypass -File scripts\restore.ps1 -From ... -Database coalrag_test -Force
#
# The target database is dropped and recreated, so pass -Force in unattended runs.
# The API service must be stopped first: it holds connections to the database.

param(
    [Parameter(Mandatory = $true)][string]$From,
    [int]$Port = 55432,
    [string]$Database = 'coalrag',
    [string]$User = 'postgres',
    [switch]$Force,
    [switch]$RestoreStorage
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$pgBin = Join-Path $root '.tools\pgsql\bin'

if (-not (Test-Path (Join-Path $pgBin 'pg_restore.exe'))) { throw "PostgreSQL client not found at $pgBin" }
$dir = if (Test-Path $From) { (Resolve-Path $From).Path } else { throw "backup not found: $From" }
$dump = Join-Path $dir 'db.dump'
if (-not (Test-Path $dump)) { throw "db.dump not found in $dir" }
$manifest = Join-Path $dir 'manifest.json'
if (Test-Path $manifest) {
    $meta = Get-Content $manifest -Raw | ConvertFrom-Json
    Write-Host "backup from $($meta.createdAt) host=$($meta.host) db=$($meta.database)"
    $hash = (Get-FileHash -Path $dump -Algorithm SHA256).Hash
    if ($meta.dumpSha256 -and $hash -ne $meta.dumpSha256) { throw 'dump sha256 mismatch, backup may be corrupt' }
}

if (-not $Force) {
    $answer = Read-Host "This will DROP and recreate database '$Database'. Type YES to continue"
    if ($answer -ne 'YES') { throw 'aborted' }
}

$psql = Join-Path $pgBin 'psql.exe'
Write-Host 'terminating existing connections ...'
& $psql -h 127.0.0.1 -p $Port -U $User -d postgres -q -c "select pg_terminate_backend(pid) from pg_stat_activity where datname='$Database' and pid <> pg_backend_pid();" | Out-Null
& $psql -h 127.0.0.1 -p $Port -U $User -d postgres -q -c "drop database if exists `"$Database`";"
if ($LASTEXITCODE -ne 0) { throw 'drop database failed' }
& $psql -h 127.0.0.1 -p $Port -U $User -d postgres -q -c "create database `"$Database`";"
if ($LASTEXITCODE -ne 0) { throw 'create database failed' }

Write-Host 'restoring database ...'
& (Join-Path $pgBin 'pg_restore.exe') -h 127.0.0.1 -p $Port -U $User -d $Database --no-owner --no-privileges $dump
if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed' }

if ($RestoreStorage) {
    $src = Join-Path $dir 'storage'
    $dst = Join-Path $root 'storage'
    if (Test-Path $src) {
        Write-Host 'restoring storage ...'
        if (Test-Path $dst) { Remove-Item -Path $dst -Recurse -Force }
        Copy-Item -Path $src -Destination $dst -Recurse -Force
    } else {
        Write-Host 'no storage folder in backup, skipped'
    }
}

& $psql -h 127.0.0.1 -p $Port -U $User -d $Database -tAc "select 'users='||count(*) from org_user; select 'docs='||count(*) from doc; select 'chunks='||count(*) from doc_chunk;"
Write-Host ''
Write-Host "restore done: database=$Database storage=$([bool]$RestoreStorage)"
Write-Host 'start the service again with scripts\dev.ps1 (or your systemd unit).'
