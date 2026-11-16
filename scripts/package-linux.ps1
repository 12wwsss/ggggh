# 打一个可直接拷到服务器解压的部署包（离线摆渡用）
#
#   powershell -ExecutionPolicy Bypass -File scripts\package-linux.ps1
#
# 产物：dist\coalrag-linux-<日期>.tar.gz
# 内容：Linux 二进制、部署脚本/单元、评测集、README

param([string]$Out = 'dist')

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format 'yyyyMMdd'
$stage = Join-Path $env:TEMP "coalrag-pkg-$stamp"

& (Join-Path $PSScriptRoot 'build-linux.ps1') | Out-Null
if (-not (Test-Path (Join-Path $root 'bin\linux\coalrag-server'))) {
    throw 'Linux 二进制未生成'
}

if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'bin') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'deploy') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $root $Out) | Out-Null

Copy-Item (Join-Path $root 'bin\linux') (Join-Path $stage 'bin') -Recurse
Copy-Item (Join-Path $root 'deploy\linux') (Join-Path $stage 'deploy') -Recurse
Copy-Item (Join-Path $root 'testdata') $stage -Recurse
Copy-Item (Join-Path $root 'README.md') $stage
Copy-Item (Join-Path $root 'docs\ACCEPTANCE.md') $stage

$tar = Join-Path $root "$Out\coalrag-linux-$stamp.tar.gz"
if (Test-Path $tar) { Remove-Item $tar -Force }
Push-Location $stage
try {
    tar -czf $tar *
    if ($LASTEXITCODE -ne 0) { throw 'tar 打包失败' }
} finally {
    Pop-Location
}

"$tar  $([math]::Round((Get-Item $tar).Length / 1MB, 1)) MB"
