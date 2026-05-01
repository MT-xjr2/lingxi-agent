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

- **Multi-Model Support** — Seamlessly switch between Anthropic-native providers and **any OpenAI-compatible provider** (DeepSeek, Qwen/DashScope, Doubao, GLM, Gemini, OpenRouter, Moonshot, Groq, Ollama, OpenAI, etc.) via a built-in routing layer.
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

**Anthropic-native (direct connection):**

| Provider | Usage Query | Notes |
|---------|-------------|-------|
| Anthropic Official | — | Direct API access |
| DashScope (Anthropic) | ✓ | Alibaba Cloud Model Studio (Anthropic-compat URL) |
| DeepSeek (Anthropic) | ✓ | DeepSeek's Anthropic-compatible endpoint |
| Custom (Anthropic) | manual | Bring your own endpoint |

**OpenAI-compatible (routed through built-in bridge layer):**

| Provider | Default Model | Usage Query |
|---------|---------------|-------------|
| DeepSeek | `deepseek-chat` | ✓ |
| Qwen / DashScope (OpenAI mode) | `qwen3-coder-plus` | ✓ |
| Doubao / Volcengine | configurable | manual |
| GLM / Z.ai | `glm-4.6` | manual |
| Moonshot / Kimi | `kimi-k2-turbo-preview` | manual |
| Google Gemini | `gemini-2.5-pro` | manual |
| OpenRouter | `google/gemini-2.5-pro` | manual |
| Groq | `llama-3.3-70b-versatile` | manual |
| SiliconFlow | `deepseek-ai/DeepSeek-V3` | manual |
| Ollama (local) | `qwen2.5-coder:14b` | — |
| OpenAI Official | `gpt-4o` | manual |
| Custom (OpenAI) | manual | manual |

