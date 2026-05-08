#!/bin/bash
# 一键启动两个独立灵犀实例用于 Agent Nexus 本地测试
# 实例1: 安装版 (端口 3001)
# 实例2: 开发版 (端口 3099, 独立数据库)

set -e

INSTANCE2_DIR="/tmp/lingxi-instance2"
INSTANCE2_PORT=3099
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=========================================="
echo "  灵犀 多实例测试启动器"
echo "=========================================="

# 创建实例2数据目录
mkdir -p "$INSTANCE2_DIR"

# 启动实例1（安装版）
echo ""
echo "▶ 启动实例1（安装版, 端口 3001）..."
if pgrep -f "灵犀.app" > /dev/null 2>&1; then
  echo "  ✓ 实例1 已在运行"
else
  open "/Applications/灵犀.app" 2>/dev/null || {
    echo "  ✗ 灵犀.app 未安装，请先执行 ./build-desktop.sh 并安装"
    exit 1
  }
  echo "  ✓ 实例1 已启动"
fi

# 等待实例1就绪
echo "  等待实例1就绪..."
for i in $(seq 1 30); do
  if curl -s http://localhost:3001/api/ping > /dev/null 2>&1; then
    echo "  ✓ 实例1 就绪 (端口 3001)"
    break
  fi
  sleep 1
done

# 检查实例2是否已在运行
if curl -s "http://localhost:$INSTANCE2_PORT/api/ping" > /dev/null 2>&1; then
  echo ""
  echo "▶ 实例2 已在运行 (端口 $INSTANCE2_PORT)"
else
  echo ""
  echo "▶ 启动实例2（开发版, 端口 $INSTANCE2_PORT）..."

  # 初始化实例2的 nexus 端口（仅首次）
  if [ -f "$INSTANCE2_DIR/smart-agent.db" ]; then
    sqlite3 "$INSTANCE2_DIR/smart-agent.db" \
      "UPDATE nexus_settings SET listen_port=$INSTANCE2_PORT WHERE id=1 AND listen_port!=3099;" 2>/dev/null
  fi

  # 启动实例2后端
  cd "$PROJECT_ROOT/backend-desktop"
  nohup env \
    PORT=$INSTANCE2_PORT \
    DB_PATH="$INSTANCE2_DIR/smart-agent.db" \
    FRONTEND_DIST="../frontend-desktop/dist" \
    go run . > "$INSTANCE2_DIR/backend.log" 2>&1 &
  echo "  PID: $!"

  # 等待实例2就绪
  echo "  等待实例2就绪..."
  for i in $(seq 1 30); do
    if curl -s "http://localhost:$INSTANCE2_PORT/api/ping" > /dev/null 2>&1; then
      echo "  ✓ 实例2 就绪 (端口 $INSTANCE2_PORT)"
      break
    fi
    sleep 1
  done

  # 设置实例2的端口和昵称
  if [ -f "$INSTANCE2_DIR/smart-agent.db" ]; then
    sqlite3 "$INSTANCE2_DIR/smart-agent.db" \
      "UPDATE nexus_settings SET listen_port=$INSTANCE2_PORT, nickname='实例2' WHERE id=1;"
  fi
fi

echo ""
echo "=========================================="
echo "  ✓ 两个实例均已就绪"
echo ""
echo "  实例1（安装版）: 灵犀桌面 App"
echo "  实例2（开发版）: http://localhost:$INSTANCE2_PORT"
echo ""
echo "  提示: 实例2首次使用需在设置中配置模型接入点"
echo "  关闭: ./scripts/stop-test-instances.sh"
echo "=========================================="
