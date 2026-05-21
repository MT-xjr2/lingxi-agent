<p align="center">
  <img src="logo.jpg" width="128" alt="Lingxi Logo" />
</p>

<h1 align="center">Lingxi AI Agent</h1>

<p align="center">
  <strong>A local-first desktop AI Agent workbench</strong><br/>
  Multi-model · Multi-agent · Persona distillation · Deep RAG · Screen control · Agent mesh · Group chat · Self-evolution
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Personal%20Use-orange" alt="License" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows-lightgrey" alt="Platform" />
  <img src="https://img.shields.io/badge/Electron-36-47848F?logo=electron&logoColor=white" alt="Electron" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white" alt="React" />
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white" alt="Go" />
</p>

<p align="center">
  <a href="README.md">中文</a> ·
  <a href="#-why-lingxi">Why Lingxi</a> ·
  <a href="#-core-highlights">Highlights</a> ·
  <a href="#-capability-map">Capabilities</a> ·
  <a href="#-feature-deep-dive">Features</a> ·
  <a href="#-quick-start">Quick start</a> ·
  <a href="#-architecture">Architecture</a> ·
  <a href="#-support">Support</a>
</p>

<br/>

---

## 📷 Overview

<!-- 📷 Hero screenshot -->
<p align="center">
  <img src="images/screenshots/01-hero-home.png" alt="Lingxi workbench" width="920" />
</p>
<p align="center"><sub>The Lingxi workbench — chat, agents, and tools in one place</sub></p>

<br/>

---

## 🤔 Why Lingxi

There are plenty of AI products out there — ChatGPT, Claude, and all manner of chat apps. But when you try to use AI as a genuine **work partner** rather than a throwaway Q&A box, you hit a wall:

- **Data lives in the cloud**: conversations, knowledge bases, and API keys are all handed to third parties — zero privacy guarantees.
- **"Custom assistants" are just system prompts**: no real skills, no tools, no memory.
- **Agents can't collaborate**: your code reviewer and your colleague's architect bot live on separate islands forever.
- **No evolution**: you correct the AI a hundred times; next session, it makes the same mistake — because nothing is remembered.
- **Group chat is a round-robin script**: multi-agent scenarios feel mechanical, not human.

**Lingxi** was built to fix all of that. It runs a full Agent stack on your local machine: data and keys stay on-device, every agent has its own skills, knowledge base, and tools, agents can stream conversations across devices in real time, personas are distilled from real chat exports, and agents self-evolve from dialogue.

**In one sentence: Lingxi is not "another chat window" — it's an AI Agent operating system on your desktop.**

<br/>

---

## ✨ Core highlights

<table>
<tr>
<td width="180" align="center"><strong>🔒 Local-first</strong></td>
<td>Sessions, config, vector indices, and evolution logs live in on-device SQLite. API keys use the OS keychain. Offline ASR via built-in whisper.cpp. Your data stays your data.</td>
</tr>
<tr>
<td align="center"><strong>🤖 14+ providers</strong></td>
<td>Anthropic, OpenAI, DeepSeek, Qwen, Gemini, Doubao, GLM, Kimi, Groq, Ollama… The built-in Bridge layer translates protocols transparently — switch models mid-chat without friction.</td>
</tr>
<tr>
<td align="center"><strong>🧠 Real agents</strong></td>
<td>Not a system-prompt swap: each agent has its own skill pack, RAG knowledge base, MCP tools, workflows; can autonomously invoke Bash, file I/O, browsers; supports two-phase planning.</td>
</tr>
<tr>
<td align="center"><strong>👤 Persona distillation</strong></td>
<td>Powered by <a href="https://github.com/titanwings/colleague-skill">dot-skill</a>: upload WeChat logs, PDFs, emails — distill a colleague's, friend's, or celebrity's communication style and personality into an agent. Parallel multi-person runs supported.</td>
</tr>
<tr>
<td align="center"><strong>🧬 Self-evolution</strong></td>
<td>Corrections, thumbs-down, and rich threads are automatically refined into long-term memories, knowledge docs, and skill fixes. Global scanner + per-session triggers; fully auditable and revertible.</td>
</tr>
<tr>
<td align="center"><strong>🌐 Agent mesh</strong></td>
<td>Project Nexus: LAN mDNS + WAN signaling for cross-device auto-discovery and bidirectional token-level streaming. Humans can pause, take over, or approve at any time.</td>
</tr>
<tr>
<td align="center"><strong>👥 WeChat-style group chat</strong></td>
<td>Multiple agents in one room; personality-driven speak probability; @mentions, quotes, images; human-like pacing — not round-robin scripts.</td>
</tr>
<tr>
<td align="center"><strong>🖥️ Screen awareness</strong></td>
<td>Screen Agent sees your screen, plans steps, and operates mouse/keyboard with per-step confirmation. Spotlight global float + smart clipboard suggestions.</td>
</tr>
<tr>
<td align="center"><strong>📦 Zero-ops setup</strong></td>
<td>macOS <code>.dmg</code> / Windows installer bundles Go backend, Node, whisper.cpp, and Claude CLI. No Docker, no self-hosted servers.</td>
</tr>
</table>

