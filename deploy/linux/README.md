# Ubuntu 离线部署（单机）

目标机器：Ubuntu 24.04 LTS，Xeon Silver 4210R（20 核 40 线程），128 GB 内存，4.5 TB 磁盘，2 x NVIDIA RTX 2080 Ti（11 GB x 2，共 22 GB 显存）。
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

## 3. 模型与 llama.cpp（CUDA 13 注意）

本机驱动 580.105.08 / CUDA 13.0。先确认运行时：

```bash
nvidia-smi | head -4
ldconfig -p | grep -E 'libcudart|libvulkan'
ls -d /usr/local/cuda* 2>/dev/null
nvcc --version 2>/dev/null
```

| 检查结果 | 走哪条路 |
| --- | --- |
| 有 `libcudart.so.12` 或 CUDA 12 toolkit | 直接用 llama.cpp 的预编译 CUDA 包 |
| 只有 CUDA 13、没有 12 | 源码编译（推荐）或 Vulkan 后端，见下 |
| 都没有 | 先 `-ngl 0` 用 CPU 跑通链路，再补 GPU |

**本机已装 CUDA 12.4 + nvcc**，直接用仓库里的脚本编译（约 15–25 分钟）：

`ash
sudo bash deploy/linux/build-llama.sh      # 默认 SRC=/opt/src/llama.cpp PREFIX=/opt/llama ARCH=75
`

离线时把 llama.cpp 源码包放到 /tmp/llama.cpp.tar.gz，脚本自动解压、不联网。

手工编译（需要 cmake、g++、CUDA toolkit 12.4）：

```bash
git clone https://github.com/ggml-org/llama.cpp  # 离线则摆渡源码包
cd llama.cpp
cmake -B build -DGGML_CUDA=ON -DCMAKE_CUDA_ARCHITECTURES=75
cmake --build build -j 20 --target llama-server
sudo install -m 0755 build/bin/llama-server /opt/llama/llama-server
```

`CMAKE_CUDA_ARCHITECTURES=75` 是 2080 Ti 的算力等级，显式指定能省编译时间。

Vulkan 兜底（不需要 CUDA，性能约为 CUDA 的 80%）：

```bash
cmake -B build -DGGML_VULKAN=ON
cmake --build build -j 20 --target llama-server
```

### 模型选择

22 GB 显存，14B 的 Q4/Q5/Q6 都放得下，**推荐 Q5_K_M**（质量接近 Q6，速度比 Q6 快）：

| 量化 | 文件大小 | +8k KV | 预期速度（双卡） |
| --- | --- | --- | --- |
| Q4_K_M | ~9.0 GB | ~10.6 GB | 30–40 tok/s |
| **Q5_K_M** | ~10.5 GB | ~12.1 GB | **25–35 tok/s** |
| Q6_K | ~12.4 GB | ~14.0 GB | 20–28 tok/s |

注意：桌面 GNOME 占 GPU0 约 0.4 GB，分片后每卡仍有 5–7 GB 余量，够 KV 缓存。
显存不够时优先把 `-c` 从 8192 降到 4096，别先降量化。

模型文件（ModelScope / HuggingFace 的 GGUF 仓库）：

- 对话：`qwen2.5-14b-instruct-q5_k_m.gguf`
- 嵌入：`bge-m3-q8_0.gguf`（1024 维）

摆渡到 `/opt/models/` 后校验：

```bash
sha256sum /opt/models/*.gguf
```

### 启动

```bash
/opt/llama/llama-server -m /opt/models/qwen2.5-14b-instruct-q5_k_m.gguf \
  --host 127.0.0.1 --port 8001 -c 8192 -t 8 -ngl 99 --tensor-split 11,11 --jinja
curl -s http://127.0.0.1:8001/v1/models

/opt/llama/llama-server -m /opt/models/bge-m3-q8_0.gguf \
  --host 127.0.0.1 --port 8002 --embeddings --pooling cls -c 512 -t 10
curl -s http://127.0.0.1:8002/v1/embeddings -H 'Content-Type: application/json' \
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
