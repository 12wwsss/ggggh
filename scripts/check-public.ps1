# External check after wiring the tunnel: DNS, TLS, health, WeChat verify file.
#
#   powershell -ExecutionPolicy Bypass -File scripts\check-public.ps1 -Domain rag.example.com
#   powershell -ExecutionPolicy Bypass -File scripts\check-public.ps1 -Domain rag.example.com -VerifyFile abc123.txt

param(
    [Parameter(Mandatory = $true)][string]$Domain,
    [string]$VerifyFile = '',
    [int]$MinCertDays = 7
)

$ErrorActionPreference = 'Stop'
$failed = 0
function Check([bool]$ok, [string]$msg) {
    if ($ok) { Write-Host "  PASS  $msg" }
    else { Write-Host "  FAIL  $msg" -ForegroundColor Red; $script:failed++ }
}

Write-Host "== DNS =="
try {
    $ips = [System.Net.Dns]::GetHostAddresses($Domain) | ForEach-Object { $_.IPAddressToString }
    Check ($ips.Count -gt 0) ("$Domain -> " + ($ips -join ', '))
} catch {
    Check $false "解析失败：$($_.Exception.Message)"
}

Write-Host "== TLS =="
try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $tcp.Connect($Domain, 443)
    $ssl = New-Object System.Net.Security.SslStream($tcp.GetStream(), $false, { $true })
    $ssl.AuthenticateAsClient($Domain)
    $cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($ssl.RemoteCertificate)
    $days = [int](($cert.NotAfter - (Get-Date)).TotalDays)
    Check ($cert.Subject -match [regex]::Escape($Domain)) "证书域名匹配：$($cert.Subject)"
    Check ($days -ge $MinCertDays) "证书剩余 $days 天"
    $ssl.Close(); $tcp.Close()
} catch {
    Check $false "TLS 握手失败：$($_.Exception.Message)"
}

Write-Host "== API =="
try {
    $h = Invoke-RestMethod -Uri "https://$Domain/healthz" -TimeoutSec 10
    Check ($h.status -eq 'ok') "healthz=$($h.status)"
} catch {
    Check $false "healthz 不可用：$($_.Exception.Message)"
}

try {
    $r = Invoke-WebRequest -Uri "https://$Domain/" -TimeoutSec 10
    Check ($r.StatusCode -eq 200 -and $r.Content -match 'app') "首页可打开（$($r.StatusCode)）"
} catch {
    Check $false "首页不可用：$($_.Exception.Message)"
}

if ($VerifyFile) {
    Write-Host "== 业务域名校验文件 =="
    try {
        $r = Invoke-WebRequest -Uri "https://$Domain/$VerifyFile" -TimeoutSec 10
        Check ($r.StatusCode -eq 200) "校验文件可访问（$($r.StatusCode)）"
        Check ($r.Headers['Content-Type'] -notmatch 'text/html') "返回类型不是 HTML（$($r.Headers['Content-Type'])）"
    } catch {
        Check $false "校验文件不可访问：$($_.Exception.Message)"
    }
}

Write-Host ''
if ($failed -gt 0) { Write-Host "外网自检未通过（$failed 项）" -ForegroundColor Red; exit 1 }
Write-Host '外网自检通过'