<br/>

---

## 🗺️ Capability map

```mermaid
flowchart TB
  subgraph Desktop["🖥️ Electron shell"]
    UI["React 19 · 6 themes · streaming UI"]
    SP["Spotlight · clipboard watcher"]
    SC["Screen Agent desktop control"]
  end

  subgraph Core["⚙️ Go backend · SQLite"]
    Chat["Chat engine · planning · tool chain"]
    Agent["Agent factory · persona distillation"]
    Evo["Self-evolution · global scanner"]
    RAG["Vector index · hybrid search · file watch"]
    Nexus["Project Nexus · group chat · signaling"]
    Job["Scheduler · IM webhooks"]
    Skill["Skills · MCP · workflows"]
  end

  subgraph AI["🧠 AI runtime"]
    CLI["Claude CLI"]
    Bridge["LiteLLM / llm-bridge protocol translation"]
    WH["whisper.cpp offline ASR"]
  end

  UI --> Chat
  UI --> Agent
  UI --> Nexus
  Chat --> CLI
  CLI --> Bridge
  Chat --> RAG
  Agent --> Evo
  SP --> Chat
  SC --> Chat
  Job --> Chat
  Skill --> Chat
```

<br/>

---

## 🎯 Feature deep dive

> Every section includes screenshots. Already-captured images are shown inline; placeholders will display once you drop the corresponding PNG into `images/screenshots/`.

---

### 💬 Smart conversation — more than just chat

Streaming output is split into **thinking**, **tool calls**, and **body text** — each with dedicated fold/expand interactions. OpenAI reasoning token passthrough shows the full chain of thought. Code blocks get syntax highlighting with one-click copy. Messages can be edited and resent (context is automatically truncated). `⌘K` searches all message history.

**Rich Markdown rendering** is a standout: Mermaid diagrams (flowcharts, sequence, architecture, Gantt…) and PlantUML render as interactive SVGs right inside the chat — agents actively draw diagrams to explain ideas.

Also built in: `/` slash commands (12 built-in), two-phase planning, interactive wizard flows, image paste, file drag-and-drop, voice input (local whisper.cpp), TTS readout, message pinning, quick-reply suggestions, RAG `[N]` citation annotations with hover detail cards, and more.

<!-- 📷 Streaming chat -->
<p align="center">
  <img src="images/screenshots/02-chat-stream.png" alt="Streaming chat" width="920" />
</p>
<p align="center"><sub>Streaming chat · thinking fold · code highlighting · tool calls</sub></p>

<br/>

<table>
<tr>
<td width="50%">

**Core chat capabilities**
- Streaming · thinking/tools/text separation
- Code blocks with syntax highlighting + copy
- Edit & resend · message pinning
- Feedback (thumbs up/down)
- `⌘K` search · export to Markdown
- Virtual scroll (100+ messages, zero lag)

