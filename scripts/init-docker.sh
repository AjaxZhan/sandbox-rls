#!/bin/bash
set -e

# AgentFense - Docker Runtime 初始化脚本
# 此脚本会拉取常用的 Docker 镜像以提升首次使用体验

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║         AgentFense - Docker Runtime 初始化                       ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# 检查 Docker 是否可用
if ! command -v docker &> /dev/null; then
    echo "❌ 错误: 未找到 Docker 命令"
    echo "   请先安装 Docker: https://docs.docker.com/get-docker/"
    exit 1
fi

# 检查 Docker 守护进程是否运行
if ! docker info &> /dev/null; then
    echo "❌ 错误: Docker 守护进程未运行"
    echo "   请启动 Docker Desktop 或 Docker 服务"
    exit 1
fi

echo "✅ Docker 已安装并运行"
echo ""

# 定义常用镜像列表
declare -A IMAGES=(
    ["alpine:latest"]="轻量级 Linux 环境（默认镜像）"
    ["ubuntu:22.04"]="Ubuntu 22.04 LTS"
    ["python:3.11-alpine"]="Python 3.11（Alpine 精简版）"
    ["python:3.11-slim"]="Python 3.11（Debian 精简版）"
    ["node:20-alpine"]="Node.js 20（Alpine 精简版）"
)

# 询问用户是否拉取所有镜像
echo "📦 可用的镜像列表:"
echo ""
for image in "${!IMAGES[@]}"; do
    desc="${IMAGES[$image]}"
    # 检查镜像是否已存在
    if docker image inspect "$image" &> /dev/null; then
        echo "   ✓ $image - $desc [已存在]"
    else
        echo "   ○ $image - $desc [未下载]"
    fi
done
echo ""

# 默认拉取模式
PULL_MODE="${1:-interactive}"

if [ "$PULL_MODE" = "all" ]; then
    # 自动拉取所有镜像
    echo "🚀 拉取所有镜像..."
    for image in "${!IMAGES[@]}"; do
        if ! docker image inspect "$image" &> /dev/null; then
            echo ""
            echo "📥 拉取 $image..."
            docker pull "$image"
        fi
    done
elif [ "$PULL_MODE" = "minimal" ]; then
    # 只拉取默认镜像
    echo "🚀 拉取默认镜像..."
    docker pull alpine:latest
else
    # 交互式选择
    echo "请选择拉取模式:"
    echo "  1) all      - 拉取所有常用镜像"
    echo "  2) minimal  - 只拉取 alpine:latest（最小集）"
    echo "  3) skip     - 跳过"
    echo ""
    read -r -p "输入 [all/minimal/skip] (默认 all): " REPLY
    REPLY="${REPLY:-all}"

    if [[ "$REPLY" =~ ^([Aa]ll|1)$ ]]; then
        echo "🚀 拉取所有镜像..."
        for image in "${!IMAGES[@]}"; do
            if ! docker image inspect "$image" &> /dev/null; then
                echo ""
                echo "📥 拉取 $image..."
                docker pull "$image"
            fi
        done
    elif [[ "$REPLY" =~ ^([Mm]inimal|2)$ ]]; then
        echo "🚀 拉取默认镜像..."
        docker pull alpine:latest
    else
        echo "⏭️  跳过镜像拉取"
        echo ""
        echo "💡 提示: 你可以稍后手动拉取镜像:"
        echo "   docker pull alpine:latest"
        echo "   docker pull python:3.11-alpine"
    fi
fi

echo ""
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  ✅ 初始化完成                                                   ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""
echo "📝 下一步:"
echo "   1. 启动服务器:"
echo "      ./bin/sandbox-server -config test-config.yaml"
echo ""
echo "   2. 运行测试:"
echo "      cd sdk/python && python test_sdk.py"
echo ""
echo "💡 提示:"
echo "   - 使用 'docker images' 查看已下载的镜像"
echo "   - 在 test-config.yaml 中可以修改默认镜像"
echo ""
