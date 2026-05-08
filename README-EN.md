<p align="center">
  <img src="logo.jpg" width="160" alt="Lingxi Logo" />
</p>

<h1 align="center">Lingxi AI Agent</h1>

<p align="center">
  <strong>🧠 Local-first · 🔌 Multi-model · 🤖 Multi-agent · 🌐 Agent Collaboration · 📦 Zero Config</strong>
</p>

<p align="center">
  <em>Not just a chatbot — a full-featured desktop AI Agent workbench.</em><br/>
  <sub>Build specialized agents, equip them with skills & knowledge, design visual workflows, enable Agent-to-Agent conversations — all running locally on your machine.</sub>
</p>

<p align="center">
  <a href="README.md">中文</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-why-lingxi">Why Lingxi</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-feature-overview">Features</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-screenshot-gallery">Screenshots</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-quick-start">Quick Start</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-architecture">Architecture</a>&nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="#-license">License</a>
</p>

<br/>

<p align="center">
  <img src="images/首页.png" alt="Lingxi Workbench" width="880" />
</p>

<p align="center">
  <sub>▲ Lingxi Workbench — clean conversation UI with session navigation and top-bar feature access</sub>
</p>

---

## 🌟 Why Lingxi

<table>
<tr>
<td width="50%">

### 🔒 Your Data Stays Yours
Conversations, configs, and API keys live in a **local SQLite** database. Keys are encrypted with OS-level security (macOS Keychain / Windows DPAPI). **Zero cloud dependency** — works offline with local models.

</td>
<td width="50%">

### 🔌 14+ Model Providers
Anthropic · OpenAI · DeepSeek · Qwen · Gemini · Doubao · GLM · Kimi · Groq · Ollama … Switch freely, never locked into a single vendor. Built-in Bridge layer auto-translates protocols.

</td>
</tr>
<tr>
<td>

### 🤖 Beyond Chat — An Agent Workbench
Create specialized agents with custom roles, skills, knowledge bases, and MCP tools. Let AI not just answer questions, but **actually get work done** — write code, query data, read docs, operate web pages.

</td>
<td>

### 🌐 Agent-to-Agent Conversations (Project Nexus)
Agents across Lingxi instances on the same LAN **auto-discover, connect, and stream bidirectionally**. Your code reviewer can discuss architecture with a colleague's architect agent — humans oversee anytime.

</td>
</tr>
<tr>
<td>

### 📦 Double-Click to Run, Zero Config
Download `.dmg` on macOS and you're set. Bundles Go backend + Node.js runtime + whisper.cpp — **no Python, Docker, or backend setup required**. All dependencies self-contained.

</td>
<td>

### 🔄 Auto-Update + 6 Beautiful Themes
Built-in OTA incremental updates — new versions download silently and install in one click. Ships with **Light / Dark / Midnight / Cyber / Aurora / Cosmos** themes with smooth Framer Motion transitions.

</td>
</tr>
</table>

---

## 🚀 Feature Overview

Lingxi is not another Chat Wrapper. It's a fully-featured **AI Agent ecosystem workbench**, with every module carefully crafted.

---

### 🏭 Agent Factory — Your AI Team Manager

> Not just swapping a System Prompt. Each agent is a complete configuration entity with **8 dimensions of customization**.

<p align="center">
  <img src="images/智能体工厂.png" alt="Agent Factory" width="880" />
</p>

<table>
<tr>
<td>🎭 <strong>Identity & Role</strong></td>
<td>Name, avatar (26 curated emojis), description, full System Prompt customization</td>
</tr>
<tr>
<td>🧩 <strong>Capability Equipment</strong></td>
<td>Independently bind skills, knowledge bases, MCP tools — code reviewer won't accidentally use email skill</td>
</tr>
<tr>
<td>🎛️ <strong>Parameter Tuning</strong></td>
<td>temperature (precise 0.1 ↔ creative 0.8), max_tokens — independent controls</td>
</tr>
<tr>
<td>🌐 <strong>External Collaboration</strong></td>
<td>Public toggle, capability tags, authorization levels, forbidden info — security boundaries for Nexus</td>
</tr>
<tr>
<td>📋 <strong>17 Built-in Templates</strong></td>
<td>Covering business, development, creative, and productivity — plus a 5-step wizard for custom creation</td>
</tr>
</table>

