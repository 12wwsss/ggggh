#!/usr/bin/env bash
# 一次性安装：目录、用户、systemd 单元、脚本权限。
# 假设二进制已拷到 /opt/coalrag/bin/，源码/配置模板在 /tmp/coalrag-src。
set -euo pipefail

SRC="${SRC:-/tmp/coalrag-src}"
ROOT="${ROOT:-/opt/coalrag}"

echo "[1/4] 用户与目录"
id -u coalrag >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin -d "$ROOT" coalrag
mkdir -p "$ROOT"/{bin,storage,scripts,backups} /etc/coalrag
chown -R coalrag:coalrag "$ROOT"

echo "[2/4] 脚本与环境文件"
cp "$SRC"/deploy/linux/backup.sh "$SRC"/deploy/linux/restore.sh "$SRC"/deploy/linux/watch.sh "$ROOT/scripts/"
chmod 0755 "$ROOT"/scripts/*.sh
if [ ! -f /etc/coalrag/coalrag.env ]; then
  cp "$SRC/deploy/linux/coalrag.env.example" /etc/coalrag/coalrag.env
  chmod 600 /etc/coalrag/coalrag.env
  echo "  已生成 /etc/coalrag/coalrag.env，记得改 DSN/JWT_SECRET/模型名"
fi
chown -R coalrag:coalrag "$ROOT/scripts"

echo "[3/4] systemd 单元"
cp "$SRC"/deploy/linux/systemd/*.service "$SRC"/deploy/linux/systemd/*.timer /etc/systemd/system/
systemctl daemon-reload

echo "[4/4] 启用"
systemctl enable --now coalrag-backup.timer coalrag-watch.service
systemctl enable coalrag.service llama-chat.service llama-embed.service

cat <<'EOF'

完成。下一步：
  1. 改 /etc/coalrag/coalrag.env（DSN、JWT_SECRET、模型名、MIN_SCORE）
  2. 确认模型文件与 llama-server 路径和 systemd 单元一致
  3. systemctl start llama-chat llama-embed coalrag
  4. journalctl -u coalrag -f
EOF