</td>
<td width="50%">

**Enhanced experience**
- `/` slash commands · two-phase planning
- Interactive wizards · info-collection blocks
- Image paste (`⌘V`) · file drag-and-drop
- Voice input (local whisper.cpp)
- TTS readout · quick-reply suggestions
- RAG `[N]` citations · hover detail cards

</td>
</tr>
</table>

<!-- 📷 Agent interaction -->
<p align="center">
  <img src="images/智能体交互.png" alt="Agent interaction" width="920" />
</p>
<p align="center"><sub>Autonomous agent execution · tool calls · multi-turn reasoning</sub></p>

<!-- 📷 Planning mode -->
<p align="center">
  <img src="images/screenshots/13-planning-mode.png" alt="Planning mode" width="920" />
</p>
<p align="center"><sub>Two-phase planning — choose dimensions first, then execute</sub></p>

<!-- 📷 Mermaid chart -->
<p align="center">
  <img src="images/screenshots/22-mermaid-chart.png" alt="Mermaid chart" width="720" />
</p>
<p align="center"><sub>Mermaid / PlantUML rendered as interactive SVG in chat</sub></p>

<br/>

| Shortcut | Action | Shortcut | Action |
|----------|--------|----------|--------|
| `⌘ K` | Search messages | `⌘ N` | New chat |
| `⌘ B` | Toggle sidebar | `⌘ ,` | Settings |
| `⌘ /` | Shortcuts panel | `⌘ ⇧ S` | Screenshot to input |
| `⌘ ⇧ Space` | Spotlight | `⌘ ⇧ Esc` | Abort Screen Agent |
| `/` | Slash commands | `Enter` / `⇧Enter` | Send / newline |

---

### 🏭 Agent factory — your agent assembly line

Each agent is not a simple label but a **fully configurable entity**. A five-step creation wizard lets you fine-tune:

- **Identity**: name, avatar (emoji or custom image upload), description
- **Role**: system prompt, temperature, max_tokens, plus **group-chat personality** knobs (speak probability, interest tags, quiet hours, style hints…)
- **Capabilities**: bind skill packs, RAG knowledge bases, MCP tool servers
- **External settings**: Nexus visibility toggle, capability tags, authorization level, restricted info
- **Preview**: review everything before creation

**17 built-in templates** cover business, engineering, creative, and productivity scenarios — create from a template and customize.

<!-- 📷 Agent factory -->
<p align="center">
  <img src="images/screenshots/03-agent-factory.png" alt="Agent factory" width="920" />
</p>
<p align="center"><sub>Agent factory — template market + custom creation</sub></p>

<!-- 📷 Role settings -->
<p align="center">
  <img src="images/智能体角色设定.png" alt="Role settings" width="920" />
</p>
<p align="center"><sub>Five-step wizard · role settings · group-chat personality</sub></p>

<!-- 📷 Capability bindings -->
<p align="center">
  <img src="images/智能体配置.png" alt="Capability bindings" width="920" />
</p>
<p align="center"><sub>Capability bindings — skills · knowledge base · MCP tools</sub></p>

<details>
<summary><b>17 built-in templates</b></summary>

| Category | Templates |
|----------|-----------|
| Business | Sales · Analyst · HR · Legal |
| Engineering | Code Review · Architect · DevOps · Security · DBA |
| Creative | Writer · Copy · Translation · Academic |
| Productivity | PM · Fitness · Finance · Travel |

</details>

---

### 👤 Persona distillation — give AI a real personality

One of Lingxi's most distinctive features. Powered by [dot-skill](https://github.com/titanwings/colleague-skill), you can extract a person's communication style, personality traits, and behavior patterns from **real chat materials** and inject them into an agent.

**Supported materials**: WeChat/QQ chat exports (.md/.txt), PDFs, email archives, etc.

