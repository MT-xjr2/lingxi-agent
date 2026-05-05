# 灵犀 AI Agent：Project Nexus —— 局域网 Agent-to-Agent 实时对话网络

> 本文是灵犀 AI Agent 系列专栏的第三篇，将深入剖析灵犀最具创新性的功能——Project Nexus。这是一个完整的 Agent-to-Agent 通信网络：局域网 mDNS 自动发现、PSK 密钥建联、双向流式对话、人类监督、结果审批。从协议设计到前端渲染，完整展现一个 Agent 间通信系统的构建过程。
>
> GitHub 地址：[https://github.com/MT-xjr2/lingxi](https://github.com/MT-xjr2/lingxi)

---

## 一、为什么需要 Agent-to-Agent 对话？

在传统的 AI 应用中，每个用户和自己的 AI 对话，AI 之间互不相通。但在真实的工作场景中，很多任务需要**多方协作**：

- 产品经理的 AI 需要和开发团队的 AI 讨论技术方案
- 设计师的 AI 需要和前端的 AI 确认实现可行性
- 法务的 AI 需要和业务的 AI 审核合同条款

如果这些 AI 能够**直接对话**，省去人类的"传话"环节，效率将大幅提升。

灵犀的 **Project Nexus** 正是为此而生——让局域网内多台灵犀实例的 Agent 可以自动发现、建立信任、发起对话、实时流式交互，人类随时可以介入监督。

```
┌──────────────┐                      ┌──────────────┐
│  灵犀实例 A   │  ← 双向流式对话 →    │  灵犀实例 B   │
│              │                      │              │
│ ┌──────────┐ │    mDNS 自动发现     │ ┌──────────┐ │
│ │ 代码审查员│ │ ◄──────────────────► │ │ 架构师   │ │
│ │  Agent   │ │    PSK 密钥建联      │ │  Agent   │ │
│ └──────────┘ │                      │ └──────────┘ │
│              │    token 级流式      │              │
│  👤 人类 A   │  ◄──────────────────► │  👤 人类 B   │
│  （观察/介入）│                      │  （观察/介入）│
└──────────────┘                      └──────────────┘
```

---

## 二、mDNS 自动发现：局域网的"雷达"

Project Nexus 的第一步是**发现**——同一局域网下的灵犀实例需要能互相感知到对方的存在。

### 2.1 mDNS 协议选型

灵犀选择了 **mDNS（Multicast DNS）** 作为发现协议，使用 `_lingxi._tcp` 服务类型。这个选择有几个关键优势：

1. **零配置**——不需要中心服务器，不需要手动输入 IP 地址
2. **局域网限定**——mDNS 报文不会穿越路由器，天然限制在本地网络
3. **实时性好**——灵犀每 10 秒扫描一次，新实例上线后几秒内即可被发现

```go
// nexus/discovery.go
const (
    serviceType  = "_lingxi._tcp"  // 服务类型
    scanInterval = 10 * time.Second // 扫描间隔
    peerTimeout  = 60 * time.Second // 超时清理
)
```

### 2.2 服务注册

当灵犀启动时，如果用户开启了"对外可见"，会通过 mDNS 注册自己的服务：

```go
func (d *Discovery) startServer(settings *db.NexusSettings) {
    info := []string{
        "id=" + d.instanceID,    // 唯一实例 ID
        "nick=" + nickname,       // 显示昵称
    }

    service, _ := mdns.NewMDNSService(
        d.instanceID,  // 实例名
        serviceType,   // _lingxi._tcp
        "",            // 域名（默认 .local）
        "",            // 主机名（自动）
        settings.ListenPort,  // 端口
        nil,           // IP（自动）
        info,          // TXT 记录
    )
    
    d.server, _ = mdns.NewServer(&mdns.Config{Zone: service})
}
```

### 2.3 服务扫描

扫描逻辑运行在独立的 goroutine 中，每 10 秒执行一次：

```go
func (d *Discovery) scan() {
    entriesCh := make(chan *mdns.ServiceEntry, 16)
    
    go func() {
        for entry := range entriesCh {
            // 解析 TXT 记录
            peerID := ""
            nickname := ""
            for _, field := range entry.InfoFields {
                if strings.HasPrefix(field, "id=") {
                    peerID = field[3:]
                }
                if strings.HasPrefix(field, "nick=") {
                    nickname = field[5:]
                }
            }
            
            // 跳过自己
            if peerID == d.instanceID {
                continue
            }
            
            // 获取对方的公开 Agent 信息
            agentsJSON := fetchRemoteAgents(host, entry.Port)
            
            // 更新到数据库
            db.UpsertNexusPeer(&db.NexusPeer{
                ID:         peerID,
                Nickname:   nickname,
                Host:       host,
                Port:       entry.Port,
                AgentsJSON: agentsJSON,
            })
        }
    }()
    
    // 发送 mDNS 查询
    params := mdns.DefaultParams(serviceType)
    params.Timeout = 3 * time.Second
    mdns.Query(params)
    
    // 清理超时的 peer
    db.CleanStalePeers(time.Now().Add(-peerTimeout))
}
```

发现对方后，灵犀还会调用对方的 `/api/nexus/info` 接口，获取对方公开的 Agent 列表。这样在发起对话时，用户可以看到对方有哪些 Agent 可以交谈。

<!-- 截图占位：Nexus 发现页面，显示附近的灵犀实例 -->
![Nexus 发现页面](images/nexus发现.png)

---

## 三、建联机制：PSK 密钥验证

发现只是第一步。两个灵犀实例要对话，必须先建立信任关系——灵犀使用 **PSK（Pre-Shared Key）密钥验证** 机制。

### 3.1 建联流程

```
实例 A                                  实例 B
  │                                        │
  │  1. POST /api/nexus/connect-request    │
  │  {peer_id, nickname, shared_secret}    │
  │ ─────────────────────────────────────► │
  │                                        │
  │        2. 用户 B 看到建联请求           │
  │           选择接受/拒绝                 │
  │                                        │
  │  3. POST /api/nexus/connect-respond    │
  │  {peer_id, accepted, shared_secret}    │
  │ ◄───────────────────────────────────── │
  │                                        │
  │  4. 双方状态变为 connected              │
  │     后续通信使用 X-Nexus-Token 验证     │
  │                                        │
```

### 3.2 Token 验证中间件

建联完成后，双方交换了共享密钥（shared_secret）。后续所有跨实例的 HTTP 请求，都需要在 Header 中携带 `X-Nexus-Token`。

```go
// handler/nexus.go
func NexusTokenAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        
        // 免验证路径：info、connect-request、connect-respond
        if strings.HasSuffix(path, "/info") ||
           strings.HasSuffix(path, "/connect-request") ||
           strings.HasSuffix(path, "/connect-respond") {
            c.Next()
            return
        }
        
        token := c.GetHeader("X-Nexus-Token")
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "missing token"})
            return
        }
        
        // 验证 token 是否属于已建联的 contact
        // 兼容 pending/pending_incoming/connected 三种状态
        contact := db.FindContactByToken(token)
        if contact == nil {
            c.AbortWithStatusJSON(403, gin.H{"error": "invalid token"})
            return
        }
        
        c.Set("nexus_contact", contact)
        c.Next()
    }
}
```

注意一个精巧的设计：`/info`、`/connect-request`、`/connect-respond` 三个接口是免验证的。因为这三个接口的调用发生在建联之前——此时双方还没有交换密钥。

---

## 四、对话引擎：双向流式的实现

建联完成后，用户可以从实例 A 发起一场 Agent 对话。灵犀的对话引擎支持 5 种消息类型：

| 类型 | 说明 | 触发条件 |
|------|------|---------|
| `message` | 普通消息 | 默认类型 |
| `proposal` | 提案 | Agent 提出需要确认的方案 |
| `decision` | 决策 | Agent 做出明确的决定 |
| `handoff` | 交接 | Agent 认为需要人类介入 |
| `close` | 结束 | Agent 认为对话目标已达成 |

### 4.1 对话发起流程

```
用户 A 在 Nexus 页面选择对方的 Agent
     │
     ▼
填写对话配置：主题、目标、最大轮次、是否需要审批
     │
     ▼
POST /api/a2a-conversations（创建本地记录）
     │
     ▼
POST 对方的 /api/nexus/conversation/request
     │
     ▼
对方用户看到邀请通知，选择己方 Agent，接受/拒绝
     │
     ▼ 接受
POST 回 /api/nexus/conversation/accept
     │
     ▼
RunConversation 引擎启动！
```

### 4.2 RunConversation：引擎核心

`RunConversation` 是对话引擎的核心函数。它的工作流程如下：

```go
func RunConversation(convID int64, sessionID int64, ...) {
    // 1. 加锁——同一对话不能并行执行
    mu := getConvMutex(convID)
    mu.Lock()
    defer mu.Unlock()
    
    // 2. 准备暂停通道——人类可以随时暂停
    pauseCh := make(chan struct{}, 1)
    pausedConvs.Store(convID, pauseCh)
    defer pausedConvs.Delete(convID)
    
    // 3. 构建流式转发器——将本地 Agent 的输出转发给对方
    forwarder := buildStreamForwarder(conv)
    
    // 4. 构建首条消息（含对话主题、目标、安全约束）
    firstMessage := buildA2AFirstMessage(conv, agent)
    
    // 5. 调用流式 Runner——获取 Agent 回复
    reply, err := streamRunner(sessionID, firstMessage, 
                               conv.LocalAgentID, forwarder)
    
    // 6. 解析回复类型（message/proposal/decision/handoff/close）
    msgType, content, structured := parseAgentReply(reply)
    
    // 7. 发送到远端
    sendToRemote(remoteURL, convID, "remote", agentName, 
                 msgType, content, structured, token)
}
```

### 4.3 双向流式转发

这是 Project Nexus 最有技术含量的部分。每当本地 Agent 产生一个 token（文本或思考过程），灵犀不仅通过 WebSocket 推送给本地前端，还要**实时转发给对方**。

```go
// 构建流式转发器
func buildStreamForwarder(conv *db.A2AConversation) StreamForwarder {
    contact, _ := db.GetNexusContactByPeerID(conv.RemotePeerID)
    remoteURL := fmt.Sprintf("http://%s/api/nexus/conversation/stream-token",
        net.JoinHostPort(contact.Host, fmt.Sprintf("%d", contact.Port)))
    token := contact.SharedSecret
    
    return func(event, data string) {
        payload := map[string]interface{}{
            "conversation_id": convID,
            "event":           event,  // "text" 或 "thinking"
            "data":            data,   // token 内容
        }
        // 异步发送，不阻塞主流程
        go httpPost(remoteURL, payload, token)
    }
}
```

对方实例收到 `stream-token` 后，通过 WebSocket 广播 `a2a_remote_stream` 事件，前端接收后更新 `a2aRemoteLiveBlocks` 状态——实现了对方 Agent 输出的实时渲染。

```
本地 Agent 产生 token "你好"
     │
     ├──► 本地 WebSocket → 前端渲染（己方 Agent 输出）
     │
     └──► HTTP POST → 对方 /api/nexus/conversation/stream-token
                         │
                         └──► 对方 WebSocket → 对方前端渲染（对方视角下的远端 Agent 输出）
```

### 4.4 持久会话

A2A 对话不是一次性的。灵犀为每个对话创建了独立的 session，关联到 `sessions` 表：

```
a2a_conversations 表
├── local_session_id  ──► sessions 表的 id
├── topic             ──► 对话主题
├── goal              ──► 对话目标
├── current_round     ──► 当前轮次
└── max_rounds        ──► 最大轮次
```

这意味着 Agent 在对话中的上下文是**跨轮次保持**的。它记得之前说过什么，可以在后续轮次中引用之前的讨论内容。

---

## 五、前端渲染：统一的 Bubble 体验

A2A 对话的前端渲染与主聊天使用相同的消息渲染组件（BlocksRenderer），这意味着 Agent 间对话完整支持：

- Markdown 渲染
- 代码块语法高亮
- 思考过程折叠块
- 工具调用状态块

### 5.1 视觉区分

己方和对方的 Agent 通过颜色区分：

- **主题色** = 己方 Agent（与应用主题一致）
- **紫色** = 对方 Agent

每条消息气泡都有 Agent 名称标签和颜色头像，确保用户一眼就能分清消息来源。

### 5.2 Zustand 状态设计

A2A 对话在 Zustand Store 中有独立的状态切片，与主聊天完全隔离：

```javascript
// A2A 对话状态（独立于主聊天）
activeA2ASessionId: null,
activeA2AConvId: null,
a2aLiveBlocks: [],           // 己方 Agent 流式输出
a2aIsStreaming: false,
a2aRemoteLiveBlocks: [],     // 对方 Agent 流式输出
a2aRemoteIsStreaming: false,
a2aMessages: [],             // 历史消息
```

WebSocket 事件路由逻辑会根据 `sessionId` 判断：如果匹配 `activeA2ASessionId`，事件会路由到 A2A 状态切片；如果匹配 `activeSessionId`，事件会路由到主聊天状态切片。

```javascript
// WS 事件路由
handleWSEvent: (msg) => {
    const { event, sessionId } = msg;
    
    // 远端 Agent 流式 token 转发（广播事件，无 sessionId）
    if (event === 'a2a_remote_stream') {
        // → 更新 a2aRemoteLiveBlocks
        return;
    }
    
    // A2A 会话的流式事件
    if (sessionId === state.activeA2ASessionId) {
        // → 更新 a2aLiveBlocks
        return;
    }
    
    // 主聊天的流式事件
    if (sessionId === state.activeSessionId) {
        // → 更新 liveBlocks
    }
}
```

<!-- 截图占位：A2A 对话界面，展示双方 Agent 的实时对话 -->
![A2A 对话界面](images/a2a对话.png)

---

## 六、人类监督：暂停、接管、终止

AI 间的自动对话虽然高效，但必须保留人类的控制权。灵犀实现了完整的人类监督机制。

### 6.1 暂停

用户随时可以暂停对话。暂停信号通过 Go channel 传递：

```go
func PauseConversation(convID int64) {
    if ch, ok := pausedConvs.Load(convID); ok {
        select {
        case ch.(chan struct{}) <- struct{}{}:
        default:
        }
    }
}
```

在对话循环中，每轮开始前都会检查暂停信号。

### 6.2 接管

用户可以"接管"对话——此时 Agent 停止自动回复，人类手动输入消息。对话从 Agent 自动模式切换为人类手动模式。

### 6.3 终止

用户可以随时终止对话。终止信号会同时发送给对方，确保双方都停止。

### 6.4 Handoff

Agent 自身也可以请求人类介入。当 Agent 判断某个决策超出自己的能力范围时，会发送 `[HANDOFF]` 标记，灵犀会自动暂停对话并弹出桌面通知。

```go
if msgType == "handoff" {
    db.UpdateA2AConversationStatus(convID, "paused")
    broadcast("a2a_handoff", ...)
    broadcast("desktop_notify", `{
        "title": "Agent 请求人类介入",
        "body": "..."
    }`)
}
```

---

## 七、对话摘要与审批

当对话结束（Agent 发送 `[CLOSE]` 标记）时，灵犀会自动生成对话摘要。

### 7.1 摘要生成

```go
func generateSummary(convID int64) string {
    messages, _ := db.ListA2AMessages(convID)
    conv, _ := db.GetA2AConversation(convID)
    
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("对话主题：%s\n", conv.Topic))
    sb.WriteString(fmt.Sprintf("对话目标：%s\n", conv.Goal))
    sb.WriteString(fmt.Sprintf("总轮次：%d\n\n", conv.CurrentRound))
    
    // 提取关键决策
    for _, m := range messages {
        if m.MsgType == "decision" || m.MsgType == "close" {
            sb.WriteString(fmt.Sprintf("【%s】%s\n", m.MsgType, m.Content))
        }
    }
    return sb.String()
}
```

### 7.2 审批流程

如果对话创建时设置了"需要审批"，对话完成后不会立即标记为完成，而是进入 `pending_approval` 状态。用户需要审阅 Agent 的决策，确认或修改后才能最终确认。

```
Agent 对话完成
     │
     ▼
需要审批？─── No ──► 直接标记为 completed
     │
     │ Yes
     ▼
状态变为 pending_approval
     │
     ▼
用户审阅摘要和决策
     │
     ├── 批准 → approved
     └── 驳回 → rejected（可修改后重新提交）
```

---

## 八、安全设计：Agent 对外设置

每个 Agent 参与 Nexus 通信时，有独立的安全配置：

| 配置项 | 说明 |
|--------|------|
| **公开开关** | 是否在 mDNS 中广播此 Agent |
| **能力标签** | 标记 Agent 的专长（"代码审查"、"产品设计"等） |
| **授权级别** | 控制 Agent 能做什么（只读/建议/执行） |
| **禁止透露** | 安全约束——指定 Agent 绝对不能透露的信息 |

```go
// 构建 A2A 首条消息时注入安全约束
func buildA2AFirstMessage(conv *db.A2AConversation, agent *db.Agent) string {
    nexusConfig, _ := db.GetAgentNexusConfig(conv.LocalAgentID)
    forbidden := ""
    if nexusConfig != nil && nexusConfig.ForbiddenInfo != "" {
        forbidden = fmt.Sprintf(
            "\n【安全约束】绝对不可透露以下信息：%s\n", 
            nexusConfig.ForbiddenInfo)
    }
    // ...
}
```

这意味着即使 Agent 在与外部 Agent 对话时，也会遵守预设的安全边界——不会泄露敏感业务信息。

---

## 九、统一 Nexus 仪表盘

灵犀将发现、联系人、对话三个功能整合到了统一的 Nexus 仪表盘中：

```
┌─────────────────────────────────────────────────┐
│  Nexus 仪表盘                                    │
├───────────┬──────────────┬──────────────────────┤
│ 附近实例   │ 已建联联系人  │ 进行中/已完成的对话   │
│           │              │                      │
│ 🟢 Mac A  │ 📍 张三的灵犀 │ 🔵 技术方案讨论       │
│ 🟢 Mac B  │ 📍 李四的灵犀 │ 🟡 合同条款审核       │
│           │              │ ✅ 需求分析         │
│ [建联]    │ [发起对话]    │ [查看详情]           │
└───────────┴──────────────┴──────────────────────┘
```

<!-- 截图占位：Nexus 仪表盘界面 -->
![Nexus 仪表盘](images/nexus仪表盘.png)

---

## 十、技术挑战与解决方案

### 10.1 并发安全

同一个对话不能被多个 goroutine 同时处理。灵犀为每个对话维护一个独立的 mutex：

```go
var convMutexes sync.Map // convID -> *sync.Mutex

func getConvMutex(convID int64) *sync.Mutex {
    v, _ := convMutexes.LoadOrStore(convID, &sync.Mutex{})
    return v.(*sync.Mutex)
}
```

### 10.2 消息重试

跨实例的 HTTP 通信可能因网络波动失败。灵犀实现了自动重试机制：

```go
func sendToRemote(url string, ...) error {
    for attempt := 0; attempt < 3; attempt++ {
        _, err := httpPost(url, payload, token)
        if err == nil {
            return nil
        }
        // 指数退避
        time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
    }
    return lastErr
}
```

### 10.3 WS 事件路由

当同时存在主聊天和 A2A 对话时，WebSocket 事件需要正确路由。灵犀通过 `sessionId` 字段区分事件归属，确保不同对话的流式数据不会串台。

---

## 十一、总结

Project Nexus 是灵犀最具创新性和技术挑战的功能模块。它实现了一个完整的 Agent-to-Agent 通信网络：

1. **mDNS 自动发现**——零配置，局域网内灵犀实例自动可见
2. **PSK 建联**——共享密钥验证，建立互信关系
3. **双向流式对话**——token 级实时转发，双方同时观察 Agent 输出
4. **5 种消息类型**——message/proposal/decision/handoff/close，覆盖协作场景
5. **人类监督**——暂停/接管/终止，人类始终拥有最终控制权
6. **安全约束**——Agent 对外设置，防止敏感信息泄露
7. **审批流程**——对话结果需要人类确认后才能生效

在下一篇文章中，我们将回到日常使用场景，深入探讨灵犀的**对话体验设计**——多模态输入、RAG 引用可视化、规划模式、流式思考链展示等功能的实现细节。

---

> **灵犀** —— 让 AI 成为你的工作伙伴，而不只是聊天对象。
>
> GitHub：[https://github.com/MT-xjr2/lingxi](https://github.com/MT-xjr2/lingxi)
>
> 如果觉得项目有价值，欢迎 Star 支持！
