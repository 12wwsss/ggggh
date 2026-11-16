# 来源 IP 白名单：实现“不在矿内局域网不能访问”

## 为什么这样做

小程序/手机无法可靠判断“是否连了矿内 WiFi”（SSID 安卓才读得到、可伪造）。
但矿内 WiFi 出公网时走**固定出口 IP**，4G 走运营商——在入口按来源 IP 放行，就是
“只有矿内网络能进”，且用户无感。

## 两层拦截

| 层 | 文件 | 作用 |
| --- | --- | --- |
| 云端 L4（硬） | `deploy/nginx/cloud-allowlist.conf` | 非白名单连 TLS 握手都建立不了 |
| 矿内 L7（软） | `deploy/nginx/mine-allowlist.conf` | 放行/拒绝 + 返回“请连接矿内网络”提示 |

只上 L4 也能用，但 4G 用户只会看到“网络错误”；加了 L7 才有中文提示。

## 需要的信息

| 信息 | 从哪来 | 用途 |
| --- | --- | --- |
| 矿内出口公网 IP 或网段 | 矿内任意电脑访问 `https://ifconfig.me` 或问网络管理员 | 白名单条目 |
| 是否固定 | 网络管理员确认 | 动态 IP 要加更新脚本 |
| 矿内网段（可选） | 网络管理员 | 如果矿内 DNS 解析到内网地址，直接按内网 IP 放行 |
| 是否走 IPv6 | 同上 | 走 IPv6 要加 v6 段，否则关掉域名的 AAAA |

## 实施步骤

1. **矿内**：改 `/etc/frp/frpc.toml` 的 proxy，加 `transport.proxyProtocolVersion = "v2"`，重启 frpc；
2. **矿内**：用 `mine-allowlist.conf` 替换原来的 `mine-tls.conf`，把 `allow` 换成矿内出口 IP，`nginx -t && systemctl reload nginx`；
3. **云端**：用 `cloud-allowlist.conf`，同样替换 allow 网段，reload nginx；
4. **验证**：
   - 矿内 WiFi：`curl -I https://rag.example.com/healthz` → 200；
   - 4G 热点：同一条命令 → 连接被拒（L4）或 403 + “请连接矿内网络”（L7）；
   - 矿内 nginx 日志里 `$remote_addr` 应该是手机的公网 IP，不是 127.0.0.1（否则 PROXY protocol 没生效）。

## 动态出口 IP（不固定时）

矿内出口 IP 变了白名单会失效，两种处理：

1. 定时更新：矿内机器每 5 分钟取一次出口 IP，变化时 SSH 到云端改 nginx 配置并 reload（需云端只授这一条命令的权限）；
2. 放弃 IP 白名单，改设备绑定：首次在矿内由管理员授权绑定手机，之后只认设备（换手机要找管理员）。

## 常见坑

| 现象 | 原因 |
| --- | --- |
| 矿内用户也被 403 | 白名单写的是内网段，但流量走公网出口；应写出口公网 IP |
| 所有人都是 127.0.0.1 | frpc 没开 proxy protocol，或 nginx 没加 `proxy_protocol` |
| nginx 起不来 | `listen ... proxy_protocol` 与 frpc 的设置必须同时改，否则握手失败 |
| 4G 能进 | 只改了矿内 nginx，云端 L4 没配；或矿内 DNS 把域名解析到了公网，4G 仍走隧道 |
| IPv6 用户绕过 | 域名有 AAAA 记录但白名单没配 v6；先删 AAAA 或补 v6 段 |
