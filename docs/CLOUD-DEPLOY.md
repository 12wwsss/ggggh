# 云服务器部署与甲方远程测试配置清单

> 版本：2026-11-27 ｜ 模型：Qwen2.5-32B-Instruct Q4_K_M ｜ 测试方式：**仅网页端**，小程序代码保持更新但不做真机测试

## 一、Qwen2.5-32B Q4_K_M 的显存测算

模型结构：64 层、8 个 KV head、head_dim 128，FP16 KV 缓存约 **0.25 MB / token**。
权重 Q4_K_M 约 **19.8 GB**；另需约 **2 GB** 计算缓冲与 CUDA 上下文。

| 上下文 | 并发槽 | KV 缓存 | 权重+缓冲 | **合计** | 32GB 卡 |
|---|---|---|---|---|---|
| 8k | 1 | 2.0 GB | 21.8 GB | **≈ 24 GB** | ✅ 宽裕 |
| 8k | 2 | 4.0 GB | 21.8 GB | **≈ 26 GB** | ✅ 推荐（留 6GB 余量） |
| 8k | 4 | 8.0 GB | 21.8 GB | **≈ 30 GB** | ⚠️ 极限，不建议 |
| 16k | 1 | 4.0 GB | 21.8 GB | **≈ 26 GB** | ✅ 可以 |
| 16k | 2 | 8.0 GB | 21.8 GB | **≈ 30 GB** | ⚠️ 极限 |
| 32k | 1 | 8.0 GB | 21.8 GB | **≈ 30 GB** | ⚠️ 极限，一有波动就 OOM |

**结论：32GB 显存选 `-c 8192 --parallel 2`**（或 `-c 16384 --parallel 1`）。
想跑 16k×2 或 32k×2，需要 **48GB 显存**（L20 48G / A6000 48G / 2×24G）。

### 推荐配置

| 项 | 最低 | **推荐** |
|---|---|---|
| 显存 | 32 GB（V100 32G） | 48 GB（L20 48G） |
| 内存 | 32 GB + 8GB swap | **64 GB** |
| vCPU | 8 | 16 |
| 数据盘 | 200 GB | **300 GB**（模型 20.6GB + 库 + 上传 + 备份） |
| 带宽 | 5 Mbps | 5–10 Mbps |
| 系统 | Ubuntu 22.04/24.04 LTS | 同左 |

> 32GB 内存能跑，但加载 32B 权重时内存峰值约 22GB，再叠加 PostgreSQL、LibreOffice、备份会很紧；**64GB 内存是更稳的选择**。

### 启动参数

```bash
# chat
llama-server -m /opt/models/qwen2.5-32b-instruct-q4_k_m.gguf \
  --host 127.0.0.1 --port 8001 \
  -c 8192 --parallel 2 -ngl 99 -t 8 --jinja

# embed（bge-m3 放 CPU，检索已有 540 QPS，不必占显存）
llama-server -m /opt/models/bge-m3-Q8_0.gguf \
  --host 127.0.0.1 --port 8002 --embeddings --pooling cls -c 512 -t 8
```

## 二、服务器组件与端口

| 组件 | 版本 | 监听 | 对外 |
|---|---|---|---|
| Ubuntu | 24.04 LTS | — | — |
| PostgreSQL | 16 | `127.0.0.1:5432` | ❌ |
| nginx | 最新稳定版 | `0.0.0.0:80/443` | ✅ |
| coalrag-server | 静态二进制 | `127.0.0.1:8080` | ❌（经 nginx） |
| llama-server chat | llama.cpp | `127.0.0.1:8001` | ❌ |
| llama-server embed | llama.cpp | `127.0.0.1:8002` | ❌ |
| poppler-utils / libreoffice-writer | — | — | — |

安全组只开 **22（限办公 IP）、80、443**；其余全部内网。

## 三、甲方远程测试需要的外部条件

| 条件 | 说明 | 谁准备 |
|---|---|---|
| 公网 IP | 云主机弹性公网 IP，固定 | 你 |
| 域名（可选但建议） | 如 `ai.xxx.com`，A 记录指向公网 IP | 你/甲方 |
| HTTPS 证书 | Let's Encrypt（certbot 自动续期），浏览器不再报不安全 | 你 |
| ICP 备案 | **大陆服务器必须**，约 2–3 周；香港/新加坡可先跳过 | 甲方主体 |
| 安全组放行 443 | 80 跳 443；22 限制来源 IP | 你 |
| 甲方出口网络 | 能访问 443（公司网络若有白名单需加域名/IP） | 甲方 |
| 测试账号 | 给甲方开通 1 个管理员 + 1 个安全科 + 1 个员工账号，首次登录改密 | 你 |
| 浏览器 | Chrome / Edge 最新版（不支持 IE） | 甲方 |
| 并发预期 | 32GB 显存建议 **同时问答 ≤2 人**；看资料/学习/报表不受限 | 双方 |

