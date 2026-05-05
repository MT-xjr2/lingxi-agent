# CLAUDE.md — 灵犀 AI Agent 项目指南

本文件为 AI 助手（Claude / Cursor / Copilot 等）提供项目上下文，帮助快速理解系统全貌并高效开发。

---

## 项目简介

**灵犀 AI Agent** 是一个本地优先的桌面 AI Agent 工作台，采用 Electron + React + Go 三层架构。支持多模型接入、智能体工厂、技能管理、知识库、MCP 工具、IM 集成等能力。

---

## 技术栈

### 前端 `frontend-desktop/`
- **React 19** + **Vite 8**（构建需 Node.js ≥ 20.19 或 ≥ 22.12）
- **Tailwind CSS 3.4** — 全局样式，6 套主题通过 CSS 变量切换
- **Zustand 5** — 全局状态管理（`src/state/useStore.js`）
- **Framer Motion 12** — 页面过渡、列表动画
- **Lucide React** — 图标（不使用 emoji）
- **prism-react-renderer 2** — 代码高亮
- **@tanstack/react-virtual 3** — 虚拟滚动
- **react-markdown + remark-gfm** — Markdown 渲染
- **Recharts 3** — 用量图表

### 后端 `backend-desktop/`
- **Go 1.24** + **Gin 1.10**
- **Gorilla WebSocket** — 流式对话
- **ncruces/go-sqlite3** — 纯 Go SQLite（无 CGO 依赖）
- **ledongthuc/pdf** — PDF 文本提取
- **nguyenthenguyen/docx** — DOCX 文本提取

### 桌面壳 `electron/`
- **Electron 36** + **electron-builder 25**
- 打包目标: macOS arm64

---

## 项目结构