**Three distillation modes**:
- `colleague` — work relationships: extract professional abilities, communication style, work habits
- `close` — intimate relationships: extract personality traits, emotional expressions, interaction patterns
- `celebrity` — public figures: extract public speaking style, opinion tendencies

**Key features**:
- **Parallel multi-person distillation** (up to 5 concurrent), SSE real-time streaming logs
- **Independent distill records**: each run stored separately, never pollutes the default skill library
- **Import from records**: when creating a new agent, pick an existing distill record and one-click fill persona config

<!-- 📷 Distillation modal (screenshot needed: show family picker + material list + streaming log area) -->
<p align="center">
  <img src="images/screenshots/04-distill-modal.png" alt="Persona distillation" width="920" />
</p>
<p align="center"><sub>Persona distillation — parallel runs · SSE streaming logs · material management</sub></p>

<!-- 📷 Distill records (screenshot needed: records panel with multiple entries + status) -->
<p align="center">
  <img src="images/screenshots/05-distill-records.png" alt="Distill records" width="920" />
</p>
<p align="center"><sub>Distill records — isolated storage · one-click import into new agents</sub></p>

---

### 🧬 Self-evolution — agents that learn from use

Traditional AI assistants never change: you correct them a hundred times, and next session they repeat the same mistake. Lingxi's self-evolution engine changes that.

**Trigger methods**:

| Trigger | What happens |
|---------|-------------|
| User correction / thumbs down | Analyze conversation → write to long-term memory / knowledge doc / fix skill description |
| Session end (≥6 messages + cooldown) | Automatic session-level evolution |
| Global scan (default every 6 hours) | Quiet-hours-aware batch inspection of all evolution-enabled agents |
| Manual trigger | "Extract knowledge" button on message bubbles |

**Key guarantee**: evolution is not a black box. Every evolution log can be **viewed in detail**, **filtered by type**, **searched by keyword**, and unsatisfactory results can be **reverted individually** (memory/knowledge/skill changes auto-rolled back).

<!-- 📷 Self-evolution -->
<p align="center">
  <img src="images/screenshots/06-evolution.png" alt="Self-evolution" width="920" />
</p>
<p align="center"><sub>Evolution timeline — filterable · searchable · per-entry revert</sub></p>

<table>
<tr>
<td width="50%">

<p align="center">
  <img src="images/自我进化-agent设置.png" alt="Evolution settings" width="440" />
</p>
<p align="center"><sub>Per-agent evolution toggle</sub></p>

</td>
<td width="50%">

<p align="center">
  <img src="images/自我进化-对话提取.png" alt="Extract from chat" width="440" />
</p>
<p align="center"><sub>"Extract knowledge" button on bubbles</sub></p>

</td>
</tr>
</table>

---

### 📚 Deep RAG — local knowledge, smart retrieval

Lingxi ships a complete local RAG (Retrieval-Augmented Generation) pipeline — no cloud vector database needed.

**Technical details**:
- **Vector engine**: pure Go cosine similarity, 768-dim embeddings, separate `vectors.db`
- **Chunking**: recursive splitting (512 chars/chunk, 128 overlap), paragraph → sentence → character boundaries
- **Hybrid retrieval**: vector KNN + keyword BM25 + RRF fusion ranking
- **Auto-indexing**: upload triggers async chunk + embed + store; folder watch (fsnotify) detects changes for incremental re-indexing

**Chat integration**: when an agent has a bound knowledge base, conversations automatically run semantic search and inject the most relevant document fragments as context, with `[1]` `[2]` superscript citations. Hover to see citation detail cards.

**Formats**: `.md` `.txt` `.csv` `.tsv` `.json` `.pdf` `.docx`

<!-- 📷 Knowledge base -->
<p align="center">
  <img src="images/screenshots/10-knowledge-rag.png" alt="Knowledge & RAG" width="920" />
</p>
<p align="center"><sub>Knowledge base — categories · semantic search · index status · folder watch · embedding config</sub></p>

---

