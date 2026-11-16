#!/usr/bin/env bash
# 备份：pg_dump + storage + manifest(sha256)，保留最近 N 份。
#   /opt/coalrag/scripts/backup.sh
#   OUT=/data/backup KEEP=14 /opt/coalrag/scripts/backup.sh
set -euo pipefail

OUT="${OUT:-/opt/coalrag/backups}"
KEEP="${KEEP:-7}"
STORAGE="${STORAGE:-/opt/coalrag/storage}"
DSN="${COALRAG_DSN:?需要 COALRAG_DSN}"
STAMP="$(date +%Y%m%d-%H%M%S)"
DIR="$OUT/coalrag-$STAMP"

mkdir -p "$DIR"
echo "[$(date '+%F %T')] backup -> $DIR"

# 1) 数据库（自定义格式，支持并行恢复）
pg_dump --format=custom --no-owner --no-privileges --file="$DIR/db.dump" "$DSN"

# 2) 附件与评测产物
if [ -d "$STORAGE" ]; then
  tar -czf "$DIR/storage.tar.gz" -C "$(dirname "$STORAGE")" "$(basename "$STORAGE")"
fi

# 3) manifest
{
  echo "{"
  echo "  \"created_at\": \"$(date -Iseconds)\","
  echo "  \"host\": \"$(hostname)\","
  echo "  \"db_sha256\": \"$(sha256sum "$DIR/db.dump" | awk '{print $1}')\","
  if [ -f "$DIR/storage.tar.gz" ]; then
    echo "  \"storage_sha256\": \"$(sha256sum "$DIR/storage.tar.gz" | awk '{print $1}')\","
  fi
  echo "  \"db_size\": $(stat -c%s "$DIR/db.dump")"
  echo "}"
} > "$DIR/manifest.json"

# 4) 清理旧备份
ls -1dt "$OUT"/coalrag-* 2>/dev/null | tail -n +$((KEEP + 1)) | while read -r old; do
  echo "  remove $old"
  rm -rf "$old"
done

du -sh "$DIR"
echo "done"