```
lingxi-agent/
├── backend-desktop/          # Go 后端
│   ├── main.go               # 入口 + 路由注册
│   ├── config/               # 配置管理
│   ├── db/                   # SQLite 数据层
│   │   ├── db.go             # 表定义、CRUD
│   │   ├── nexus.go          # Nexus 表 CRUD（peers/contacts/a2a）
│   │   └── mcp_agent.go      # MCP-Agent 关联
│   ├── handler/              # HTTP Handlers
│   │   ├── agent.go          # 智能体 CRUD
│   │   ├── chat.go           # 对话 + WebSocket 流式
│   │   ├── knowledge.go      # 知识库（支持 .md/.txt/.csv/.json/.pdf/.docx）
│   │   ├── session.go        # 会话管理 + 消息搜索
│   │   ├── provider.go       # 模型接入点
│   │   ├── skill.go          # 技能管理
│   │   ├── mcp.go            # MCP 服务管理
│   │   ├── usage.go          # 用量统计
│   │   ├── im_connector.go   # IM 连接器
│   │   ├── scheduled.go      # 定时任务 CRUD
│   │   ├── nexus.go          # Nexus 对外 API + 设置 + TokenAuth 中间件
│   │   ├── nexus_contact.go  # 建联管理（请求/响应/列表/删除）
│   │   ├── a2a_conversation.go # A2A 对话管理（发起/接受/拒绝/暂停/接管/终止/审批）
│   │   ├── agent_nexus_config.go # Agent 对外设置 CRUD
│   │   ├── memory.go         # 长期记忆 CRUD + 消息固定
│   │   ├── transcribe.go     # 语音识别（本地 whisper.cpp 优先，回退远端 API）
│   │   └── ws_hub.go         # WebSocket Hub
│   ├── connector/            # IM 平台对接（企微/钉钉/飞书）
│   ├── model/                # 数据模型
│   ├── nexus/                # Agent 间对话引擎（Project Nexus）
│   │   ├── discovery.go      # mDNS 发现服务
│   │   ├── conversation.go   # 对话执行引擎（RunConversation + HandleIncomingMessage）
│   │   └── http_client.go    # HTTP 通信工具（PostJSON + 重试）
│   ├── router/               # AI 引擎路由（CCR）
│   ├── scheduler/            # 定时任务调度器
│   └── usage/                # 用量计算 + 定价
│
├── frontend-desktop/         # React 前端
│   ├── src/
│   │   ├── main.jsx          # 入口
│   │   ├── index.css         # Tailwind + 主题 CSS 变量
│   │   ├── api/client.js     # fetch 封装
│   │   ├── state/useStore.js # Zustand store
│   │   ├── ui/               # 通用 UI
│   │   │   ├── AppShell.jsx  # 主布局（顶部导航+侧边栏+主区域+AnimatePresence）
│   │   │   ├── primitives.jsx # 原子组件（Button/Card/Modal/Badge/Input...）
│   │   │   ├── cn.js         # clsx + tailwind-merge
│   │   │   ├── SidebarSessions.jsx  # 会话列表（置顶/重命名/批量删除）
│   │   │   ├── ModelSwitcher.jsx
│   │   │   └── RouterPill.jsx
│   │   ├── chat/             # 对话模块
│   │   │   ├── ChatView.jsx  # 对话主页面
│   │   │   ├── Composer.jsx  # 输入框 + 斜杠命令 + 图片粘贴
│   │   │   ├── MessageList.jsx # 消息列表 + 虚拟滚动
│   │   │   ├── Bubble.jsx    # 消息气泡 + 复制按钮
│   │   │   ├── blocks.jsx    # 文本块/思考块/工具块渲染
│   │   │   ├── SearchModal.jsx # Cmd+K 全文搜索
│   │   │   └── AgentPicker.jsx
│   │   ├── settings/         # 设置页
│   │   │   ├── SettingsPage.jsx
│   │   │   ├── ProfilesPage.jsx   # 接入点管理
│   │   │   ├── AppearancePage.jsx  # 6 套主题
│   │   │   ├── MemoryPage.jsx     # 长期记忆管理
│   │   │   ├── NexusSettingsPage.jsx # 网络与协作设置
│   │   │   └── UsagePage.jsx       # 用量 + 预算预警
│   │   ├── nexus/            # Agent 间对话（Project Nexus）
│   │   │   ├── NexusPage.jsx        # 统一 Nexus 赛博朋克仪表盘（含发现/联系人/对话）
│   │   │   ├── NearbyPage.jsx       # 附近面板（已整合入 NexusPage）
│   │   │   ├── ContactsPage.jsx     # 联系人面板（已整合入 NexusPage）
│   │   │   ├── A2AConversationView.jsx # Agent 对话观察视图（已整合入 NexusPage）
│   │   │   ├── A2AMessageBubble.jsx    # 专用消息气泡
│   │   │   └── StartA2AModal.jsx       # 发起对话弹窗
│   │   ├── AgentFactoryPage.jsx    # 智能体工厂 + 模板市场
│   │   ├── WorkflowPage.jsx        # 可视化工作流编排（拖拽节点式编辑器）
│   │   ├── SkillsPage.jsx
│   │   ├── KnowledgePage.jsx
│   │   ├── MCPPage.jsx
│   │   ├── IMConnectorPage.jsx
│   │   └── ScheduledTasksPage.jsx  # 定时任务管理
│   ├── package.json
│   ├── vite.config.js
│   ├── tailwind.config.js
│   └── postcss.config.js
│
├── electron/                 # Electron 主进程
│   ├── main.js               # 窗口管理、子进程启动
│   ├── preload.js            # IPC Bridge
│   ├── splash.html           # 冷启动 Splash 页（后端就绪前显示）
│   ├── package.json          # electron-builder 配置
│   ├── assets/               # 图标、entitlements
│   └── resources/            # 构建时填充的运行时资源
│       ├── ai-engine/        # Claude CLI
│       ├── bridge/           # llm-bridge (JS)
│       ├── litellm-bridge/   # LiteLLM Bridge (Python)
│       ├── node-bin/         # 内嵌 Node.js
│       └── whisper/          # whisper.cpp 离线 ASR (whisper-cli + ggml-base.bin)
│
├── ai-config/                # AI 引擎配置模板
├── build-desktop.sh          # 一键构建脚本
├── CLAUDE.md                 # 本文件
├── README.md                 # 用户文档
└── README-EN.md              # 英文文档
```

---

## 核心开发流程

### 开发模式