<details>
<summary>📦 <strong>Built-in Templates (click to expand)</strong></summary>
<br/>

| Category | Templates |
|----------|-----------|
| 🏢 **Business** | Sales Assistant · Business Analyst · HR · Legal Advisor |
| 💻 **Development** | Code Reviewer · Architect · DevOps Expert · Security Engineer · DBA |
| ✍️ **Creative** | Content Creator · Copywriter · Translation Expert · Academic Writer |
| 🌈 **Productivity** | Product Manager · Fitness Coach · Financial Advisor · Travel Planner |

</details>

<p align="center">
  <img src="images/智能体角色设定.png" alt="Role Setup" width="880" />
</p>
<p align="center"><sub>▲ 5-Step Creation Wizard — Role setup with rich System Prompt editing and quick templates</sub></p>

---

### 💬 Premium Conversation Experience — Every Detail Built for Productivity

> Streaming output isn't just showing text character by character — it's **thinking blocks + tool blocks + text blocks** rendered with precision.

<p align="center">
  <img src="images/普通对话.png" alt="Chat Experience" width="880" />
</p>

| Capability | Description |
|------------|-------------|
| ⚡ **Streaming + Chain of Thought** | Real-time token-by-token output, collapsible thinking process, OpenAI reasoning passthrough |
| 🎨 **Code Highlighting** | 50+ languages (prism-react-renderer), one-click copy for every code block |
| 🖼️ **Multimodal Input** | Image paste (Cmd+V) · file drag-and-drop (60+ formats) · offline voice · screenshot (⌘⇧S) |
| 📚 **RAG Citation Visualization** | Inline `[N]` superscripts from knowledge base, hover cards, collapsible reference list |
| 🔍 **Search & Commands** | ⌘K full-text search · `/` slash commands (12 quick prompts) · message edit & resend |
| 🗺️ **Two-Phase Planning** | Complex tasks: collect requirements across dimensions first, then execute after confirmation |
| 💡 **Smart Reply Suggestions** | 2-3 follow-up question capsules after every AI response |
| 📌 **Message Management** | Pin · feedback (👍👎) · session pinning · batch delete · Markdown export |
| 🔊 **TTS Read Aloud** | Web Speech API, one-click read for assistant messages, auto-detects Chinese/English |
| ⏹️ **Abort Generation** | Stop AI response anytime, partial content preserved |

<p align="center">
  <img src="images/规划推理.png" alt="Planning & Reasoning" width="880" />
</p>
<p align="center"><sub>▲ Two-Phase Planning — Collect requirement dimensions first, execute after confirmation</sub></p>

---

### 🎤 Offline Voice Input — Built-in whisper.cpp, No Internet Needed

Lingxi bundles **whisper.cpp** (Apple Metal accelerated). Click mic → record → stop → local recognition → text fills the input box.

**Fully offline, zero latency, no cloud API required.** Falls back to remote Whisper API when preferred.

---

### 🔗 14+ Model Providers, Unified

> One panel to manage all providers — connectivity tests, cost estimation, usage tracking.

<p align="center">
  <img src="images/接入点管理.png" alt="Provider Management" width="880" />
</p>

| Protocol | Providers |
|----------|-----------|
| **Anthropic Native** | Anthropic Official · DashScope (Alibaba Cloud) |
| **OpenAI Compatible** | DeepSeek · Qwen · Doubao · GLM · Kimi · Gemini · OpenRouter · Groq · SiliconFlow · Ollama · OpenAI |

<details>
<summary>🔧 <strong>How the Bridge Layer Works (click to expand)</strong></summary>
<br/>

Lingxi's AI engine uses the Anthropic protocol. When users pick an OpenAI-compatible provider, a local Bridge process auto-starts to do **bidirectional real-time protocol translation**:

```
Claude Code CLI ──Anthropic──► Bridge (127.0.0.1) ──OpenAI──► DeepSeek / Qwen / ...
```

Prefers LiteLLM (Python), falls back to llm-bridge (Node.js). Transparent to the user — switching providers is one click in settings.

</details>

---

### 🧩 Skills · Knowledge Base · MCP — The Agent Capability Trifecta

