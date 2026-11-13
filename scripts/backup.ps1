# Backup: database dump + uploaded files + manifest.
#
#   powershell -ExecutionPolicy Bypass -File scripts\backup.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\backup.ps1 -Keep 14 -Out D:\backups
#
# Layout: <Out>\coalrag-YYYYMMDD-HHMMSS\db.dump | storage\ | manifest.json

param(
    [string]$Out = '',
    [int]$Port = 55432,
    [string]$Database = 'coalrag',
    [string]$User = 'postgres',
    [int]$Keep = 7
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $Out) { $Out = Join-Path $root 'backups' }
$pgBin = Join-Path $root '.tools\pgsql\bin'
$storage = Join-Path $root 'storage'

if (-not (Test-Path (Join-Path $pgBin 'pg_dump.exe'))) { throw "PostgreSQL client not found at $pgBin" }

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$dir = Join-Path $Out "coalrag-$stamp"
New-Item -ItemType Directory -Force -Path $dir | Out-Null

$dump = Join-Path $dir 'db.dump'
Write-Host "dumping database $Database ..."
& (Join-Path $pgBin 'pg_dump.exe') -h 127.0.0.1 -p $Port -U $User -Fc -f $dump $Database
if ($LASTEXITCODE -ne 0) { throw 'pg_dump failed' }

$storageCopy = Join-Path $dir 'storage'
$fileCount = 0
$storageBytes = 0
if (Test-Path $storage) {
    Write-Host 'copying storage ...'
    Copy-Item -Path $storage -Destination $storageCopy -Recurse -Force
    $files = Get-ChildItem -Path $storageCopy -Recurse -File
    $fileCount = $files.Count
    if ($fileCount -gt 0) { $storageBytes = ($files | Measure-Object -Property Length -Sum).Sum }
}

$dumpHash = (Get-FileHash -Path $dump -Algorithm SHA256).Hash
$manifest = [ordered]@{
    createdAt   = (Get-Date).ToString('o')
    database    = $Database
    port        = $Port
    dump        = 'db.dump'
    dumpBytes   = (Get-Item $dump).Length
    dumpSha256  = $dumpHash
    storage     = 'storage'
    storageFiles = $fileCount
    storageBytes = $storageBytes
    host        = $env:COMPUTERNAME
}
$manifest | ConvertTo-Json | Set-Content -Path (Join-Path $dir 'manifest.json') -Encoding UTF8

Write-Host ''
Write-Host "backup: $dir"
Write-Host "  database : $([math]::Round((Get-Item $dump).Length / 1MB, 2)) MB"
Write-Host "  storage  : $fileCount files, $([math]::Round($storageBytes / 1MB, 2)) MB"
Write-Host "  sha256   : $dumpHash"

# prune old backups
if ($Keep -gt 0 -and (Test-Path $Out)) {
    $old = Get-ChildItem -Path $Out -Directory -Filter 'coalrag-*' |
        Sort-Object Name -Descending |
        Select-Object -Skip $Keep
    foreach ($d in $old) {
        Write-Host "pruning $($d.Name)"
        Remove-Item -Path $d.FullName -Recurse -Force
    }
}
