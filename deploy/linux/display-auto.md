## 显示自适应（有显示器 / 无显示器）

矿内服务器经常是“平时没接显示器、偶尔接一台”。两种状态下 Xorg 配置不同：

| 状态 | 配置 | 效果 |
| --- | --- | --- |
| 接了显示器 | `/etc/X11/xorg.conf.real` | 正常读 EDID，HDMI/DP 输出 |
| 没接显示器 | `/etc/X11/xorg.conf.virtual` | 强制虚拟 DP-0 1920×1080，供 AnyDesk |

开机由 `coalrag-display-auto.service` 自动判断并选择：

```
gdm 起来 → 等 X socket（最多 40s）
        → 用 xrandr 数 connected 输出
             >0  → 保持/恢复 real 配置
             =0  → 切 virtual 并重启 gdm
             读不到 → 保持现状（不做危险切换）
```

**检测必须用 X 的 `xrandr`，不能用 `/sys/class/drm/*/status`**：NVIDIA 驱动在这台机器上
即使接了显示器也报 `disconnected`，用 sysfs 会误判成“无显示器”并把屏幕切黑。

手动切换：

```bash
sudo coalrag-display real      # 接显示器
sudo coalrag-display virtual   # 拔显示器、只用远程
```

已知限制：

- 开机后才插显示器不会自动切回，需要 `coalrag-display real` 或重启；
- 切 virtual 会重启 gdm，远程桌面断几秒；
- 配置里 `BusID "PCI:23:0:0"` 是第一张卡（GPU0），显示器必须插这张卡。

排查命令：

```bash
journalctl -t coalrag-display -n 5          # 上次判断结果
DISPLAY=:0 XAUTHORITY=/run/user/1000/gdm/Xauthority xrandr --query | grep connected
```
