# 生产化部署与运维

本目录是煤炭安全员平台的部署脚本与配置模板，服务器路径约定：

```
/opt/coalrag/bin/coalrag-server     主程序
/opt/coalrag/scripts/               本目录脚本
/opt/coalrag/storage/               上传文件（uploads、media、issues）
/opt/coalrag/logs/                  日志（logrotate 管理）
/opt/coalrag/backups/               备份
/etc/coalrag/coalrag.env            环境变量（含 DSN、JWT）
/etc/systemd/system/coalrag.service
```

## 首次部署

```bash
# 1. 目录与用户（已存在可跳过）
useradd -r -s /sbin/nologin coalrag || true
mkdir -p /opt/coalrag/{bin,scripts,storage,logs,backups,testdata}
chown -R coalrag:coalrag /opt/coalrag

# 2. 安装脚本与单元
install -m 0755 deploy/linux/*.sh /opt/coalrag/scripts/
install -m 0644 deploy/linux/coalrag.service /etc/systemd/system/
install -m 0644 deploy/linux/coalrag-backup.* /etc/systemd/system/
install -m 0644 deploy/linux/coalrag-monitor.* /etc/systemd/system/
install -m 0644 deploy/linux/logrotate-coalrag /etc/logrotate.d/coalrag

systemctl daemon-reload
systemctl enable --now coalrag
systemctl enable --now coalrag-backup.timer
systemctl enable --now coalrag-monitor.timer
```

## 发布新版本

```bash
scp bin/linux/coalrag-server scl@服务器:/tmp/coalrag-server
ssh scl@服务器 'sudo /opt/coalrag/scripts/deploy.sh /tmp/coalrag-server'
```

健康检查失败会自动回滚到上一版二进制（`/opt/coalrag/bin/coalrag-server.bak-时间`）。

## 备份与恢复

```bash
# 手动备份
sudo /opt/coalrag/scripts/backup.sh

# 查看
ls -lh /opt/coalrag/backups/

# 恢复（会覆盖数据库和 storage）
sudo /opt/coalrag/scripts/restore.sh /opt/coalrag/backups/20261124-023000 --yes
```

- 每天 02:30 自动备份，保留 14 天；
- 备份内容：PostgreSQL 自定义格式 dump + storage 压缩包 + SHA256；
- 建议每周把备份目录同步到另一台机器或 NAS。

## 监控

```bash
systemctl list-timers | grep coalrag
sudo /opt/coalrag/scripts/monitor.sh
tail -20 /opt/coalrag/logs/monitor.log
```

- 每 5 分钟检查一次 `/healthz`；
- HTTP 不通 → 自动重启 coalrag；
- 数据库 down → 先重启 postgresql，再重启 coalrag；
- 模型 down → 只记日志（不会反复重启模型导致显存抖动）。

## 一键自检

```bash
# 需要 /opt/coalrag/testdata/smoke-doc.docx
sudo /opt/coalrag/scripts/check.sh
```

会检查服务状态、健康接口，并跑一遍 smoke（103 项）。

## HTTPS / 公网接入（备案后）

1. 矿内服务器跑 `frpc`，把本机 8080 映射到公网入口机 18080（见 `frpc.toml`）；
2. 公网入口机跑 `frps`（见 `frps.toml`），并用 nginx 反代 18080（见 `nginx-coalrag.conf`）；
3. 域名解析到入口机，申请证书（certbot），把 `server_name`、证书路径换掉；
4. 小程序 `utils/config.js` 的 `baseUrl` 改成 `https://域名`；
5. 小程序后台把域名加入 request 合法域名。

## 安全注意

- 8080 只应在内网/本机监听；公网只暴露 443；
- `/etc/coalrag/coalrag.env` 权限保持 600，里面有 DSN 和 JWT 密钥；
- 定期检查 `journalctl -u coalrag -p warning`。
