# End-to-end acceptance run against a running server.
#
#   powershell -ExecutionPolicy Bypass -File scripts\selftest.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\selftest.ps1 -Base http://127.0.0.1:8080

param(
    [string]$Base = 'http://127.0.0.1:8080',
    [string]$Doc = 'testdata\煤矿安全规程节选.docx'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not (Test-Path (Join-Path $root 'bin\smoke.exe'))) {
    $env:GOROOT = Join-Path $root '.tools\go'
    $env:GOPATH = Join-Path $root '.tools\gopath'
    $env:GOMODCACHE = Join-Path $root '.tools\gopath\pkg\mod'
    $env:GOCACHE = Join-Path $root '.tools\gocache'
    $env:GOTOOLCHAIN = 'local'
    $env:PATH = (Join-Path $root '.tools\go\bin') + ';' + $env:PATH
    & go build -o bin\smoke.exe .\cmd\smoke
}

& .\bin\smoke.exe -base $Base -doc $Doc
exit $LASTEXITCODE