<table>
<tr>
<td width="33%" valign="top">

#### ⚡ Skills System
- 🤖 **AI Auto-Generation**: Describe needs, stream-generate code
- 📦 **ZIP Import** / Batch upload
- 🛒 **Smithery Marketplace** one-click install
- ✏️ Online edit / export installed skills
- 🔗 Per-agent independent binding

</td>
<td width="33%" valign="top">

#### 📚 Knowledge Base
- 📄 `.md` `.txt` `.csv` `.json` `.pdf` `.docx` support
- 📂 Three-category management (Docs / QA / Data)
- 🖱️ Drag-and-drop batch upload + preview
- 🔍 Auto-generated INDEX
- 📎 RAG retrieval + citation visualization

</td>
<td width="33%" valign="top">

#### 🔧 MCP Tools
- 📡 stdio / SSE / HTTP — all three protocols
- 🌐 Built-in Playwright MCP (auto-detects Chrome)
- 📋 One-click config export (Claude Desktop compatible)
- 🔌 Extend agent capabilities beyond chat
- 🤖 Let agents browse web, access filesystem…

</td>
</tr>
</table>

<p align="center">
  <img src="images/skill管理.png" alt="Skills Management" width="880" />
</p>
<p align="center"><sub>▲ Skills Management — AI generation / marketplace install / online edit / ZIP import</sub></p>

<p align="center">
  <img src="images/知识库.png" alt="Knowledge Base" width="880" />
</p>
<p align="center"><sub>▲ Knowledge Base — drag-and-drop upload, categorized, agents retrieve and cite</sub></p>

<p align="center">
  <img src="images/mcp.png" alt="MCP" width="880" />
</p>
<p align="center"><sub>▲ MCP Tool Management — all three protocols supported, one-click config export</sub></p>

---

### 🔀 Visual Workflow Designer

> Drag nodes, draw connections, build Agent execution flows — no code required.

<p align="center">
  <img src="images/工作流编排首页.png" alt="Workflow Designer" width="880" />
</p>

| Node | Description |
|------|-------------|
| 💬 **Prompt** | Send a prompt to AI, get a response |
| 🔀 **Condition** | Branch based on previous step's output |
| 🔄 **Loop** | Repeat a group of nodes N times |
| ⏱️ **Delay** | Wait for a specified duration |
| 💻 **Code** | Run custom Bash / Python scripts |
| 📤 **Output** | Final result output |

---

### 🌐 Project Nexus — Agent-to-Agent Conversation Network

> A Lingxi original. Let agents across different Lingxi instances **auto-discover, connect, and stream bidirectionally** — with the same immersive experience as the main chat.

<p align="center">
  <img src="images/Agent Nexus网络.png" alt="Agent Nexus Network" width="880" />
</p>

```
┌──────────────┐                        ┌──────────────┐
│  Instance A   │  ◄── Bidirectional ──►  │  Instance B   │
│  🧑 Human A  │     mDNS Discovery      │  🧑 Human B  │
│  🤖 Reviewer │     PSK Key Auth        │  🤖 Architect │
│  (observe)    │     Token-level Stream  │  (observe)    │
└──────────────┘                        └──────────────┘
```

| Capability | Description |
|------------|-------------|
| 🔍 **LAN Auto-Discovery** | mDNS broadcasts `_lingxi._tcp`, instances visible within 10 seconds |
| 🤝 **One-Click Connect** | PSK shared secret verification, subsequent comms token-encrypted |
| ⚡ **Bidirectional Streaming** | Both agents stream token-by-token in real-time, thinking process synced |
| 🎨 **Clear Identity** | Distinct colored avatars + labels — instantly tell local vs. remote agent |
| 🧠 **Persistent Context** | Each conversation maps to an isolated session, memory persists across rounds |
| 👁️ **Dual-Side Observation** | Both parties watch agents think and respond live |
| ✋ **Human Oversight** | Pause · takeover · terminate · handoff auto-notification |
| ✅ **Approval Workflow** | Auto-generated summary after completion, human approval before finalization |
| 📝 **Full Rendering** | Code highlighting, tables, lists, thinking blocks — same UI as main chat |

