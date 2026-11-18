#!/usr/bin/env bash
# ============================================================
#  煤矿安全员 AI 智能体 · 公网云一键部署（Ubuntu 22.04 / Debian 11+）
#  用法：把整个 server 目录上传到 /opt/coal-ai 后执行
#        sudo bash /opt/coal-ai/deploy/install.sh
# ============================================================
set -euo pipefail

APP_DIR="/opt/coal-ai"
SVC_USER="coaliai"
NODE_MAJOR="20"

say() { printf "\n\033[1;33m==> %s\033[0m\n" "$1"; }
die() { printf "\n\033[1;31m[X] %s\033[0m\n" "$1"; exit 1; }

[ "$(id -u)" -eq 0 ] || die "请用 root 执行：sudo bash $0"
[ -f "$APP_DIR/index.js" ] || die "代码不在 $APP_DIR，请先把 server 目录上传到 $APP_DIR"

# ---------- 1. 系统依赖 ----------
say "安装系统依赖"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates nginx tar >/dev/null

# ---------- 2. Node.js ----------
if command -v node >/dev/null 2>&1 && [ "$(node -v | cut -d. -f1 | tr -d v)" -ge 18 ]; then
  say "已装 Node $(node -v)，跳过"
else
  say "安装 Node.js ${NODE_MAJOR}.x"
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get install -y -qq nodejs >/dev/null
fi
node -v

# ---------- 3. 服务账号与目录 ----------
say "创建运行账号 ${SVC_USER}"
id "$SVC_USER" >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$SVC_USER"
mkdir -p "$APP_DIR/data/logs" "$APP_DIR/kb"
chown -R "$SVC_USER":"$SVC_USER" "$APP_DIR"
chmod 600 "$APP_DIR/config.json" 2>/dev/null || true

# ---------- 4. systemd 服务 ----------
say "注册 systemd 服务"
install -m 644 "$APP_DIR/deploy/coal-ai.service" /etc/systemd/system/coal-ai.service
systemctl daemon-reload
systemctl enable coal-ai >/dev/null 2>&1 || true
systemctl restart coal-ai

sleep 2
if systemctl is-active --quiet coal-ai; then
  say "服务已启动"
else
  journalctl -u coal-ai -n 30 --no-pager || true
  die "服务启动失败，看上面的日志"
fi

# ---------- 5. Nginx ----------
say "配置 Nginx 反向代理"
install -m 644 "$APP_DIR/deploy/nginx-coal-ai.conf" /etc/nginx/sites-available/coal-ai.conf
ln -sf /etc/nginx/sites-available/coal-ai.conf /etc/nginx/sites-enabled/coal-ai.conf
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx

# ---------- 6. 防火墙 ----------
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
  say "放行 80/443"
  ufw allow 80/tcp >/dev/null
  ufw allow 443/tcp >/dev/null
fi

# ---------- 7. 自检 ----------
say "自检"
sleep 1
curl -s http://127.0.0.1:8787/api/health || die "本地接口不通"
echo
echo "---------------------------------------------"
echo " 部署完成"
echo " 1) 编辑密钥：nano $APP_DIR/config.json"
echo " 2) 改完重启：systemctl restart coal-ai"
echo " 3) 配域名+证书：见 deploy/部署手册.md 第 4 节"
echo "---------------------------------------------"
