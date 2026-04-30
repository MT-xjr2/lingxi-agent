<p align="center">
  <img src="logo.jpg" width="180" alt="Lingxi Agent Logo" />
</p>

<h1 align="center">灵犀 AI Agent (Lingxi Agent)</h1>

<p align="center">
  <strong>An intelligent AI desktop agent powered by Claude CLI, with multi-provider support and a modern UI.</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#prerequisites">Prerequisites</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#build--package">Build</a> •
  <a href="#screenshots">Screenshots</a> •
  <a href="#license">License</a>
</p>

---

## Features

- **Multi-Model Support** — Seamlessly switch between Anthropic, DashScope, DeepSeek, or any Anthropic-compatible API provider.
- **Secure Key Management** — API keys encrypted via macOS Keychain (`safeStorage`); plaintext never touches disk.
- **Real-time Streaming** — WebSocket-based chat with live token-by-token streaming, thinking process, and tool invocation visualization.
- **Usage Analytics** — Per-message token/cost tracking, daily/model aggregation charts, and upstream account quota queries.
- **Skills & Knowledge Base** — Create, manage, and invoke custom AI skills; attach local knowledge bases for RAG-like context.
- **IM Integration** — Connect to WeChat Work (企业微信) and DingTalk (钉钉) for automated AI-powered messaging.
- **Modern UI** — Clean, responsive interface with light/dark/midnight themes, inspired by leading AI assistants.
- **Fully Local** — All data stored locally in SQLite; no external telemetry or tracking.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Electron Shell                        │
│  ┌──────────┐  ┌─────────────┐  ┌────────────────────┐  │
│  │ main.js  │  │ preload.js  │  │   BrowserWindow    │  │
│  │ safeStore│  │ IPC bridge  │  │   (React Frontend) │  │
│  └────┬─────┘  └──────┬──────┘  └────────┬───────────┘  │
│       │               │                  │              │
│       │    ┌──────────────────────────────┘              │
│       │    │ REST API + WebSocket                        │
│       ▼    ▼                                            │
│  ┌─────────────────────────────────────────────┐        │
│  │           Go Backend (Gin + SQLite)          │        │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌───────┐ │        │
│  │  │Sessions│ │Provider│ │ Usage  │ │ Skills│ │        │
│  │  │ & Chat │ │ Mgmt   │ │Tracker │ │ & KB  │ │        │
│  │  └────┬───┘ └────────┘ └────────┘ └───────┘ │        │
│  │       │                                      │        │
│  │       ▼ spawns                               │        │
│  │  ┌──────────────────┐                        │        │
│  │  │  Claude CLI       │                        │        │
│  │  │  (stream-json)    │                        │        │
│  │  └──────────────────┘                        │        │
│  └─────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────┘
```

| Layer | Tech Stack |
|-------|-----------|
| Desktop Shell | Electron 36 |
| Frontend | React 19, Vite 8, Tailwind CSS, Zustand, Recharts, Framer Motion |
| Backend | Go 1.24, Gin, Gorilla WebSocket, SQLite |
| AI Engine | Claude CLI (`@anthropic-ai/claude-code`) |

## Prerequisites

| Dependency | Version | Notes |
|-----------|---------|-------|
| **Node.js** | ≥ 20.19 or ≥ 22.12 | Required by Vite 8 |
| **Go** | ≥ 1.24 | For backend compilation |
| **Claude CLI** | latest | `npm install -g @anthropic-ai/claude-code` |
| **macOS** | arm64 (Apple Silicon) | Current build target |

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/MT-xjr2/lingxi-agent.git
cd lingxi-agent
```

### 2. Configure AI credentials

```bash
cp ai-config/auth.json.example ai-config/auth.json
```

Edit `ai-config/auth.json` with your API credentials:

```json
{
  "ANTHROPIC_AUTH_TOKEN": "sk-your-api-key-here",
  "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
  "ANTHROPIC_MODEL": "claude-opus-4-5"
}
```

> **Tip:** For DashScope or DeepSeek, set `ANTHROPIC_BASE_URL` to the provider's Anthropic-compatible endpoint and use their API key.

### 3. Build & run

```bash
chmod +x build-desktop.sh
./build-desktop.sh
```

The packaged app will be in `dist-electron/mac-arm64/灵犀.app`.

### 4. Development mode

