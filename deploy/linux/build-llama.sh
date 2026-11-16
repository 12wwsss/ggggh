#!/usr/bin/env bash
# 在服务器上编译 llama.cpp（CUDA 12.4，2080 Ti 算力 7.5）
#
#   sudo bash deploy/linux/build-llama.sh
#   SRC=/opt/src/llama.cpp ARCH=75 JOBS=20 bash deploy/linux/build-llama.sh
#
# 离线：把源码包放到 /tmp/llama.cpp.tar.gz，脚本会自动解压，不联网。
set -euo pipefail

SRC="${SRC:-/opt/src/llama.cpp}"
PREFIX="${PREFIX:-/opt/llama}"
ARCH="${ARCH:-75}"          # 2080 Ti = 7.5
JOBS="${JOBS:-20}"

# 非登录 shell（sudo/ssh 命令）不会带上 CUDA 的 PATH，必须显式补；
# 否则 cmake 报 "No CMAKE_CUDA_COMPILER could be found"。
export PATH="/usr/local/cuda/bin:${PATH}"
export CUDACXX="${CUDACXX:-/usr/local/cuda/bin/nvcc}"

echo "[1/4] 检查依赖"
for c in cmake g++ nvcc; do
  if ! command -v "$c" >/dev/null 2>&1; then
    echo "缺少 $c：cmake 用 apt install -y cmake（3.27 即可），g++ 与 nvcc 见 README" >&2
    exit 1
  fi
done
cmake --version | head -1
nvcc --version | tail -2

echo "[2/4] 准备源码：$SRC"
if [ ! -d "$SRC" ]; then
  mkdir -p "$(dirname "$SRC")"
  if [ -f /tmp/llama.cpp.tar.gz ]; then
    tar -xzf /tmp/llama.cpp.tar.gz -C "$(dirname "$SRC")"
    # 压缩包解压后目录名可能是 llama.cpp-master
    if [ ! -d "$SRC" ]; then
      cand="$(find "$(dirname "$SRC")" -maxdepth 1 -type d -name 'llama.cpp*' | head -1)"
      [ -n "$cand" ] && mv "$cand" "$SRC"
    fi
  else
    git clone --depth 1 https://github.com/ggml-org/llama.cpp "$SRC"
  fi
fi
[ -d "$SRC" ] || { echo "源码目录不存在：$SRC" >&2; exit 1; }

echo "[3/4] 编译（CUDA 架构 $ARCH，$JOBS 线程）"
# g++ 13.3 + CUDA 12.4 会触发主机编译器版本检查，加 allow 标志跳过（仅告警层面放宽）
cmake -S "$SRC" -B "$SRC/build" \
  -DGGML_CUDA=ON \
  -DCMAKE_CUDA_ARCHITECTURES="$ARCH" \
  -DLLAMA_CURL=OFF \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_CUDA_FLAGS=-allow-unsupported-compiler
cmake --build "$SRC/build" --config Release -j "$JOBS" --target llama-server llama-cli

echo "[4/4] 安装到 $PREFIX"
mkdir -p "$PREFIX"
install -m 0755 "$SRC/build/bin/llama-server" "$PREFIX/llama-server"
install -m 0755 "$SRC/build/bin/llama-cli" "$PREFIX/llama-cli"

echo "完成。快速验证："
echo "  $PREFIX/llama-server --help | head -5"
echo "  nvidia-smi   # 启动后应看到显存占用"
