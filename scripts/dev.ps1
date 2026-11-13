# Development launcher: starts the local PostgreSQL instance, builds and runs the API server.
#
#   powershell -ExecutionPolicy Bypass -File scripts\dev.ps1
#
# Requires the portable toolchain in .tools\ (see README).

param(
    [int]$PgPort = 55432,
    [string]$Addr = '127.0.0.1:8080',
    [string]$Database = 'coalrag'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$pgBin = Join-Path $root '.tools\pgsql\bin'
$dataDir = Join-Path $root '.data\pg'
$goBin = Join-Path $root '.tools\go\bin'
$logDir = Join-Path $root '.data'

if (-not (Test-Path $pgBin)) { throw "PostgreSQL not found at $pgBin" }
if (-not (Test-Path $goBin)) { throw "Go toolchain not found at $goBin" }

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

if (-not (Test-Path (Join-Path $dataDir 'PG_VERSION'))) {
    Write-Host 'initdb...'
    & (Join-Path $pgBin 'initdb.exe') -D $dataDir -U postgres -E UTF8 --locale=C --auth=trust | Out-Null
}

$running = $false
try {
    & (Join-Path $pgBin 'pg_isready.exe') -h 127.0.0.1 -p $PgPort | Out-Null
    $running = ($LASTEXITCODE -eq 0)
} catch {
    $running = $false
}

if (-not $running) {
    Write-Host "starting PostgreSQL on port $PgPort..."
    & (Join-Path $pgBin 'pg_ctl.exe') -D $dataDir -o "-p $PgPort -c listen_addresses=127.0.0.1" -l (Join-Path $logDir 'pg.log') start | Out-Null
    Start-Sleep -Seconds 3
}

$exists = & (Join-Path $pgBin 'psql.exe') -h 127.0.0.1 -p $PgPort -U postgres -tAc "select 1 from pg_database where datname='$Database'"
if ($exists -ne '1') {
    Write-Host "creating database $Database..."
    & (Join-Path $pgBin 'psql.exe') -h 127.0.0.1 -p $PgPort -U postgres -c "create database $Database" | Out-Null
}

$env:GOROOT = Join-Path $root '.tools\go'
$env:GOPATH = Join-Path $root '.tools\gopath'
$env:GOMODCACHE = Join-Path $root '.tools\gopath\pkg\mod'
$env:GOCACHE = Join-Path $root '.tools\gocache'
$env:GOTOOLCHAIN = 'local'
$env:PATH = "$goBin;$env:PATH"
$env:COALRAG_HTTP_ADDR = $Addr

Set-Location $root
Write-Host 'building...'
& go build -o bin\coalrag-server.exe .\cmd\server
if ($LASTEXITCODE -ne 0) { throw 'build failed' }

Write-Host "starting server on $Addr"
& .\bin\coalrag-server.exe
