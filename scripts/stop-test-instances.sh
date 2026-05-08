#!/bin/bash
# 一键关闭所有灵犀测试实例

echo "=========================================="
echo "  灵犀 多实例测试关闭器"
echo "=========================================="

# 关闭实例2（开发版, 端口 3099）
echo ""
echo "▶ 关闭实例2 (端口 3099)..."
PID=$(lsof -ti:3099 2>/dev/null)
if [ -n "$PID" ]; then
  kill $PID 2>/dev/null
  echo "  ✓ 实例2 已关闭 (PID: $PID)"
else
  echo "  - 实例2 未在运行"
fi

# 关闭实例1（安装版）
echo ""
echo "▶ 关闭实例1（灵犀.app）..."
if pgrep -f "灵犀" > /dev/null 2>&1; then
  osascript -e 'tell application "灵犀" to quit' 2>/dev/null
  sleep 1
  pkill -f "灵犀" 2>/dev/null
  echo "  ✓ 实例1 已关闭"
else
  echo "  - 实例1 未在运行"
fi

echo ""
echo "=========================================="
echo "  ✓ 所有实例已关闭"
echo "=========================================="
