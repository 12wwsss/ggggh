# One-shot quality gate: format check, vet, unit tests, build, optional e2e smoke.
#
#   powershell -ExecutionPolicy Bypass -File scripts\check.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\check.ps1 -Smoke
#
# -Smoke needs a running server on -Base (default http://127.0.0.1:8080).

param(
    [switch]$Smoke,
    [string]$Base = 'http://127.0.0.1:8080'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$goBin = Join-Path $root '.tools\go\bin'
if (-not (Test-Path $goBin)) { throw "Go toolchain not found at $goBin" }

$env:GOROOT = Join-Path $root '.tools\go'
$env:GOPATH = Join-Path $root '.tools\gopath'
$env:GOMODCACHE = Join-Path $root '.tools\gopath\pkg\mod'
$env:GOCACHE = Join-Path $root '.tools\gocache'
$env:GOTOOLCHAIN = 'local'
$env:PATH = "$goBin;$env:PATH"

Set-Location $root
$failed = 0

Write-Host '[1/4] gofmt'
$unformatted = & gofmt -l cmd internal web
if ($unformatted) {
    Write-Host "  unformatted files:" -ForegroundColor Red
    $unformatted | ForEach-Object { Write-Host "    $_" }
    $failed++
} else {
    Write-Host '  ok'
}

Write-Host '[2/4] go vet'
& go vet ./...
if ($LASTEXITCODE -ne 0) { $failed++ } else { Write-Host '  ok' }

Write-Host '[3/4] go test'
& go test ./internal/...
if ($LASTEXITCODE -ne 0) { $failed++ } else { Write-Host '  ok' }

Write-Host '[4/4] go build'
& go build -o bin\coalrag-server.exe .\cmd\server
if ($LASTEXITCODE -ne 0) { $failed++ } else { Write-Host '  ok' }

if ($Smoke) {
    Write-Host '[smoke] end-to-end'
    & (Join-Path $root 'bin\smoke.exe') -base $Base -doc 'testdata\煤矿安全规程节选.docx'
    if ($LASTEXITCODE -ne 0) { $failed++ }
}

if ($failed -gt 0) {
    Write-Host "`nCHECK FAILED ($failed step(s))" -ForegroundColor Red
    exit 1
}
Write-Host "`nCHECK PASSED"