<p align="center">
  <img src="images/Agent与Agent对话.png" alt="Agent-to-Agent Conversation" width="880" />
</p>
<p align="center"><sub>▲ Agent-to-Agent Conversation — purple = remote agent, theme color = local agent, both streaming live</sub></p>

---

### ⏰ Scheduled Tasks — Let Agents Work 24/7

> Hourly email checks, daily reports, weekly data cleanup — agents work tirelessly for you.

<p align="center">
  <img src="images/定时任务.png" alt="Scheduled Tasks" width="880" />
</p>

| Schedule | Example |
|----------|---------|
| Every N min / hour | Check emails every 30 minutes |
| Daily / Weekly / Monthly | Daily report at 9:00, weekly summary on Monday |
| Custom Cron | `0 */2 * * 1-5` (weekdays every 2 hours) |

- ✅ **Stateful Mode**: Agent remembers previous runs, reports incremental changes only
- 🔔 **Desktop Notifications**: macOS / Windows system-level notification on completion
- 📋 **Execution History**: View past runs + jump to the corresponding session

---

### 💬 Enterprise IM Integration — Team-Level AI Automation

> Connect agent capabilities to enterprise chat tools — @Agent in a group and get automatic replies.

<p align="center">
  <img src="images/IM.png" alt="IM Integration" width="880" />
</p>

| Platform | Integration |
|----------|-------------|
| 🟢 **WeChat Work** | Custom bot webhook |
| 🔵 **DingTalk** | Custom bot webhook |
| 🟣 **Feishu (Lark)** | Custom bot webhook |

Lingxi handles signature verification, message parsing, and format conversion — just configure App ID and secret.

---

### 🧠 Long-Term Memory — Agents That Actually Remember You

Cross-session persistent memory system, **managed per-agent in isolation**.

- 🤖 **Auto Memory**: AI detects important info during conversations (preferences, habits, key facts)
- ✍️ **Manual Entry**: Add memories in Settings > Long-Term Memory
- 🗂️ **Categorized**: View · add · delete · filter by category · clear all
- 🔒 **Isolated & Secure**: Each agent accesses only its own memories

---

### 🎨 6 Themes — Immersive Visual Experience

| Light | Dark | Midnight |
|:-----:|:----:|:--------:|
| Fresh & bright · Purple accent | Classic dark · Easy on eyes | Deep black · Pure minimal |

| Cyber | Aurora | Cosmos |
|:-----:|:------:|:------:|
| Cyberpunk · Cyan-pink neon | Northern lights · Green-teal gradient | Space · Purple-pink nebula |

- 🎭 **CSS Variable Driven**: Zero-flicker theme switching, pure CSS behavior
- 🌊 **Framer Motion**: AnimatePresence page transitions, silky smooth
- 🎯 **Aurora Backgrounds**: Triple-layer radial gradient overlays for immersive atmosphere

---

### 📊 Usage Analytics & Budget Alerts

<p align="center">
  <img src="images/用量计费.png" alt="Usage Analytics" width="880" />
</p>

- 📈 **Precise Billing**: Token-level accuracy for Anthropic official API
- 📊 **Local Estimation Fallback**: Built-in pricing table for other providers (marked with `~`)
- 🔔 **Budget Alerts**: Set daily/monthly limits, Toast reminder when approaching budget

---

## 📸 Screenshot Gallery

<table>
<tr>
<td><img src="images/智能体交互.png" alt="Agent Interaction" /></td>
<td><img src="images/规划模式.png" alt="Planning Mode" /></td>
</tr>
<tr>
<td align="center"><sub>🤖 Agent Interaction — autonomous task execution</sub></td>
<td align="center"><sub>🗺️ Planning Mode — multi-dimension requirement collection</sub></td>
</tr>
<tr>
<td><img src="images/智能体配置.png" alt="Agent Configuration" /></td>
<td><img src="images/agent ppt创作.png" alt="Agent PPT Creation" /></td>
</tr>
<tr>
<td align="center"><sub>⚙️ Agent Configuration — 8-dimension customization</sub></td>
<td align="center"><sub>📊 Agent PPT Creation — AI actually getting work done</sub></td>
</tr>
<tr>
<td><img src="images/llm.png" alt="Multi-Model Switching" /></td>
<td><img src="images/skill安装.png" alt="Skill Install" /></td>
</tr>
<tr>
<td align="center"><sub>🔗 Multi-Model Switching — 14+ providers</sub></td>
<td align="center"><sub>🛒 Smithery Marketplace — one-click skill install</sub></td>
</tr>
<tr>
<td><img src="images/工作流编排首页.png" alt="Workflow Designer" /></td>
<td><img src="images/Agent Nexus网络.png" alt="Nexus Network" /></td>
</tr>
<tr>
<td align="center"><sub>🔀 Visual Workflow — drag-and-drop node editor</sub></td>
<td align="center"><sub>🌐 Nexus Network — agent auto-discovery & conversation</sub></td>
</tr>
</table>

