#!/usr/bin/env bash
# 健康巡检：HTTP 不通重启 coalrag；数据库 down 先重启 postgres 再重启 coalrag。
# 结果追加到 /opt/coalrag/logs/monitor.log，并输出到 stdout（journald 可查）。
set -uo pipefail

LOG=/opt/coalrag/logs/monitor.log
URL=${1:-http://127.0.0.1:8080/healthz}
TS=$(date '+%F %T')
mkdir -p "$(dirname "$LOG")"

body=$(curl -s -m 5 "$URL") || {
  echo "$TS http-fail -> restart coalrag" | tee -a "$LOG"
  systemctl restart coalrag
  exit 1
}

db=$(printf '%s' "$body" | grep -o '"db":{"status":"[a-z]*"' | head -1)
llm=$(printf '%s' "$body" | grep -o '"llm":{"status":"[a-z]*"' | head -1)

if ! printf '%s' "$body" | grep -q '"db":{"status":"up"'; then
  echo "$TS db-down ($db) -> restart postgresql + coalrag" | tee -a "$LOG"
  systemctl restart postgresql
  sleep 2
  systemctl restart coalrag
  exit 1
fi

if ! printf '%s' "$body" | grep -q '"llm":{"status":"up"'; then
  echo "$TS warn: llm-down ($llm)" | tee -a "$LOG"
  exit 0
fi

echo "$TS ok" >> "$LOG"
