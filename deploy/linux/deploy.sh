#!/usr/bin/env bash
# 发布：deploy.sh /tmp/coalrag-server
# 1) 备份旧二进制 2) 安装新二进制 3) 重启 4) 健康检查失败自动回滚
set -euo pipefail

NEW=${1:?用法: deploy.sh /tmp/coalrag-server}
BIN=/opt/coalrag/bin/coalrag-server
TS=$(date +%Y%m%d-%H%M%S)

[ -x "$NEW" ] || { echo "文件不可执行: $NEW"; exit 1; }

cp -a "$BIN" "$BIN.bak-$TS" 2>/dev/null || true
install -m 0755 "$NEW" "$BIN"

systemctl restart coalrag

for _ in $(seq 1 10); do
  sleep 2
  if curl -fsS -m 3 http://127.0.0.1:8080/healthz 2>/dev/null | grep -q '"db"'; then
    echo "[deploy] ok $TS"
    exit 0
  fi
done

echo "[deploy] 健康检查失败，回滚到 $BIN.bak-$TS"
if [ -f "$BIN.bak-$TS" ]; then
  install -m 0755 "$BIN.bak-$TS" "$BIN"
  systemctl restart coalrag
fi
exit 1
