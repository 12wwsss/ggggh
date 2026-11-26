# 小程序上线部署包

备案完成后，把小程序接到矿内服务需要三样东西：**一个已备案域名、一张证书、一台有公网 IP 的服务器**。
矿内服务不出入站端口，由 frpc 主动连云端。

```
手机/小程序 ──HTTPS(443)──▶ 云端 nginx stream（按 SNI 转发，不解密）
                                  │
                                  ▼
                            frps (云端, 7000/7100)
                                  ▲
                                  │ 主动长连接
                            frpc（矿内）
                                  │
                                  ▼
                    矿内 nginx TLS(8443) ──▶ Go 服务 127.0.0.1:8080
                    （证书私钥只在矿内）
```

## 0. 前置清单

| 项 | 说明 |
| --- | --- |
| 已备案域名 | 例 `rag.example.com`，备案主体与小程序主体一致 |
| 证书 | 域名证书 + 私钥，放进矿内 `/etc/coalrag/tls/` |
| 公网服务器 | 1 核 1G 够用，装 nginx + frps |
| 矿内出站 | 允许连云端 7000 端口 |
| 小程序后台 | request 合法域名、业务域名、downloadFile 域名 |
| 小程序插件 | 「微信同声传译」（语音提问/朗读）：**测试号加不了**，需正式小程序；未启用时长按语音=录音附件 |

### 服务器依赖（知识库导入）

`docx` 由内置解析器直接读，**不需要**额外软件；`pdf`、`doc`、`wps` 走 LibreOffice 子进程转换：

```bash
# Ubuntu / 麒麟：装一次即可，之后 PDF 与老 Word 才能导入
apt-get install -y libreoffice-writer --no-install-recommends
soffice --version   # 能打印版本即就绪
```

未安装时上传 PDF 会明确报「解析 PDF 需要安装 LibreOffice」，不会留下脏切片。
FAQ 用 CSV（`问题,答案` 两列）或 JSON（`[{"q":"…","a":"…"}]`），不依赖 LibreOffice。

## 1. DNS

`rag.example.com` A 记录指向云端公网 IP。生效后用 `nslookup rag.example.com` 确认。

## 2. 云端：nginx + frps

```bash
# 证书不需要放云端（四层转发不解密）
cp deploy/nginx/cloud-stream.conf /etc/nginx/conf.d/rag-stream.conf
cp deploy/frp/frps.toml /etc/frp/frps.toml   # 先改 token
systemctl reload nginx
systemctl restart frps
```

## 3. 矿内：frpc + nginx TLS + 服务

```bash
cp deploy/frp/frpc.toml /etc/frp/frpc.toml   # 改 serverAddr / token
cp deploy/nginx/mine-tls.conf /etc/nginx/conf.d/rag.conf  # 改域名与证书路径
cp deploy/systemd/coalrag.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now frpc coalrag
```

证书目录：

```
/etc/coalrag/tls/fullchain.pem
/etc/coalrag/tls/privkey.pem   # 权限 600，属主 root
```

## 4. 小程序后台

| 配置 | 填写 |
| --- | --- |
| request 合法域名 | `https://rag.example.com` |
| 业务域名（web-view） | `https://rag.example.com`，需校验文件 |
| downloadFile 合法域名 | `https://rag.example.com` |
| uploadFile 合法域名 | 同上（上报隐患拍照用） |

校验文件放 `web/public/<文件名>.txt` → 重新构建前端与服务端 → 重启服务。
详见 `web/public/README.md`。

## 5. 外网自检

```powershell
powershell -ExecutionPolicy Bypass -File scripts\check-public.ps1 `
  -Domain rag.example.com -VerifyFile abc123.txt
```

检查 DNS、443 连通、证书剩余天数、`/healthz`、校验文件、静态资源，任何一项失败退出码 1。

## 6. 常见问题

| 现象 | 原因 |
| --- | --- |
| 小程序报“不在以下 request 合法域名列表中” | 后台没配或配了 http；改完要重新编译小程序 |
| web-view 白屏，提示“不支持打开非业务域名” | 业务域名未校验通过，检查 .txt 是否可访问 |
| 校验文件返回 HTML | 文件没进 `web/public/` 或没重新编译二进制 |
| 证书报错 | 云端做了 TLS 终结导致 SNI 不匹配；必须四层转发 |
| 手机能开、电脑不能 | 只放行了 443，检查公司网络是否拦截 |

## 回滚

```bash
systemctl stop frpc            # 断掉隧道，外网立即不可达
systemctl restart coalrag      # 只影响矿内
```

小程序侧可先在后台把域名移除，避免请求打到旧地址。
