#!/usr/bin/env bash
# 从备份恢复：restore.sh <备份目录> --yes
set -euo pipefail

DEST=${1:?用法: restore.sh /opt/coalrag/backups/20261124-023000 --yes}
CONFIRM=${2:-}
if [ "$CONFIRM" != "--yes" ]; then
  echo "这会覆盖当前数据库和 storage，确认请加 --yes"
  exit 2
fi

ENV_FILE=${ENV_FILE:-/etc/coalrag/coalrag.env}
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a
STORAGE_DIR=${COALRAG_STORAGE_DIR:-/opt/coalrag/storage}

[ -f "$DEST/db.dump" ] || { echo "备份缺少 db.dump"; exit 1; }
[ -f "$DEST/storage.tar.gz" ] || { echo "备份缺少 storage.tar.gz"; exit 1; }

echo "[restore] 停止服务"
systemctl stop coalrag

echo "[restore] 恢复数据库"
pg_restore --clean --if-exists --no-owner --dbname="$COALRAG_DSN" "$DEST/db.dump"

echo "[restore] 恢复 storage"
rm -rf "${STORAGE_DIR:?}"/*
tar -C "$(dirname "$STORAGE_DIR")" -xzf "$DEST/storage.tar.gz"
chown -R coalrag:coalrag "$STORAGE_DIR"

echo "[restore] 启动服务"
systemctl start coalrag
sleep 3
curl -fsS -m 5 http://127.0.0.1:8080/healthz >/dev/null && echo "[restore] 完成，健康检查通过"
