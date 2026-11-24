#!/usr/bin/env bash
# 备份数据库 + storage（上传附件、视频），默认保留 14 天。
set -euo pipefail

ENV_FILE=${ENV_FILE:-/etc/coalrag/coalrag.env}
BACKUP_ROOT=${BACKUP_ROOT:-/opt/coalrag/backups}
RETENTION_DAYS=${RETENTION_DAYS:-14}

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

: "${COALRAG_DSN:?COALRAG_DSN 未配置}"
STORAGE_DIR=${COALRAG_STORAGE_DIR:-/opt/coalrag/storage}

STAMP=$(date +%Y%m%d-%H%M%S)
DEST="$BACKUP_ROOT/$STAMP"
mkdir -p "$DEST"

echo "[backup] dump database -> $DEST/db.dump"
pg_dump --dbname="$COALRAG_DSN" -Fc -f "$DEST/db.dump"

echo "[backup] archive storage -> $DEST/storage.tar.gz"
tar -C "$(dirname "$STORAGE_DIR")" -czf "$DEST/storage.tar.gz" "$(basename "$STORAGE_DIR")"

pg_restore -l "$DEST/db.dump" >/dev/null
sha256sum "$DEST/db.dump" "$DEST/storage.tar.gz" > "$DEST/SHA256SUMS"

find "$BACKUP_ROOT" -maxdepth 1 -type d \( -name '20*' -o -name 'coalrag-*' \) -mtime +"$RETENTION_DAYS" -exec rm -rf {} + 2>/dev/null || true

SIZE=$(du -sh "$DEST" | cut -f1)
echo "[backup] ok $DEST ($SIZE)"