### 甲方访问地址（示例）

```
https://ai.xxx.com/           员工端（网页版）
https://ai.xxx.com/admin/     管理后台
```

没有域名时退回 `http://<公网IP>:8080`，但**浏览器会提示不安全，且后续小程序必须 HTTPS**，所以建议一次把域名+证书配好。

### nginx 关键配置

```nginx
server {
    listen 443 ssl http2;
    server_name ai.xxx.com;
    ssl_certificate     /etc/letsencrypt/live/ai.xxx.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ai.xxx.com/privkey.pem;

    client_max_body_size 64m;          # 文档/视频上传
    proxy_read_timeout 180s;           # 问答最长等待
    proxy_send_timeout 180s;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
server {
    listen 80;
    server_name ai.xxx.com;
    return 301 https://$host$request_uri;
}
```

## 四、部署步骤

1. 建用户与目录：`/opt/coalrag/{bin,storage,backups}`、`/opt/models`；
2. 装依赖：`postgresql-16`、`nginx`、`poppler-utils`、`libreoffice-writer`；
3. 建库 `coalrag`，账号密码写进 `/etc/coalrag/coalrag.env`；
4. 下载模型（ModelScope 更快）并 `sha256sum` 校验；
5. 编译 llama.cpp（CUDA），写 `llama-chat` / `llama-embed` 两个 systemd 单元；
6. 上传 `coalrag-server`，写 `coalrag.service`；
7. nginx 站点 + certbot 证书；
8. 启动顺序：PostgreSQL → llama-embed → llama-chat（加载 32B 约 1–3 分钟）→ coalrag → nginx；
9. 验证：`/healthz` 的 `llm` 为 up；后台能传文档、能问答并带引用；员工端能登录；
10. 配 `pg_dump` + `storage` 每日备份 timer。

## 五、甲方测试内容建议

- 问答：10 条本矿真实问题（瓦斯、顶板、运输、机电），看是否带条款引用；
- 知识库：上传 1 份规程 docx + 1 份 PDF，看切分与检索；
- 隐患：上报 1 条 → 后台受理 → 整改 → 闭环；
- 宣贯：发布 1 条 → 员工签收；
- 报表：看隐患趋势、按岗位分布、导出一人一档。

## 六、小程序处理策略（本轮不测试）

| 项 | 做法 |
|---|---|
| 代码 | 继续随网页版同步更新，推 GitHub，后端 `/api/*` 兼容层保持不变 |
| 部署 | 不发布体验版、不做真机测试 |
| 原因 | 真机调试需要 HTTPS 域名 + 小程序后台 request 合法域名（备案域名）；现在只有 IP:8080 |
| 何时开 | 域名备案 + 证书就绪后：小程序后台加合法域名 → 开发者工具上传体验版 → 真机测试 |
| 数据 | 网页版与小程序共用同一数据库；测试期间建议用独立测试库或测试账号，避免污染 |

## 七、上线前自查

- [ ] `https://域名/` 与 `/admin/` 均能打开
- [ ] `/healthz` 显示 `llm: up`、`db: up`
- [ ] 问答 3 题，均有引用且不超时
- [ ] 后台能上传 docx / pdf / doc 并切分
- [ ] 安全组外网只能连 22/80/443
- [ ] 数据库与 `/opt/coalrag/storage` 在备份范围内
- [ ] 重启机器后 4 个服务全部自启
- [ ] 甲方账号可登录、首登改密

## 八、风险提示

1. **32GB 显存不要开 32k×多并发**，OOM 后 llama-server 会反复重启；
2. **32B 首次加载慢**（1–3 分钟），部署后先预热再给甲方；
3. **备案周期**是最大变量，急用先上香港节点，网页版不受影响；
4. **测试数据会进正式库**，建议给甲方测试单独建库或定期清理；
5. 32GB 内存机器务必加 swap，否则大文档解析时可能被 OOM Killer 杀掉。
