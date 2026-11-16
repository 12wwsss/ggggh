# 交叉编译 Linux 版本（在 Windows 开发机上执行）
#
#   powershell -ExecutionPolicy Bypass -File scripts\build-linux.ps1
#
# 产物在 bin\linux\：coalrag-server、smoke、loadtest、userctl
# 纯 Go（无 CGO），拷到服务器即可运行，不依赖 glibc 版本。

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$out = Join-Path $root 'bin\linux'
New-Item -ItemType Directory -Force -Path $out | Out-Null

$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'

$targets = @(
    @{ name = 'coalrag-server'; pkg = './cmd/server' },
    @{ name = 'smoke'; pkg = './cmd/smoke' },
    @{ name = 'loadtest'; pkg = './cmd/loadtest' },
    @{ name = 'userctl'; pkg = './cmd/userctl' }
)

Push-Location $root
try {
    foreach ($t in $targets) {
        Write-Host "building $($t.name) ..."
        & go build -trimpath -ldflags '-s -w' -o (Join-Path $out $t.name) $t.pkg
        if ($LASTEXITCODE -ne 0) { throw "build $($t.name) failed" }
    }
} finally {
    Pop-Location
    $env:CGO_ENABLED = $null
    $env:GOOS = $null
    $env:GOARCH = $null
}

Get-ChildItem $out | Select-Object Name, @{ n = 'MB'; e = { [math]::Round($_.Length / 1MB, 1) } } | Format-Table -AutoSize
Write-Host "`n拷到服务器：scp bin\linux\* user@host:/opt/coalrag/bin/"