```bash
# 终端 1: 前端（热更新）
cd frontend-desktop && npm install && npm run dev

# 终端 2: Go 后端
cd backend-desktop && go run .

# 终端 3: Electron
cd electron && npm install && npm start
```

### 打包 & 安装

```bash
# 0. 确保 Node.js 版本足够
export PATH="/tmp/node22/bin:$PATH"  # 若系统 node < 20.19

# 1. 退出现有程序
pkill -x "灵犀" 2>/dev/null; sleep 1

# 2. 一键构建（支持 mac / win / all）
./build-desktop.sh          # 默认 macOS
./build-desktop.sh win      # Windows（交叉编译）
./build-desktop.sh all      # 同时构建两个平台

# 3. 覆盖安装（macOS）
rm -rf "/Applications/灵犀.app"
cp -R "dist-electron/mac-arm64/灵犀.app" "/Applications/灵犀.app"
xattr -cr "/Applications/灵犀.app"
open "/Applications/灵犀.app"
```

### 自动更新发版

应用内置 `electron-updater`（generic provider），CDN 地址配置在 `electron/package.json` 的 `build.publish.url`。

**发版步骤：**
1. 修改 `electron/package.json` 中的 `version`
2. 运行 `./build-desktop.sh`
3. 上传到 CDN：`latest-mac.yml` + `灵犀-{version}-arm64-mac.zip` + `.blockmap`

**构建产物：**
```
dist-electron/
├── mac-arm64/灵犀.app              # 直接运行
├── 灵犀-{version}-arm64.dmg        # 安装包
├── 灵犀-{version}-arm64-mac.zip    # 自动更新包
├── 灵犀-{version}-arm64-mac.zip.blockmap
└── latest-mac.yml                   # 版本清单
```

**运行时行为：**
- 启动 10 秒后检查更新，之后每 4 小时轮询
- 发现新版本 → native dialog 提示下载 → 下载完成提示重启
- 设置 → 关于与更新 可手动检查
- `app.isPackaged === false` 时跳过
- IPC: `check-for-update` / `update-status` 事件

---

## 开发约定

### 必须遵守

1. **不允许开启子代理** — 所有开发任务在当前会话中直接完成
2. **每次开发完成后** 必须执行：
   - 打包 & 覆盖安装（流程见上方）
   - 更新 `.cursor/rules/lingxi-agent.mdc`（如有架构/规范变更）
   - 更新 `CLAUDE.md`（如有新模块/技术栈/流程变更）
   - 更新 `README.md`（如有用户可见的新功能/快捷键/配置）
3. **前端样式只用 Tailwind CSS + CSS 变量**，不写独立 CSS 文件
4. **组件必须使用 primitives.jsx 中的原子组件**（Button/Card/Modal 等）
5. **图标只用 lucide-react**
6. **状态管理只用 Zustand**（`useStore`）
7. **className 合并使用 `cn()` 函数**

### 编码风格

- 前端：函数组件 + Hooks，不使用 class 组件
- 后端：标准 Go 风格，handler 函数签名统一 `func XxxHandler(c *gin.Context)`
- 注释语言：中文
- 变量/函数命名：英文

### CSS 变量命名

```
--bg           背景
--bg-soft      次级背景
--bg-elev      悬浮/卡片背景
--text          主文字
--text-soft     次级文字
--text-faint    最淡文字
--accent        主题强调色
--accent-soft   强调色淡底
--accent-glow   强调色发光
--line          分割线
--ring          聚焦边框
```

### API 路由一览

