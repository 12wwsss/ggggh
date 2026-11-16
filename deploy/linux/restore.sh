#!/usr/bin/env bash
# 恢复：校验 manifest 的 sha256，然后重建库并还原 storage。
#   /opt/coalrag/scripts/restore.sh /opt/coalrag/backups/coalrag-20261113-023000
# 服务必须先停：systemctl stop coalrag
set -euo pipefail

DIR="${1:?用法: restore.sh <备份目录>}"
DSN="${COALRAG_DSN:?需要 COALRAG_DSN}"
STORAGE="${STORAGE:-/opt/coalrag/storage}"

[ -f "$DIR/manifest.json" ] || { echo "缺少 manifest.json"; exit 1; }
[ -f "$DIR/db.dump" ] || { echo "缺少 db.dump"; exit 1; }

want_db="$(grep -o '"db_sha256": "[a-f0-9]*"' "$DIR/manifest.json" | cut -d'"' -f4)"
got_db="$(sha256sum "$DIR/db.dump" | awk '{print $1}')"
[ "$want_db" = "$got_db" ] || { echo "db.dump 校验失败：$got_db != $want_db"; exit 1; }

if [ -f "$DIR/storage.tar.gz" ]; then
  want_st="$(grep -o '"storage_sha256": "[a-f0-9]*"' "$DIR/manifest.json" | cut -d'"' -f4)"
  got_st="$(sha256sum "$DIR/storage.tar.gz" | awk '{print $1}')"
  [ "$want_st" = "$got_st" ] || { echo "storage.tar.gz 校验失败"; exit 1; }
fi

read -r -p "将覆盖当前数据库，输入 yes 继续: " ans
[ "$ans" = "yes" ] || { echo "已取消"; exit 1; }

pg_restore --clean --if-exists --no-owner --no-privileges --dbname="$DSN" "$DIR/db.dump"

if [ -f "$DIR/storage.tar.gz" ]; then
  rm -rf "$STORAGE"
  tar -xzf "$DIR/storage.tar.gz" -C "$(dirname "$STORAGE")"
fi

echo "恢复完成，启动服务：systemctl start coalrag"