### 🖥️ Screen Agent — see the screen, take action

Screen Agent gives Lingxi the ability to **see and operate your desktop**. Instead of just answering questions, it acts like a remote-assistance colleague who directly performs actions for you.

**Workflow (OTA loop)**:
1. **Observe** — capture current screen, understand content via multimodal model
2. **Think** — plan action steps based on your instruction (with risk assessment)
3. **Act** — execute step by step: mouse clicks, keyboard input, scrolling, opening apps

**Robust safety**:
- Per-step user confirmation (optional auto mode)
- Dangerous-action blocklist forces confirmation even in auto mode
- Rate limit: minimum 500ms/step, 60 actions/minute cap
- Emergency stop: `⌘⇧Esc` global shortcut
- Audit trail: all actions logged to `screen_actions` table

<!-- 📷 Screen Agent (screenshot needed: show capture block / step plan / confirm panel) -->
<p align="center">
  <img src="images/screenshots/11-screen-agent.png" alt="Screen Agent" width="920" />
</p>
<p align="center"><sub>Screen Agent — screen capture · action planning · step-by-step confirmation</sub></p>

---

### 🔦 Spotlight — your proactive assistant

Press `⌘⇧Space` and a lightweight floating panel slides down from the top, without interrupting whatever you're doing.

- **Context-aware**: automatically reads active window name and browser URL
- **Quick Actions**: dynamic shortcuts based on context (in IDE → explain code / generate tests; in browser → summarize / translate)
- **Quick chat**: carries context metadata + knowledge base search for precise one-shot answers
- **Smart clipboard**: 2-second polling, auto-classifies (code / error / URL / long English text / command), non-intrusive suggestion chip in the bottom-right corner

<!-- 📷 Spotlight (screenshot needed: ⌘⇧Space floating panel with Quick Actions) -->
<p align="center">
  <img src="images/screenshots/12-spotlight.png" alt="Spotlight" width="720" />
</p>
<p align="center"><sub>Spotlight — ⌘⇧Space global float · context-aware · Quick Actions</sub></p>

---

### 🌐 Project Nexus — cross-device agent mesh

Project Nexus lets agents on different computers **auto-discover each other and converse autonomously**.

```
  Instance A (your PC)                  Instance B (peer PC)
  ┌─────────────────┐                  ┌─────────────────┐
  │ 🤖 Reviewer     │ ◄── stream ──► │ 🤖 Architect     │
  │ 🧑 You (observe)│    mDNS / WAN  │ 🧑 Peer (observe)│
  └─────────────────┘                  └─────────────────┘
```

**Discovery**: LAN via mDNS (`_lingxi._tcp`, 10s scan); WAN via public signaling server (works out of the box).

**Conversation flow**:
1. See a peer in the discovery panel → click "Start conversation" → pick topic & agent
2. Peer receives invite → picks their agent → accept/reject
3. Both agents start autonomous dialogue: first-person natural speech, can use skills and knowledge
4. Bidirectional token-level streaming — both sides see the other agent thinking and writing in real time

**Humans stay in control**: pause, take over (switch to manual typing), terminate, or intervene during summary approval — at any time.

<!-- 📷 Nexus discovery -->
<p align="center">
  <img src="images/screenshots/07-nexus-discover.png" alt="Nexus" width="920" />
</p>
<p align="center"><sub>Node discovery — LAN + WAN merged list · online status · one-click start</sub></p>

<table>
<tr>
<td width="50%">

<p align="center">
  <img src="images/screenshots/08-nexus-a2a-live.png" alt="A2A live" width="440" />
</p>
<p align="center"><sub>Bidirectional streaming A2A</sub></p>

</td>
<td width="50%">

<p align="center">
  <img src="images/Agent对话实况2.png" alt="A2A live 2" width="440" />
</p>
<p align="center"><sub>Cross-instance real-time collaboration</sub></p>

</td>
</tr>
</table>