Run each component separately for hot-reload development:

```bash
# Terminal 1 — Go backend
cd backend-desktop
go run . --port 23343 --frontend-dist ../frontend-desktop/dist

# Terminal 2 — Frontend dev server
cd frontend-desktop
npm install
npm run dev

# Terminal 3 — Electron
cd electron
npm install
npm start
```

## Configuration

### API Profiles (In-App)

After launching, go to **Settings → Models & Endpoints** to:

1. Create API profiles for any Anthropic-compatible provider
2. Enter API endpoint, model name, and API key
3. Switch between profiles on the fly
4. Test connectivity before activating

### Supported Providers

| Provider | Protocol | Usage Query | Notes |
|---------|----------|-------------|-------|
| Anthropic Official | Anthropic | ❌ | Direct API access |
| DashScope | Anthropic Compatible | ✅ | Alibaba Cloud Model Studio |
| DeepSeek | Anthropic Compatible | ✅ | Balance query supported |
| Custom | Anthropic Compatible | ⚙️ | Configure `usage_api_meta` |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BACKEND_PORT` | Go backend HTTP port | `23343` |
| `ANTHROPIC_AUTH_TOKEN` | API key (from auth.json or in-app) | — |
| `ANTHROPIC_BASE_URL` | API endpoint URL | `https://api.anthropic.com` |
| `ANTHROPIC_MODEL` | Default model name | `claude-opus-4-5` |

### Themes

Three built-in themes available in **Settings → Appearance**:

- **Light** — Clean white background
- **Dark** — Soft dark with blue accents
- **Midnight** — Deep OLED-friendly dark

## Project Structure

```
lingxi-agent/
├── backend-desktop/       # Go backend server
│   ├── main.go            # Entry point, Gin router
│   ├── handler/           # HTTP & WebSocket handlers
│   │   ├── chat.go        # Claude CLI interaction & streaming
│   │   ├── session.go     # Session CRUD
│   │   ├── provider.go    # API profile management
│   │   ├── usage.go       # Usage statistics endpoints
│   │   ├── skill.go       # Skill management
│   │   ├── knowledge.go   # Knowledge base
│   │   └── im_connector.go# IM integration
│   ├── db/                # SQLite schema & queries
│   ├── model/             # Data structures
│   ├── config/            # Configuration loading
│   ├── connector/         # IM connector dispatchers
│   └── usage/             # Upstream quota adapters
├── frontend-desktop/      # React frontend
│   ├── src/
│   │   ├── main.jsx       # App entry point
│   │   ├── ui/            # Shared components (AppShell, Sidebar, etc.)
│   │   ├── chat/          # Chat components (Bubble, Composer, etc.)
│   │   ├── settings/      # Settings pages (Profiles, Usage, Appearance)
│   │   ├── state/         # Zustand store
│   │   └── api/           # API client & WebSocket
│   ├── index.html
│   ├── tailwind.config.js
│   └── vite.config.js
├── electron/              # Electron wrapper
│   ├── main.js            # Main process (window, backend spawn, IPC)
│   ├── preload.js         # Context bridge (safeStorage, etc.)
│   ├── package.json       # Electron builder config
│   └── assets/            # Icons, entitlements
├── ai-config/             # AI engine configuration (template)
│   ├── auth.json.example  # Credentials template (copy to auth.json)
│   ├── settings.json      # Claude CLI permissions
│   └── claude.json        # Claude CLI state
├── build-desktop.sh       # One-click build script
├── logo.jpg               # Application logo
└── README.md
```

## Build & Package

### One-Click Build

```bash
./build-desktop.sh
```

This script performs 5 steps:

1. **Compile Go backend** → `backend-desktop/smart-agent`
2. **Build frontend** → `frontend-desktop/dist/`
3. **Bundle AI engine** — Copies system Claude CLI into the app
4. **Embed Node.js runtime** — For running Claude CLI without system Node
5. **Package Electron app** → `dist-electron/mac-arm64/灵犀.app`

### Manual Build

```bash
# Backend
cd backend-desktop
GOOS=darwin GOARCH=arm64 go build -o smart-agent .

# Frontend
cd frontend-desktop
npm install && npm run build

# Electron
cd electron
npm install && npm run dist:mac
```

### Code Signing (Optional)

For distribution outside the Mac App Store, configure your Developer ID:

