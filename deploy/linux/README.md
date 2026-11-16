# Ubuntu 离线部署（单机）

目标机器：Ubuntu 24.04 LTS，Xeon Silver 4210R（20 核 40 线程），128 GB 内存，4.5 TB 磁盘。
服务、数据库、模型全部跑在这一台，数据不出机。

## 0. 先确认三件事

```bash
nvidia-smi || echo "无 NVIDIA 驱动/显卡"
lspci | grep -Ei 'nvidia|vga|3d'
nproc; free -h; df -h /
```

| 情况 | 模型选择 | 预期 |
| --- | --- | --- |
| 有 ≥16 GB 显存的 NVIDIA 卡 | 14B Q4_K_M（约 9 GB），`-ngl 99` | 20–40 token/s |
| 只有 CPU | 7B/8B Q4_K_M（约 5 GB），`-t 20` | 3–6 token/s |
| CPU 硬跑 14B Q4 | 可以，但慢 | 1.5–3 token/s，一道题要 40–90 秒 |

**没有显卡时不要硬上 14B**：128 GB 内存能装下，但吞吐撑不住多人同时问。先用 7B 把链路跑通，再按实际效果决定。

## 1. 传文件（离线摆渡）

在开发机执行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build-linux.ps1
# 产物在 bin\linux\，纯静态编译，不依赖服务器 glibc
scp bin\linux\* user@server:/opt/coalrag/bin/
scp -r testdata deploy scripts user@server:/tmp/coalrag-src/
```

模型文件（GGUF）从 ModelScope / HuggingFace 下载后同样走摆渡，落地先校验：

```bash
sha256sum /opt/models/*.gguf
```

## 2. 安装依赖（离线）

需要：PostgreSQL 16（Ubuntu 24.04 自带）、ca-certificates、tzdata。
有网时 `apt install -y postgresql`；离线时在**同版本**机器上下载 deb 再 `dpkg -i`。

```bash
sudo apt-get install -y postgresql postgresql-contrib
sudo systemctl enable --now postgresql

sudo -u postgres psql <<'SQL'
create role coalrag login password '换成强口令';
create database coalrag owner coalrag encoding 'UTF8';
SQL
```

## 3. 模型与 llama.cpp

```bash
sudo mkdir -p /opt/llama /opt/models
# 预编译包：llama-bXXXX-bin-ubuntu-x64.zip（CPU）或 cuda 版，解压到 /opt/llama
chmod +x /opt/llama/llama-server

# 冒烟：直接用命令行问一句
/opt/llama/llama-server -m /opt/models/qwen2.5-7b-instruct-q4_k_m.gguf \
  --host 127.0.0.1 --port 8001 -c 8192 -t 20 -ngl 0
curl -s http://127.0.0.1:8001/v1/models
```

嵌入模型单独起一个实例（bge-m3 是 1024 维，`--pooling cls` 必须对）：

```bash
/opt/llama/llama-server -m /opt/models/bge-m3-q8_0.gguf \
  --host 127.0.0.1 --port 8002 --embeddings --pooling cls -c 512 -t 10
curl -s http://127.0.0.1:8002/v1/embeddings \
  -H 'Content-Type: application/json' \
  -d '{"model":"bge-m3","input":"瓦斯浓度超限"}' | head -c 200
```

## 4. 配置服务

```bash
sudo useradd -r -s /usr/sbin/nologin coalrag || true
sudo mkdir -p /opt/coalrag/{bin,storage} /etc/coalrag
sudo cp /tmp/coalrag-src/deploy/linux/coalrag.env.example /etc/coalrag/coalrag.env
sudo nano /etc/coalrag/coalrag.env      # 改 DSN、JWT_SECRET、模型名
sudo chmod 600 /etc/coalrag/coalrag.env
```

**`COALRAG_MIN_SCORE` 必须重标**：默认 0.08 是给桩嵌入用的。换成 bge-m3 后，
相关条款的余弦相似度通常在 0.5–0.7，无关的在 0.2–0.4，起点取 `0.35`，
再用第 6 步的评测集调到“该答的答、该兜底的兜底”。

## 5. 首次启动

```bash
sudo install -m 0755 /tmp/coalrag-src/deploy/linux/systemd/*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now llama-chat llama-embed coalrag
journalctl -u coalrag -f
```

首启（`COALRAG_BOOTSTRAP=true`）会自动建表并写入演示账号。登录后台改掉演示口令，
再上传真实的《煤矿安全规程》并发布。

## 6. 验收（同一台机器）

```bash
cd /opt/coalrag
./bin/smoke -base http://127.0.0.1:8080 -doc /opt/coalrag/testdata/煤矿安全规程节选.docx
# 期望：103 项全过；问答/检索评测数字会被真实模型改变，看报告而不是只看通过

# 检索与问答评测（20/30 题）
curl -s -X POST http://127.0.0.1:8080/api/v1/eval/retrieval \
  -H "Authorization: Bearer <token>" -H 'Content-Type: application/json' \
  -d @testdata/eval/retrieval_20.json
```

评测报告在 `/opt/coalrag/storage/eval/`。准确率低于 0.9 时按顺序调：
`COALRAG_MIN_SCORE` → `COALRAG_TOP_K` → 换更大模型。

## 7. 备份与探活

```bash
sudo install -m 0755 /tmp/coalrag-src/deploy/linux/*.sh /opt/coalrag/scripts/
sudo /opt/coalrag/scripts/backup.sh              # 手动跑一次
sudo systemctl enable --now coalrag-backup.timer # 每天 02:30
sudo systemctl enable --now coalrag-watch        # 每 60 秒探活
```

## 8. 性能基线

```bash
./bin/loadtest -users 5 -duration 30s -mix ask,search
```

CPU 跑 7B 时关注 **问答 p95**：这一项由模型决定，不是 Go 服务；检索 p95 应在 50ms 内。
把数字记进交付文档，别拿桩模型的 1000 QPS 当真实容量。

## 常见问题

| 现象 | 原因 |
| --- | --- |
| `向量维度不一致，拒绝启动` | `COALRAG_EMBED_DIM` 与模型不符（bge-m3 = 1024） |
| 答案都走兜底 | `min_score` 太高，或嵌入实例没起 |
| 答案很慢 | 模型太大 / 没用 GPU / `max_tokens` 太高 |
| `unsupported architecture` | llama.cpp 版本太旧，换新 release |
| 启动即退出 | 看 `journalctl -u coalrag -n 50`，多为 DSN 或迁移失败 |