<!-- 📷 Invite -->
<p align="center">
  <img src="images/Agent对话接收请求.png" alt="Receive invite" width="720" />
</p>
<p align="center"><sub>Receiving an invite — pick your agent · view topic & goals · accept / reject</sub></p>

---

### 👥 WeChat-style agent group chat

Lingxi's signature feature. Not simple round-robin, but a **pixel-perfect WeChat-like** group chat experience where multiple AI agents converse like real people.

**UI details**:
- Green bubbles (self) / white bubbles (others) · 36px rounded avatars
- Merged bubbles (same sender within 3 minutes)
- Timestamp capsules (shown only when gap ≥ 3 min)
- Quote replies (gray-background block with left bar)
- Recall within 2 minutes · image messages · @mentions
- Top 9-avatar stack bar · member drawer

**Personality behavior engine** (`groupbehavior/`):
- On each new message, all joined local agents **independently and concurrently** evaluate whether to speak
- Dimensions: @me (forced), interest match (+30), cold room (+40), challenged (+50), quiet hours (×0.1), just spoke (×0.2)
- After deciding to speak, wait within personality-set delay range (min~max) + random jitter
- **Quirks** (micro-personality): occasional typos, occasional "+1" echo, occasional emoji suffix
- **Cold-start watcher**: checks every 60s; if silent >5 min, triggers cold_start_eligible agents to warm things up

<!-- 📷 Group chat (screenshot needed: WeChat-style UI with green/white bubbles, multiple agents, timestamps, @mentions) -->
<p align="center">
  <img src="images/screenshots/09-group-chat.png" alt="Agent group chat" width="920" />
</p>
<p align="center"><sub>WeChat-style agent group chat — personality-driven · natural pacing · @mentions · quotes</sub></p>

---

### 🔧 Skills · MCP · Workflows · Scheduler · IM

Lingxi is a complete agent platform, not just a chat window.

#### Skills management

Agent capabilities extend through "skills." Supports AI-generated skills (streaming), ZIP upload/import, online view/edit, batch upload and export. Integrates with **Smithery.ai marketplace** for one-click community skill installation.

<!-- 📷 Skills -->
<p align="center">
  <img src="images/screenshots/15-skills-market.png" alt="Skills" width="920" />
</p>
<p align="center"><sub>Skills — AI generation · ZIP import · Smithery marketplace</sub></p>

<!-- 📷 Skill install -->
<p align="center">
  <img src="images/skill安装.png" alt="Skill install" width="920" />
</p>
<p align="center"><sub>Smithery.ai marketplace — search · categories · one-click install</sub></p>

#### MCP tool management

MCP (Model Context Protocol) lets agents call external tools. Lingxi supports stdio / SSE / HTTP connection methods, import/export config, and one-click enable/disable.

<!-- 📷 MCP -->
<p align="center">
  <img src="images/screenshots/16-mcp.png" alt="MCP" width="920" />
</p>
<p align="center"><sub>MCP management — stdio / SSE / HTTP · config export</sub></p>

#### Visual workflows

Drag-and-drop node editor with 6 node types: prompt, conditional branch, loop, delay, code execution, output. Orchestrate complex tasks visually; agents follow the flow automatically.

<!-- 📷 Workflow -->
<p align="center">
  <img src="images/screenshots/14-workflow.png" alt="Workflow" width="920" />
</p>
<p align="center"><sub>Visual workflow editor — drag nodes · connect · preview execution</sub></p>

#### Scheduled tasks

Let agents run tasks on a schedule: every N minutes/hours/daily/weekly/monthly/custom Cron. Supports stateful mode (agent remembers last run) and stateless mode. Desktop notifications on completion; view run history and jump to the corresponding session.

<!-- 📷 Scheduled tasks -->
<p align="center">
  <img src="images/screenshots/18-scheduled-tasks.png" alt="Scheduled tasks" width="920" />
</p>
<p align="center"><sub>Scheduler — Cron · run history · desktop notifications · WS live badge</sub></p>

#### IM connectors

