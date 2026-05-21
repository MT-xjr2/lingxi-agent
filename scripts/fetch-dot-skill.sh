#!/bin/bash
# 拉取 dot-skill（colleague-skill 仓库 dot-skill 分支）到 ai-config/skills，供打包内置
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/ai-config/skills/dot-skill"
mkdir -p "$(dirname "$DEST")"
if [ -d "$DEST/.git" ]; then
  echo "  更新 dot-skill..."
  git -C "$DEST" fetch --depth 1 origin dot-skill
  git -C "$DEST" checkout dot-skill
  git -C "$DEST" reset --hard origin/dot-skill
else
  echo "  克隆 dot-skill..."
  rm -rf "$DEST"
  git clone --depth 1 -b dot-skill https://github.com/titanwings/colleague-skill.git "$DEST"
fi
echo "  ✓ dot-skill @ $(git -C "$DEST" rev-parse --short HEAD)"
