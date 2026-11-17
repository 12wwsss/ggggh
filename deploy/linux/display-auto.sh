#!/bin/bash
# 开机自适应显示：检测到真实显示器就用真实配置，否则切虚拟屏。
# 由 coalrag-display-auto.service 在 gdm 起来后执行一次。
#
# 注意：X 的 Xauthority 在“greeter 阶段”和“用户会话阶段”是两份，
# 必须都试；任何一份都读不到时保持现状，绝不因为“读不到”就切虚拟屏。
set -u
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
# systemd 系统服务里 HOME 可能未设置，set -u 下直接引用会退出
HOME="${HOME:-/root}"

log() { logger -t coalrag-display "$*"; }

GDM_UID=$(id -u gdm 2>/dev/null || echo 120)
USER_UID=$(id -u "${SUDO_USER:-$(logname 2>/dev/null || echo scl)}" 2>/dev/null || echo 1000)

# 等 X 起来（最多 40 秒）
for _ in $(seq 1 40); do
    [ -S /tmp/.X11-unix/X0 ] && break
    sleep 1
done
if [ ! -S /tmp/.X11-unix/X0 ]; then
    log "X 未就绪，跳过"
    exit 0
fi

# 依次尝试可能的 Xauthority，返回 xrandr 的原始输出；全部失败返回空。
# 用通配符遍历，避免 systemd 下 $USER/logname 取到 root 而漏掉真实用户。
probe() {
    for auth in \
        /run/user/*/gdm/Xauthority \
        /home/*/.Xauthority \
        "$HOME/.Xauthority"; do
        [ -f "$auth" ] || continue
        out=$(DISPLAY=:0 XAUTHORITY="$auth" xrandr --query 2>/dev/null) || {
            log "探测失败 auth=$auth rc=$?"
            continue
        }
        case "$out" in
            *"Screen 0:"*) printf '%s' "$out"; return 0 ;;
        esac
    done
    return 1
}

# gdm 重启期间 Xauthority 会短暂失效，重试几次再下结论
out=""
for _ in 1 2 3 4 5; do
    if out=$(probe); then break; fi
    sleep 3
done
if [ -z "$out" ]; then
    log "无法读取 X 输出（Xauthority 均无效），保持当前配置"
    exit 0
fi

connected=$(printf '%s\n' "$out" | grep -c " connected")
log "connected outputs: $connected"

if [ "$connected" -gt 0 ]; then
    if ! cmp -s /etc/X11/xorg.conf /etc/X11/xorg.conf.real 2>/dev/null; then
        cp /etc/X11/xorg.conf.real /etc/X11/xorg.conf
        log "检测到显示器，恢复真实配置并重启 gdm"
        systemctl restart gdm3
    fi
    exit 0
fi

if ! cmp -s /etc/X11/xorg.conf /etc/X11/xorg.conf.virtual 2>/dev/null; then
    cp /etc/X11/xorg.conf.virtual /etc/X11/xorg.conf
    log "无显示器，切换虚拟屏并重启 gdm"
    systemctl restart gdm3
fi
exit 0