---

## ⌨️ Keyboard Shortcuts

| Shortcut | Action | Shortcut | Action |
|----------|--------|----------|--------|
| `⌘ K` | Search messages | `⌘ N` | New conversation |
| `⌘ B` | Toggle sidebar | `⌘ ,` | Open settings |
| `⌘ /` | Shortcuts panel | `⌘ ⇧ S` | Screenshot to input |
| `/` | Slash commands | `Esc` | Close modal / panel |
| `Enter` | Send message | `Shift+Enter` | New line |

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Electron 36                          │
│  Desktop shell · Window mgmt · safeStorage · OTA update  │
├──────────────────────────┬──────────────────────────────┤
│     React 19 + Vite 8    │      Go 1.24 + Gin 1.10     │
│  Tailwind CSS 3.4        │   SQLite (pure Go WASM)      │
│  Zustand 5 · Motion 12   │   WebSocket · mDNS           │
│  6 themes · virtual scroll│   70+ APIs · scheduler       │
│  prism syntax highlight   │   IM connectors · Bridge     │
└──────────────────────────┴──────────────────────────────┘
             Bundled runtimes (no install required)
    Node.js · whisper.cpp · Claude CLI · LiteLLM Bridge
```

| Layer | Tech | Purpose |
|-------|------|---------|
| 🖥️ Shell | Electron 36 | Window management · safeStorage · screenshots · auto-update |
| 🎨 Frontend | React 19 + Vite 8 + Tailwind 3.4 | 6 themes · Zustand state · Framer Motion animations |
| ⚙️ Backend | Go 1.24 + Gin + SQLite | 70+ APIs · WebSocket · mDNS · task scheduler |
| 🔊 Voice | whisper.cpp (Metal) | Offline ASR · ggml-base model |
| 🔄 Router | LiteLLM / llm-bridge | Anthropic ↔ OpenAI bidirectional protocol translation |

---

## 📥 Quick Start

### macOS (Apple Silicon)

1. **Download** the `.dmg` installer from [Releases](https://github.com/MT-xjr2/lingxi/releases)
2. **Double-click** to install, drag to Applications
3. If macOS says "cannot be verified":
   ```bash
   xattr -cr "/Applications/灵犀.app"
   ```
4. Configure at least one model API key in **Settings → Providers**
5. Start chatting! ✨

### Build from Source

```bash
# Prerequisites: Node.js ≥ 20.19, Go ≥ 1.24
git clone https://github.com/MT-xjr2/lingxi.git
cd lingxi-agent && ./build-desktop.sh
# Output → dist-electron/mac-arm64/灵犀.app
```

<details>
<summary>🔧 <strong>Development Mode (click to expand)</strong></summary>
<br/>

```bash
# Terminal 1: Frontend (hot reload)
cd frontend-desktop && npm install && npm run dev

# Terminal 2: Go backend
cd backend-desktop && go run .

# Terminal 3: Electron
cd electron && npm install && npm start
```

</details>

---

## 📜 License

[MIT License](LICENSE)

---

<p align="center">
  <br/>
  <img src="logo.jpg" width="48" alt="Lingxi" />
  <br/><br/>
  <strong>Lingxi</strong> — Making AI your work partner, not just a chatbot.<br/>
  <sub>Built with ❤️ by the Lingxi team</sub>
  <br/><br/>
  ⭐ If you find this project valuable, please <a href="https://github.com/MT-xjr2/lingxi">Star us on GitHub</a>!
</p>
