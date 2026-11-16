#!/usr/bin/env bash
# 探活：轮询 healthz/metrics，异常写日志并可选 POST webhook。
#   systemctl start coalrag-watch
#   ONCE=1 /opt/coalrag/scripts/watch.sh
set -uo pipefail

BASE="${BASE:-http://127.0.0.1:8080}"
INTERVAL="${INTERVAL:-60}"
WEBHOOK="${WEBHOOK:-}"
MAX_FAILED="${MAX_FAILED:-5}"
MAX_OVERDUE="${MAX_OVERDUE:-30}"
LOG="${LOG:-/var/log/coalrag-watch.log}"

log() { echo "$(date '+%F %T') [$1] $2" | tee -a "$LOG"; }

alert() {
  log ALERT "$1"
  if [ -n "$WEBHOOK" ]; then
    curl -s -m 10 -H 'Content-Type: application/json' \
      -d "{\"msgtype\":\"text\",\"text\":{\"content\":\"CoalRAG: $1\"}}" "$WEBHOOK" >/dev/null || true
  fi
}

check_once() {
  local health failed overdue
  health="$(curl -s -m 8 "$BASE/healthz" || true)"
  if [ -z "$health" ]; then
    alert "healthz 不可达"; return 1
  fi
  if ! echo "$health" | grep -q '"status":"ok"'; then
    alert "healthz 异常: $health"; return 1
  fi

  local metrics
  metrics="$(curl -s -m 8 "$BASE/metrics" || true)"
  failed="$(echo "$metrics" | sed -n 's/^coalrag_tasks_failed_recent \([0-9]*\)/\1/p' | head -1)"
  overdue="$(echo "$metrics" | sed -n 's/^coalrag_issues_overdue \([0-9]*\)/\1/p' | head -1)"
  failed="${failed:-0}"; overdue="${overdue:-0}"

  if [ "$failed" -gt "$MAX_FAILED" ]; then alert "近一小时失败任务 $failed"; return 1; fi
  if [ "$overdue" -gt "$MAX_OVERDUE" ]; then alert "超时未闭环问题 $overdue"; return 1; fi

  log OK "all checks passed"
  return 0
}

if [ "${ONCE:-0}" = "1" ]; then
  check_once
  exit $?
fi

log INFO "watchdog started (interval ${INTERVAL}s, base $BASE)"
while true; do
  check_once || true
  sleep "$INTERVAL"
done
