# Build the web frontend and re-embed it into the server binary.
#
#   powershell -ExecutionPolicy Bypass -File scripts\build-web.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\build-web.ps1 -SkipServer

param(
    [switch]$SkipServer,
    [string]$Registry = 'https://registry.npmjs.org'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root 'web'

$pnpm = (Get-Command pnpm -ErrorAction SilentlyContinue).Source
if (-not $pnpm) { $pnpm = $env:COALRAG_PNPM }
if (-not $pnpm) {
    throw 'pnpm not found. Install pnpm or set COALRAG_PNPM to the pnpm.mjs / pnpm.cmd path.'
}

Set-Location $web
if (-not (Test-Path (Join-Path $web 'node_modules'))) {
    Write-Host 'installing frontend dependencies...'
    & $pnpm install "--registry=$Registry"
}

Write-Host 'building frontend...'
& $pnpm build

if ($SkipServer) {
    Write-Host 'frontend built: web\dist'
    exit 0
}

$goBin = Join-Path $root '.tools\go\bin'
if (-not (Test-Path $goBin)) { throw "Go toolchain not found at $goBin" }
$env:GOROOT = Join-Path $root '.tools\go'
$env:GOPATH = Join-Path $root '.tools\gopath'
$env:GOMODCACHE = Join-Path $root '.tools\gopath\pkg\mod'
$env:GOCACHE = Join-Path $root '.tools\gocache'
$env:GOTOOLCHAIN = 'local'
$env:PATH = "$goBin;$env:PATH"

Set-Location $root
Write-Host 'rebuilding server with embedded frontend...'
& go build -o bin\coalrag-server.exe .\cmd\server
Write-Host 'done: bin\coalrag-server.exe'