```bash
export CSC_NAME="Developer ID Application: Your Name"
cd electron && npm run dist:mac
```

Without code signing, users need to run:
```bash
xattr -cr /Applications/灵犀.app
```

## Security

- **API keys** are encrypted using macOS `safeStorage` (backed by Keychain) before being stored in SQLite. The plaintext key only exists in-memory during runtime.
- **No telemetry** — The app does not phone home or collect any analytics data.
- **auth.json** is in `.gitignore` and never committed to the repository.
- All communication between frontend and backend happens over localhost.

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<h1 align="center">灵犀 AI Agent — 中文文档</h1>

## 功能特性

- **多模型切换** — 一键切换 Anthropic 官方、DashScope（通义）、DeepSeek 或其他兼容 Anthropic 协议的供应商。
- **安全密钥管理** — API 密钥通过 macOS 钥匙串（`safeStorage`）加密存储；明文永不落盘。
- **实时流式对话** — 基于 WebSocket 的逐字符流式输出，可视化展示"深度思考"过程和"技能调用"。
- **用量统计** — 每条消息显示 Token 用量/费用，支持按日/按模型聚合图表和上游账户额度查询。
- **技能与知识库** — 创建和管理自定义 AI 技能；上传本地知识库实现 RAG 增强。
- **IM 集成** — 对接企业微信、钉钉，实现 AI 自动回复。
- **现代界面** — 简洁美观的 UI，支持浅色/深色/午夜三种主题。
- **完全本地** — 所有数据存储在本地 SQLite，无任何外部追踪。

## 环境要求

| 依赖 | 版本 | 说明 |
|------|------|------|
| **Node.js** | ≥ 20.19 或 ≥ 22.12 | Vite 8 要求 |
| **Go** | ≥ 1.24 | 后端编译 |
| **Claude CLI** | 最新版 | `npm install -g @anthropic-ai/claude-code` |
| **macOS** | arm64（Apple Silicon） | 当前构建目标 |

## 快速开始

### 1. 克隆仓库

```bash
git clone https://github.com/MT-xjr2/lingxi-agent.git
cd lingxi-agent
```

### 2. 配置 AI 凭据

```bash
cp ai-config/auth.json.example ai-config/auth.json
```

编辑 `ai-config/auth.json`，填入你的 API 凭据：

```json
{
  "ANTHROPIC_AUTH_TOKEN": "sk-你的API密钥",
  "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
  "ANTHROPIC_MODEL": "claude-opus-4-5"
}
```

> **提示：** 如使用 DashScope 或 DeepSeek，将 `ANTHROPIC_BASE_URL` 设为该供应商的兼容端点，并填入对应 API Key。

### 3. 一键构建

```bash
chmod +x build-desktop.sh
./build-desktop.sh
```

打包后的应用位于 `dist-electron/mac-arm64/灵犀.app`。

### 4. 开发模式

分别启动各组件，支持热更新开发：

```bash
# 终端 1 — Go 后端
cd backend-desktop
go run . --port 23343 --frontend-dist ../frontend-desktop/dist

# 终端 2 — 前端开发服务器
cd frontend-desktop
npm install
npm run dev

# 终端 3 — Electron
cd electron
npm install
npm start
```

## 应用内配置

启动后进入 **设置 → 模型与接入点**：

1. 添加 API 接入点（供应商、端点、模型、密钥）
2. 随时切换不同的接入点
3. 测试连通性后再激活使用

### 支持的供应商

| 供应商 | 协议 | 额度查询 | 说明 |
|--------|------|---------|------|
| Anthropic 官方 | Anthropic | ❌ | 直接 API 访问 |
| DashScope | Anthropic 兼容 | ✅ | 阿里云模型服务 |
| DeepSeek | Anthropic 兼容 | ✅ | 支持余额查询 |
| 自定义 | Anthropic 兼容 | ⚙️ | 需配置 `usage_api_meta` |

## 安全说明

- **API 密钥** 通过 macOS `safeStorage`（底层为钥匙串）加密后存入 SQLite，明文仅存在于运行时内存。
- **无遥测** — 应用不会向外部发送任何数据或统计信息。
- **auth.json** 已在 `.gitignore` 中，永远不会被提交到仓库。
- 前后端通信全部经由 localhost 完成。

## 开源协议

MIT License — 详见 [LICENSE](LICENSE)。
