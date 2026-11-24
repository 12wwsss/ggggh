#!/usr/bin/env bash
# 一键自检：服务状态 + 健康接口 + 全量 smoke
set -uo pipefail

BASE=${BASE:-http://127.0.0.1:8080}
DATA=/opt/coalrag/testdata
FAIL=0

echo "== 服务状态 =="
for s in postgresql coalrag llama-embed llama-chat; do
  st=$(systemctl is-active "$s")
  printf '%-12s %s\n' "$s" "$st"
  [ "$st" = "active" ] || FAIL=1
done

echo
echo "== 健康检查 =="
if ! curl -fsS -m 5 "$BASE/healthz"; then
  echo "healthz 失败"
  FAIL=1
fi

echo
echo "== 兼容接口 =="
curl -fsS -m 5 "$BASE/api/health" >/dev/null && echo "api/health ok" || { echo "api/health 失败"; FAIL=1; }
code=$(curl -s -o /dev/null -w '%{http_code}' -m 5 "$BASE/api/options")
echo "api/options $code"
[ "$code" = "200" ] || FAIL=1

echo
echo "== smoke 全量自测 =="
if [ -x /opt/coalrag/bin/smoke ] && [ -f "$DATA/smoke-doc.docx" ]; then
  ( cd /opt/coalrag && /opt/coalrag/bin/smoke -base "$BASE" -doc testdata/smoke-doc.docx ) 2>&1 | tail -3
else
  echo "跳过：缺少 smoke 或 $DATA/smoke-doc.docx"
fi

exit "$FAIL"