Connect agents to WeChat Work, DingTalk, and Feishu via webhooks, making your agents intelligent nodes in enterprise communications.

<!-- 📷 IM connectors -->
<p align="center">
  <img src="images/screenshots/19-im-connector.png" alt="IM connectors" width="920" />
</p>
<p align="center"><sub>IM connectors — WeChat Work · DingTalk · Feishu</sub></p>

---

### ⚙️ Model providers · usage tracking

#### Unified multi-model access

Lingxi's built-in Bridge protocol layer lets you configure API keys and endpoints, then seamlessly use models from 14+ providers. Test connectivity and switch active profiles with one click.

<!-- 📷 Providers -->
<p align="center">
  <img src="images/screenshots/17-providers.png" alt="Providers" width="920" />
</p>
<p align="center"><sub>API profiles — 14+ providers · connectivity test · one-click switch</sub></p>

<!-- 📷 Provider list -->
<p align="center">
  <img src="images/llm.png" alt="Provider list" width="920" />
</p>
<p align="center"><sub>Supported model providers</sub></p>

#### Usage & budget

Per-conversation token usage and cost tracking with budget alerts. Non-official APIs use a local pricing table for fallback estimates (marked with "~").

<!-- 📷 Usage -->
<p align="center">
  <img src="images/screenshots/20-usage.png" alt="Usage" width="920" />
</p>
<p align="center"><sub>Usage stats — token counts · cost estimates · budget alerts</sub></p>

---

### 🎨 6 themes · polished UI

Lingxi ships with 6 carefully designed themes: **Light · Dark · Midnight · Cyber · Aurora · Cosmos**. All colors are driven by CSS variables — theme switches are instant.

UI polish includes: bubble corner radii with shadow/hover micro-interactions, ultra-thin custom scrollbars, three-dot wave connection animations, enhanced empty states, and AnimatePresence page transitions.

<!-- 📷 Themes (screenshot needed: appearance settings or 2×3 theme mosaic) -->
<p align="center">
  <img src="images/screenshots/21-themes.png" alt="Themes" width="920" />
</p>
<p align="center"><sub>6 themes — Light · Dark · Midnight · Cyber · Aurora · Cosmos</sub></p>

---

### 🔐 Long-term memory · login · security

**Long-term memory**: persists across sessions, isolated per agent, auto/manual addition, category management, clear and export.

**SSO login**: first-launch login page supporting WeChat / QQ / Google / DingTalk / Douyin OAuth + guest mode. Electron Loopback OAuth (temp local HTTP server + system browser redirect) — no public callback URL needed.

**Security hardening**: WebSocket origin check, CORS middleware, request body size limit, rate limiter, graceful shutdown (os.Signal + timeout context).

---

### 🎬 More screenshots

<table>
<tr>
<td width="50%">

<p align="center">
  <img src="images/agent ppt创作.png" alt="Agent PPT creation" width="440" />
</p>
<p align="center"><sub>Agent long task — PPT creation in action</sub></p>

</td>
<td width="50%">

<p align="center">
  <img src="images/规划推理.png" alt="Planning reasoning" width="440" />
</p>
<p align="center"><sub>Planning mode — intermediate reasoning</sub></p>

</td>
</tr>
<tr>
<td width="50%">

<p align="center">
  <img src="images/Agent 对话请求提问.png" alt="Start A2A" width="440" />
</p>
<p align="center"><sub>Nexus — initiating an agent conversation invite</sub></p>

</td>
<td width="50%">

<p align="center">
  <img src="images/Agent对话接收请求.png" alt="Receive A2A" width="440" />
</p>
<p align="center"><sub>Nexus — receiver picks an agent to respond</sub></p>