> When you activate an OpenAI-compatible profile, the app automatically launches a tiny local proxy that uses [supermemoryai/llm-bridge](https://github.com/supermemoryai/llm-bridge) to translate Anthropic protocol ↔ OpenAI protocol in real-time. **Everything stays on your machine** — no third-party gateways involved.

### Routing Layer (llm-bridge)

The bundled routing layer is a ~200-line Node.js HTTP server that imports `llm-bridge` directly. It is automatically managed for you:

```
┌──────────────────┐   Anthropic    ┌──────────┐   OpenAI   ┌──────────────┐
│ Claude CLI       │ ─────────────▶ │  bridge  │ ─────────▶ │ DeepSeek /   │
│ (lingxi backend) │   stream-json  │ (local)  │   stream   │ Qwen / GLM…  │
└──────────────────┘                └──────────┘            └──────────────┘
```

- **Lifecycle**: spawned on demand when an OpenAI profile is activated; killed when switching back to an Anthropic profile. Profile switches reuse the running process (just re-pushes config).
- **Isolation**: state lives only in `~/Library/Application Support/lingxi-agent/bridge-home/`.
- **Configuration**: pushed in-memory via `POST /__config` from the Go backend. No JSON config file on disk.
- **Bundle size**: only ~330KB (vs ~30MB for the previous CCR-based router).
- **Status**: surfaced via the green "路由层 已就绪" pill in the title bar.

If you need to inspect or stop the routing layer manually:

```bash
curl http://127.0.0.1:23343/api/router/status
curl -X POST http://127.0.0.1:23343/api/router/stop
```

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

## Keyboard Shortcuts

| Action | Shortcut |
|--------|----------|
| Send message | `Enter` |
| Insert newline in composer | `Shift + Enter` |
| Paste image (auto-attach) | `⌘ + V` (in composer) |
| Stop streaming response | Click the **Stop** button in the composer |
| New chat | Click **+ 新对话** in sidebar |
| Open / close DevTools | `⌥ + ⌘ + I` |
| Reload window | `⌘ + R` |
| Quit | `⌘ + Q` |

## Screenshots

> Screenshots are intentionally not committed to keep the repo light. Run `./build-desktop.sh && open /Applications/灵犀.app` to see the live UI, which features:
>
> - **Aurora gradient** backdrop layered behind the entire window
> - **Frosted-glass** title bar with gradient brand text
> - **Sidebar** with gradient pill on the active session and a glowing "+ 新对话" button
> - **Message bubbles** with soft depth shadows; user bubbles use a violet-blue gradient with hover-lift
> - **Empty state** centred hero with a pulse-ring halo around the Sparkles icon
> - **Composer** lifts on focus with a violet ring and ambient glow

## Troubleshooting

### The app says "未配置" / shows no models
Open **设置 → 模型与接入点**, click **添加接入点**, fill in the provider, model, endpoint and API key, then click **激活**. Activation also triggers a connectivity test.

### "路由层 错误" pill in the title bar
This appears only when an OpenAI-compatible profile is active and the bridge process failed to start. Click the pill for the error tooltip. Common causes:

- Port `127.0.0.1:<auto>` blocked by another local proxy — try restarting the app.
- Internal Node runtime failed to spawn — re-run `./build-desktop.sh` to refresh `electron/resources/node-bin/`.
- Inspect manually:
  ```bash
  curl http://127.0.0.1:23343/api/router/status
  curl -X POST http://127.0.0.1:23343/api/router/stop
  ```

### macOS says the app is "damaged" or from an unidentified developer
This is normal for unsigned builds. Run once:
```bash
xattr -cr /Applications/灵犀.app
```
Or sign locally — see [Code Signing](#code-signing-optional).

### Build fails with "Vite requires Node.js 20.19+ or 22.12+"
You're on an older Node. Upgrade with `brew install node` or use a Node manager (`mise`, `nvm`, `fnm`, `volta`). The build script will pick up `which node`, so make sure the new one is on `PATH`.

### Where is my data?
- **App database** (sessions, messages, profiles, encrypted keys): `~/Library/Application Support/lingxi-agent/lingxi.db`
- **Bridge runtime state**: `~/Library/Application Support/lingxi-agent/bridge-home/`
- **AI engine config**: `ai-config/` in this repo (only `auth.json.example` is committed)

To reset the app completely: quit it, then `rm -rf ~/Library/Application\ Support/lingxi-agent`.

### Resetting / uninstalling
```bash
pkill -x "灵犀" 2>/dev/null
rm -rf "/Applications/灵犀.app"
rm -rf ~/Library/Application\ Support/lingxi-agent
```

## Changelog

### v1.0.0 — 2026-05
- 🎨 **UI polish**: aurora gradient backdrop, glass title bar, gradient brand text, gradient sidebar active pill, gradient primary buttons with hover-lift, message-entry animation, pulse-ring empty-state hero, refined Markdown styles (block-quote, link underline, code block inset highlight), thinner & cleaner scrollbar, `prefers-reduced-motion` support.
- 🔄 **Routing layer**: replaced `claude-code-router` with [`supermemoryai/llm-bridge`](https://github.com/supermemoryai/llm-bridge); bundle shrunk from ~30 MB to ~330 KB.
- 🐍 **LiteLLM bridge** (Python) added for stricter tool-call compatibility on smaller OpenAI-compatible models.
- 🔐 **Provider lifecycle**: clear in-memory token + stop bridge on profile delete / activation switch.
- 🧠 **Prompt tuning** for smaller OpenAI-compatible models.
- 🎁 Initial public release of the Lingxi AI Agent desktop application.

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<h1 align="center">灵犀 AI Agent — 中文文档</h1>

## 功能特性

- **多模型切换** — 既可直连 Anthropic 协议供应商，也可通过内置路由层接入 **任意 OpenAI 协议供应商**（DeepSeek / 千问 / 豆包 / GLM / Gemini / OpenRouter / Moonshot / Groq / Ollama / OpenAI 等）。
- **安全密钥管理** — API 密钥通过 macOS 钥匙串（`safeStorage`）加密存储；明文永不落盘。
- **实时流式对话** — 基于 WebSocket 的逐字符流式输出，可视化展示"深度思考"过程和"技能调用"。
- **用量统计** — 每条消息显示 Token 用量/费用，支持按日/按模型聚合图表和上游账户额度查询。
- **技能与知识库** — 创建和管理自定义 AI 技能；上传本地知识库实现 RAG 增强。
- **IM 集成** — 对接企业微信、钉钉，实现 AI 自动回复。
- **现代界面** — 极光渐变底图、毛玻璃标题栏、渐变品牌字、消息气泡景深阴影、消息进场动画、空态英雄区光晕脉冲；浅色 / 深色 / 午夜三种主题。
- **完全本地** — 所有数据存储在本地 SQLite，无任何外部追踪。
- **无障碍支持** — 尊重系统 `prefers-reduced-motion` 设置，对比度兼顾浅色与 OLED 风深色主题。

## 键盘快捷键

| 操作 | 快捷键 |
|------|--------|
| 发送消息 | `Enter` |
| 输入框换行 | `Shift + Enter` |
| 粘贴图片自动附加 | `⌘ + V`（输入框聚焦时） |
| 停止流式响应 | 点击输入框右下角的「停止」 |
| 新建对话 | 侧栏「+ 新对话」 |
| 打开 / 关闭 DevTools | `⌥ + ⌘ + I` |
| 重载窗口 | `⌘ + R` |
| 退出 | `⌘ + Q` |

## 常见问题（FAQ）

**Q：标题栏出现「路由层 错误」？**
A：仅在激活 OpenAI 协议接入点且 bridge 进程失败时出现。鼠标悬停查看错误提示；常见原因为本地端口被占用或内嵌 Node 缺失，重新执行 `./build-desktop.sh` 即可恢复。

**Q：macOS 提示"应用已损坏"或"无法验证开发者"？**
A：未签名构建的预期表现，运行一次：
```bash
xattr -cr /Applications/灵犀.app
```

**Q：构建报错 "Vite requires Node.js 20.19+ or 22.12+"？**
A：升级 Node：`brew install node`，或用 `mise` / `nvm` / `fnm` / `volta`。

**Q：数据存放位置？**
- 应用数据库（会话/消息/接入点/加密密钥）：`~/Library/Application Support/lingxi-agent/lingxi.db`
- Bridge 运行时数据：`~/Library/Application Support/lingxi-agent/bridge-home/`
- AI 引擎配置：仓库内的 `ai-config/`（仅 `auth.json.example` 入库）

完全重置：
```bash
pkill -x "灵犀" 2>/dev/null
rm -rf "/Applications/灵犀.app"
rm -rf ~/Library/Application\ Support/lingxi-agent
```

## 更新日志

### v1.0.0 — 2026-05
- 🎨 **视觉与交互升级**：极光渐变底图、毛玻璃标题栏、品牌渐变字、侧栏会话渐变指示条、主按钮渐变 + 悬停上抬、消息进场动画、空态光晕脉冲、Markdown 样式精修、更精致的滚动条、`prefers-reduced-motion` 支持。
- 🔄 **路由层重构**：从 `claude-code-router` 切换到 [`supermemoryai/llm-bridge`](https://github.com/supermemoryai/llm-bridge)，包体从约 30MB 减到约 330KB。
- 🐍 **LiteLLM Bridge**（Python）：为小尺寸 OpenAI 兼容模型提供更严格的工具协议兼容性。
- 🔐 **接入点生命周期**：删除 / 切换接入点时清理运行时密钥并停止 bridge。
- 🧠 **Prompt 调优**：兼容更小的 OpenAI 协议模型。
- 🎁 灵犀桌面端首个公开版本。

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

**Anthropic 协议（直连）：**

| 供应商 | 额度查询 | 说明 |
|--------|---------|------|
| Anthropic 官方 | — | 直接 API |
| DashScope (Anthropic) | ✓ | 阿里云模型服务 Anthropic 兼容端点 |
| DeepSeek (Anthropic) | ✓ | DeepSeek 的 Anthropic 兼容端点 |
| 自定义 (Anthropic) | 手动 | 自带端点 |

**OpenAI 协议（经内置 bridge 路由层）：**

| 供应商 | 默认模型 | 额度查询 |
|--------|---------|---------|
| DeepSeek | `deepseek-chat` | ✓ |
| 千问 / DashScope（OpenAI 模式） | `qwen3-coder-plus` | ✓ |
| 豆包 / 火山方舟 | 自配 | 手动 |
| GLM / 智谱 | `glm-4.6` | 手动 |
| Moonshot / Kimi | `kimi-k2-turbo-preview` | 手动 |
| Google Gemini | `gemini-2.5-pro` | 手动 |
| OpenRouter | `google/gemini-2.5-pro` | 手动 |
| Groq | `llama-3.3-70b-versatile` | 手动 |
| 硅基流动 | `deepseek-ai/DeepSeek-V3` | 手动 |
| Ollama（本地） | `qwen2.5-coder:14b` | — |
| OpenAI 官方 | `gpt-4o` | 手动 |
| 自定义 (OpenAI) | 自配 | 手动 |

> 激活 OpenAI 协议的接入点后，应用会自动在 `127.0.0.1:<随机端口>` 启动一个本地代理（基于 [supermemoryai/llm-bridge](https://github.com/supermemoryai/llm-bridge)），实时把 Anthropic 协议 ↔ OpenAI 协议进行双向翻译。**全程发生在本机**，不经过任何第三方网关。

### 路由层（llm-bridge）

应用内置的路由层是约 200 行的 Node.js HTTP 服务，直接 import `llm-bridge`，全自动托管：

- **生命周期**：激活 OpenAI 协议接入点时自动启动；切回 Anthropic 协议时自动停止；切换 profile 时复用进程（只重推 config）。
- **隔离**：进程数据写入 `~/Library/Application Support/lingxi-agent/bridge-home/`。
- **配置**：通过 `POST /__config` 内存推送，磁盘上不留任何配置文件。
- **包体积**：约 330KB（旧 CCR 方案约 30MB）。
- **状态**：标题栏的绿色「路由层 已就绪」徽章实时反映状态。

排障接口：

```bash
curl http://127.0.0.1:23343/api/router/status
curl -X POST http://127.0.0.1:23343/api/router/stop
```

## 安全说明

- **API 密钥** 通过 macOS `safeStorage`（底层为钥匙串）加密后存入 SQLite，明文仅存在于运行时内存。
- **无遥测** — 应用不会向外部发送任何数据或统计信息。
- **auth.json** 已在 `.gitignore` 中，永远不会被提交到仓库。
- 前后端通信全部经由 localhost 完成。

## 开源协议

MIT License — 详见 [LICENSE](LICENSE)。
