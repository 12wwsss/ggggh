#!/usr/bin/env bash
# 每日备份：知识库 + 数据 + 配置（不含密钥明文请自行确认权限）
# 用法：sudo bash backup.sh
# 定时：sudo crontab -e
#       30 2 * * * /opt/coal-ai/deploy/backup.sh >> /var/log/coal-ai-backup.log 2>&1

set -euo pipefail

APP_DIR="/opt/coal-ai"
DEST="/var/backups/coal-ai"
KEEP_DAYS=14
STAMP=$(date +%Y%m%d-%H%M)

mkdir -p "$DEST"

tar -czf "$DEST/coal-ai-$STAMP.tar.gz" \
  -C "$APP_DIR" data kb config.json 2>/dev/null || true

# 只留最近 N 天
find "$DEST" -name 'coal-ai-*.tar.gz' -mtime +"$KEEP_DAYS" -delete

echo "[$(date '+%F %T')] backup ok -> $DEST/coal-ai-$STAMP.tar.gz"
ls -lh "$DEST" | tail -n 5