| Method | Path | Handler | 说明 |
|--------|------|---------|------|
| GET | /api/sessions | ListSessions | 会话列表（支持 agent_id 筛选，按 pinned DESC 排序） |
| POST | /api/sessions | CreateSession | 创建会话 |
| PATCH | /api/sessions/:id | UpdateSession | 更新会话（title/pinned/folder） |
| DELETE | /api/sessions/:id | DeleteSession | 删除会话 |
| POST | /api/sessions/batch-delete | BatchDeleteSessions | 批量删除会话 |
| GET | /api/sessions/:id/messages | ListMessages | 消息列表 |
| GET | /api/sessions/:id/pending | GetPendingTask | 查询挂起任务 |
| DELETE | /api/sessions/:id/pending | ClearPendingTask | 清除挂起任务 |
| POST | /api/sessions/:id/agent | SetSessionAgent | 设置会话智能体 |
| GET | /api/messages/search | SearchMessages | 消息全文搜索 |
| PUT | /api/messages/:id | UpdateMessage | 编辑用户消息（+删除后续） |
| POST | /api/messages/:id/feedback | SetMessageFeedback | 消息反馈（up/down） |
| POST | /api/messages/:id/pin | ToggleMessagePin | 固定/取消固定消息 |
| POST | /api/chat | Chat | 发起对话 |
| POST | /api/chat/batch | BatchChat | 批量对话 |
| POST | /api/chat/abort | AbortChat | 中止正在进行的对话 |
| GET | /ws | WebSocket | 流式对话 |
| GET | /api/tasks | ListTasks | 后台任务列表 |
| DELETE | /api/tasks/:id | DeleteTask | 删除后台任务 |
| GET/POST/DELETE | /api/agents/* | Agent CRUD | 智能体管理 |
| GET/POST/DELETE | /api/knowledge/* | Knowledge CRUD | 知识库管理 |
| GET | /api/knowledge/:id/preview | PreviewKnowledge | 知识库预览 |
| GET/POST/DELETE | /api/api-profiles/* | Profile CRUD | 接入点管理 |
| POST | /api/api-profiles/:id/activate | ActivateAPIProfile | 激活接入点 |
| POST | /api/api-profiles/:id/test | TestAPIProfile | 测试连通性 |
| GET | /api/skills | ListSkills | 技能列表 |
| POST | /api/skills/upload | UploadSkill | 上传技能 |
| POST | /api/skills/batch-upload | BatchUploadSkill | 批量上传技能 |
| POST | /api/skills/generate/stream | GenerateSkillStream | AI 流式生成技能 |
| POST | /api/skills/generate/confirm | ConfirmGeneratedSkill | 确认生成的技能 |
| GET | /api/skills/:id/content | GetSkillContent | 技能文件内容 |
| PUT | /api/skills/:id/content | UpdateSkillContent | 更新技能文件 |
| GET | /api/skills/:id/export | ExportSkill | 导出技能 ZIP |
| POST | /api/skills/:id/install | InstallSkill | 安装技能 |
| POST | /api/skills/:id/uninstall | UninstallSkill | 卸载技能 |
| DELETE | /api/skills/:id | DeleteSkill | 删除技能 |
| GET | /api/skills/marketplace | MarketplaceSearch | Smithery 市场搜索 |
| GET | /api/skills/marketplace/categories | MarketplaceCategories | 市场分类 |
| GET | /api/skills/marketplace/:ns/:slug | MarketplaceGetSkill | 市场技能详情 |
| POST | /api/skills/marketplace/install | MarketplaceInstall | 安装市场技能 |
| GET/POST/PUT/DELETE | /api/mcp/* | MCP CRUD | MCP 管理 |
| POST | /api/mcp/:id/toggle | ToggleMCPServer | 启用/禁用 MCP |
| GET | /api/mcp/export | ExportMCPConfig | 导出 MCP 配置 |
| GET/POST/PUT/DELETE | /api/im-connectors/* | IM CRUD | IM 连接器管理 |
| GET | /api/providers | ListProviders | 供应商列表 |
| GET | /api/usage | GetUsage | 用量查询 |
| GET | /api/usage/quota | GetUsageQuota | 额度查询 |
| GET | /api/router/status | GetRouterStatus | 路由状态 |
| POST | /api/router/stop | StopRouter | 停止路由 |
| GET | /api/scheduled-tasks | ListScheduledTasks | 定时任务列表 |
| POST | /api/scheduled-tasks | CreateScheduledTask | 创建定时任务 |
| PUT | /api/scheduled-tasks/:id | UpdateScheduledTask | 更新定时任务 |
| DELETE | /api/scheduled-tasks/:id | DeleteScheduledTask | 删除定时任务 |
| POST | /api/scheduled-tasks/:id/toggle | ToggleScheduledTask | 启用/禁用 |
| POST | /api/scheduled-tasks/:id/run | TriggerScheduledTask | 手动触发 |
| GET | /api/scheduled-tasks/:id/runs | ListScheduledTaskRuns | 执行记录 |
| GET | /api/memories | ListMemories | 长期记忆列表 |
| POST | /api/memories | CreateMemory | 添加记忆 |
| DELETE | /api/memories/:id | DeleteMemory | 删除记忆 |
| DELETE | /api/memories/clear | ClearMemories | 清空记忆 |
| POST | /api/transcribe | TranscribeAudio | 语音识别（本地 whisper.cpp 优先，回退远端 API） |
| POST | /api/runtime/active-secret | SetActiveSecret | 运行时密钥注入 |
| GET | /api/nexus/info | NexusInfo | 本实例公开信息（免 token） |
| POST | /api/nexus/connect-request | NexusConnectRequest | 接收建联请求（免 token） |
| POST | /api/nexus/connect-respond | NexusConnectRespond | 接收建联响应（免 token） |
| POST | /api/nexus/conversation/request | NexusReceiveConvRequest | 接收对话请求 |
| POST | /api/nexus/conversation/accept | NexusReceiveConvAccept | 接收对话接受通知 |
| POST | /api/nexus/conversation/reject | NexusReceiveConvReject | 接收对话拒绝通知 |
| POST | /api/nexus/conversation/message | NexusReceiveMessage | 接收 A2A 对话消息 |
| POST | /api/nexus/conversation/pause | NexusReceivePause | 接收暂停通知 |
| POST | /api/nexus/conversation/terminate | NexusReceiveTerminate | 接收终止通知 |
| POST | /api/nexus/conversation/stream-token | NexusReceiveStreamToken | 接收远端流式 token 转发 |
| GET | /api/nexus/settings | GetNexusSettings | 网络协作设置 |
| PUT | /api/nexus/settings | UpdateNexusSettings | 更新协作设置 |
| GET | /api/peers | ListPeers | 已发现的局域网实例 |
| GET | /api/contacts | ListContacts | 已建联联系人列表 |
| POST | /api/contacts/request | SendConnectRequest | 发起建联 |
| POST | /api/contacts/:id/respond | RespondConnect | 响应建联 |
| DELETE | /api/contacts/:id | DeleteContact | 解除建联 |
| GET | /api/a2a-conversations | ListA2AConversations | Agent 对话列表 |
| POST | /api/a2a-conversations | CreateA2AConversation | 发起 Agent 对话 |
| GET | /api/a2a-conversations/:id | GetA2AConversation | 对话详情+消息 |
| POST | /api/a2a-conversations/:id/pause | PauseA2AConversation | 暂停对话 |
| POST | /api/a2a-conversations/:id/takeover | TakeoverA2AConversation | 人类接管 |
| POST | /api/a2a-conversations/:id/terminate | TerminateA2AConversation | 终止对话 |
| POST | /api/a2a-conversations/:id/approve | ApproveA2AConversation | 审批结果 |
| POST | /api/a2a-conversations/:id/accept-remote | AcceptRemoteConversation | 接受远端对话（含创建会话） |
| POST | /api/a2a-conversations/:id/reject-remote | RejectRemoteConversation | 拒绝远端对话 |
| GET | /api/agents/:id/nexus-config | GetAgentNexusConfig | Agent 对外设置 |
| PUT | /api/agents/:id/nexus-config | UpsertAgentNexusConfig | 更新对外设置 |

---

## 数据存储

- **SQLite 数据库**: `~/Library/Application Support/灵犀/smart-agent.db`
- **知识库文件**: `~/Library/Application Support/灵犀/knowledge/`（按 docs/qa/data 分类）
- **上传图片**: `~/Library/Application Support/灵犀/uploads/`
- **AI 引擎 HOME**: `~/Library/Application Support/灵犀/ai-home/`
- **Bridge 数据**: `~/Library/Application Support/灵犀/bridge-home/`

---

## 常见问题排查

### Vite 构建报 Node 版本错误
Vite 8 需要 Node.js ≥ 20.19。解决：下载 Node 22 并设置 PATH。

### npm EACCES 权限错误
使用临时缓存目录：`NPM_CONFIG_CACHE=/tmp/npm-lingxi-cache npm install ...`

### macOS 提示应用无法验证
```bash
xattr -cr "/Applications/灵犀.app"
```

### Go 编译失败
确保 Go ≥ 1.24，执行 `go mod tidy` 后重试。

---

## 已实现的功能（最新）

### 对话体验
- 流式输出 + 思考过程折叠
- 代码块语法高亮 + 复制按钮
- 消息一键复制
- Cmd+K 全文消息搜索
- 对话导出为 Markdown
- / 斜杠命令快捷输入（12 个内置命令）
- 虚拟滚动（100+ 条消息自动启用）
- **统一 Agent 模式（自主执行）**
- **交互式信息收集块（选择块 + 输入块），Agent 按需向用户提问**
- **用户 & 智能体头像显示**
- **图片粘贴（Cmd+V）+ 聊天中图片展示**
- **OpenAI 兼容模型思考链（reasoning）展示**
- **消息编辑/重发（hover 编辑按钮 → textarea 内联编辑 → 保存并重发，自动截断后续消息）**
- **消息反馈（thumbs up/down，持久化到 SQLite，选中状态高亮）**
- **知识库 RAG 引用可视化（内联 [N] 上角标 + hover 弹出引用详情 + 气泡底部引用列表折叠卡片）**
- **语音输入（MediaRecorder 录音 + 本地 whisper.cpp 离线识别，回退远端 API → 文字填入输入框）**
- **TTS 朗读（Web Speech API，AssistantBubble 朗读按钮，支持中/英文）**
- **文件拖拽对话（拖入 .md/.py/.go/.json 等文本文件，内容作为消息附件发送）**
- **快捷截屏（Cmd+Shift+S 全局快捷键 + 按钮截屏，截图自动填入输入框）**
- **消息固定（Pin 重要消息，用户和助理消息均支持）**
- **快捷回复建议（assistant 回复后显示 2-3 个推荐后续问题胶囊按钮）**
- **对话中止按钮（abort 正在进行的 AI 回复）**
- **对话批量发送（batch chat）**

### 智能体
- 智能体工厂（创建/编辑/删除）
- **五步引导式创建向导（身份/角色/能力/对外设置/预览）**
- **支持 temperature、max_tokens 参数调整**
- **支持 post_actions 后续动作（工作流链式执行数据模型）**
- **Agent 对外设置（公开/能力标签/授权级别/禁止透露信息/知识库限定）**
- 模板市场（4 类 17 个模板：商业办公/技术开发/内容创意/生活效率）
- 智能体绑定模型/技能/MCP/知识库

### 工作流
- **可视化工作流编排（WorkflowPage：拖拽节点式编辑器）**
- **6 种节点类型：提示词/条件分支/循环/延迟/代码/输出**
- **节点间连线 + 执行预览**

### 技能管理
- **Smithery.ai 技能市场集成（搜索/分类/详情/安装）**
- **在线查看/编辑技能文件（SKILL.md + 脚本）**
- **技能导出为 ZIP 包**
- **技能批量上传**
- AI 生成技能 / ZIP 上传导入

### 知识库
- 支持 .md/.txt/.csv/.tsv/.json/.pdf/.docx 格式
- 分类管理（文档/问答/数据）
- 拖拽批量上传
- 内容预览

### UI/UX
- 6 套主题（light/dark/midnight/cyber/aurora/cosmos）
- AnimatePresence 页面切换动画
- **顶部导航栏（主导航 5 项 + 辅助导航 5 项，layoutId 动画指示器）**
- 会话重命名（双击编辑）+ **会话置顶**
- **会话批量删除**
- Modal 化删除确认
- **费用估算（非官方 API 本地定价表兜底，标注"~"估算标记）**
- 用量统计 + 预算预警
- **交互式向导流（Wizard Flow）：多选择题逐一展示，支持前后翻页、进度指示、汇总确认后才继续对话**
- **两阶段规划模式：用户决定是否进入规划模式，进入后沉浸式多维度选择面板，全部确认后再执行**
- **精致化 UI 细节：气泡圆角/阴影/hover 微交互、超薄滚动条、三点波浪连接动画、增强版空状态页**
- **快捷键面板（Cmd+/ 唤出）**

### 定时任务
- **周期性自动执行 Agent 任务（每 N 分钟/小时/每天/每周/每月/自定义 Cron）**
- **有状态/无状态模式（有状态保持同一会话，Agent 可记忆上次执行内容）**
- **执行完成桌面通知**
- **执行记录查看 + 跳转到对应会话**
- **手动触发执行**

### 长期记忆与上下文
- **跨会话长期记忆（memories 表，按智能体隔离，自动/手动添加，分类管理）**
- **记忆管理 UI（设置 > 长期记忆：查看/添加/删除/分类/清空）**
- **对话摘要数据模型（sessions.summary 字段，为后续自动摘要压缩做准备）**
- **会话文件夹数据模型（sessions.folder 字段）**
- **会话置顶（sessions.pinned 字段，置顶会话排序优先）**

### Agent 间对话（Project Nexus）
- **局域网 mDNS 自动发现（_lingxi._tcp 服务广播 + 10 秒扫描周期）**
- **手动建联 + PSK 密钥验证（双方交换共享密钥，后续通信 X-Nexus-Token 验证）**
- **NexusTokenAuth 中间件（/info、/connect-request、/connect-respond 免验证，其余需 X-Nexus-Token）**
- **Token 验证兼容 pending/pending_incoming/connected 三种状态**
- **一对一 Agent 自动对话（异步执行循环，支持 message/proposal/decision/handoff/close 5 种消息类型）**
- **流式实时对话（每端映射独立 Claude 会话，token 级流式 WS 推送，主聊天同款 UI）**
- **双向流式对话（跨实例 stream-token 转发，双方均可实时看到对方 Agent 思考和输出过程）**
- **持久会话（a2a_conversations.local_session_id 关联 sessions 表，跨轮次保持对话上下文）**
- **统一 Bubble 渲染（A2AAgentBubble 组件 + BlocksRenderer，完整 Markdown/代码高亮/思考块/工具块）**
- **己方/对方 Agent 视觉区分（不同颜色头像+标签+边框，紫色=对方，主题色=己方）**
- **右侧面板 Markdown 渲染（摘要用 ReactMarkdown + remarkGfm，决策用结构化卡片）**
- **实时观察模式 UI（发起方和接收方均可看到 Agent 流式输出）**
- **智能滚动（stick-to-bottom 逻辑：用户上滚后不强制拉回，新消息时自动回底部）**
- **人类介入（暂停/接管/终止 + handoff 自动暂停通知）**
- **对话摘要自动生成 + 人类审批流程**
- **接收方完整邀请流程（显示主题/目标/对方Agent，选择己方Agent，接受/拒绝）**
- **Agent 对外设置（公开开关/能力标签/授权级别/禁止透露信息）**
- **统一 Nexus 仪表盘（合并发现/联系人/对话三合一）**
- **网络与协作全局设置（对外可见/昵称/端口）**

### 平台能力
- 多模型多供应商接入
- MCP 工具管理（stdio/SSE/HTTP）+ 配置导出
- IM 集成（企业微信/钉钉/飞书）
- **Windows 构建支持（NSIS 安装包 + Portable）**
- **OpenAI 兼容模型技能识别增强（自动注入已安装技能清单到 system prompt）**
- **防死循环保护（禁止调用 Cursor 专有工具，避免 tool_use 循环）**
- **冷启动优化（Splash 页面立即显示，后端就绪后无缝切换）**
- **本地离线语音识别（内置 whisper.cpp + ggml-base 模型，无需网络）**
- **定时任务调度修复（Go 侧时间比较，15 秒检查间隔，时区兼容）**
- **后台任务管理（GET /api/tasks、DELETE /api/tasks/:id）**
- **挂起任务机制（Agent 向用户请求信息时挂起，前端提交后继续）**
- **自动更新（electron-updater + generic CDN provider，启动后自动检查，支持下载进度+重启安装，设置页手动检查）**