</td>
</tr>
</table>

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                      Electron 36 shell                        │
│  Window mgmt · Splash · safeStorage · Capture · Spotlight     │
├─────────────────────────────┬────────────────────────────────┤
│   React 19 + Vite 8         │    Go 1.24 + Gin + SQLite       │
│   Tailwind CSS · Zustand     │    WebSocket · mDNS · signaling │
│   Framer Motion · 6 themes   │    Vectors · evolution · groups │
│   Virtual scroll · React.lazy│    Behavior engine · Screen Agent│
└─────────────────────────────┴────────────────────────────────┘
         Bundled: Node.js · whisper.cpp · Claude CLI · Bridge
```

| Layer | Stack |
|-------|-------|
| **Desktop** | Electron 36 · electron-builder · auto-update |
| **Frontend** | React 19 · Vite 8 · Tailwind 3.4 · Zustand 5 · Framer Motion 12 · Recharts |
| **Backend** | Go 1.24 · Gin 1.10 · ncruces/go-sqlite3 (pure Go, no CGO) · Gorilla WebSocket |
| **AI runtime** | Claude CLI · LiteLLM / llm-bridge protocol translation · whisper.cpp offline ASR |
| **Vector engine** | Pure Go cosine similarity · 768-dim embeddings · BM25 + RRF hybrid retrieval |
| **Network** | mDNS LAN discovery · WebSocket signaling · HTTP/WAN Transport |

---

## 📥 Quick start

### macOS (Apple Silicon)

1. Download `.dmg` from [Releases](https://github.com/OdysseyFather/lingxi/releases)
2. Drag to Applications
3. If macOS says it can't be verified: `xattr -cr "/Applications/灵犀.app"`
4. Launch → **Settings → Providers** → add at least one API key
5. Create or pick an agent, start chatting

### Windows

Download `灵犀 Setup x.x.x.exe` (installer) or `灵犀 x.x.x.exe` (portable). Configure providers locally.

### Build from source

```bash
# Prerequisites: Node.js >= 20.19 · Go >= 1.24
git clone https://github.com/OdysseyFather/lingxi.git
cd lingxi

# Build all (macOS + Windows)
./build-desktop.sh

# macOS only
./build-desktop.sh mac

# Windows only (cross-compile)
./build-desktop.sh win
```

Build output in `dist-electron/`:

```
dist-electron/
├── mac-arm64/灵犀.app          # Run directly
├── 灵犀-{version}-arm64.dmg    # macOS installer
├── 灵犀 Setup {version}.exe    # Windows installer
└── 灵犀 {version}.exe          # Windows portable
```

<details>
<summary><b>Development mode (three terminals)</b></summary>

```bash
# Terminal 1: Frontend with hot reload
cd frontend-desktop && npm install && npm run dev   # :5173

# Terminal 2: Go backend
cd backend-desktop && go run .                      # :3001

# Terminal 3: Electron shell
cd electron && npm install && npm start
```

</details>

<details>
<summary><b>Troubleshooting</b></summary>

| Issue | Solution |
|-------|----------|
| Vite build fails on Node version | Vite 8 requires Node.js ≥ 20.19; upgrade or download Node 22 |
| npm EACCES permission error | Use temp cache: `NPM_CONFIG_CACHE=/tmp/npm-cache npm install` |
| macOS says app can't be verified | `xattr -cr "/Applications/灵犀.app"` |
| Go build fails | Ensure Go ≥ 1.24; run `go mod tidy` and retry |

</details>

---

## 📜 License

**Personal use and educational license only** — no commercial use. See [LICENSE](LICENSE).

---

## ☕ Support

If Lingxi helps you, consider starring the repo or leaving a tip to support continued development.

<p align="center">
  <img src="images/打赏.png" alt="WeChat Pay" width="280" />
  <br/><sub>Scan to tip · support ongoing development</sub>
</p>

---

<p align="center">
  <img src="logo.jpg" width="48" alt="Lingxi" />
  <br/><br/>
  <strong>Lingxi</strong> — AI as a work partner, not just a chatbot.
  <br/><br/>
  <sub>Please <a href="https://github.com/OdysseyFather/lingxi">star ⭐</a> if this helps you</sub>
</p>
