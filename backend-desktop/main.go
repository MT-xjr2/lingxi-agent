package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lingxi-agent/config"
	"lingxi-agent/connector"
	"lingxi-agent/db"
	"lingxi-agent/handler"
	"lingxi-agent/nexus"
	"lingxi-agent/scheduler"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Get()
	db.Init()
	db.SeedDingTalkOAuth("ding984jgwjcylgoyb2y", "-1O1oF49HSvkPvcFWq2cIVmGLX3hvEj35QWdSI1P_16Vc9CKjhLgqq4vuwF7oQye")
	go handler.BootstrapInstalledSkills()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	dist := cfg.Server.FrontendDist
	r.Static("/assets", dist+"/assets")
	r.StaticFile("/favicon.svg", dist+"/favicon.svg")
	r.StaticFile("/icons.svg", dist+"/icons.svg")
	r.StaticFile("/logo.png", dist+"/logo.png")
	r.StaticFile("/logo.jpg", dist+"/logo.jpg")
	r.StaticFile("/favicon.ico", dist+"/favicon.ico")

	api := r.Group("/api")

	// 用户上传图片静态目录
	uploadsDir := os.Getenv("UPLOADS_PATH")
	if uploadsDir == "" {
		uploadsDir = filepath.Join(os.TempDir(), "lingxi-uploads")
	}
	os.MkdirAll(uploadsDir, 0755)
	api.Static("/uploads", uploadsDir)

	// 初始化 IM 连接器管理器（在路由组创建后立即初始化，wecom 会注册子路由）
	connector.InitManager(api)
	connector.SetClaudeRunner(handler.RunClaudeSync)
	go connector.GlobalManager.LoadFromDB()

	// 健康检查
	api.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// ── 认证（SSO + 游客）────────────────────────────────────────
	api.GET("/auth/status", handler.HasUser)
	api.GET("/auth/me", handler.GetCurrentUser)
	api.POST("/auth/guest", handler.GuestLogin)
	api.POST("/auth/oauth/callback", handler.OAuthCallback)
	api.POST("/auth/logout", handler.Logout)
	api.GET("/auth/oauth-configs", handler.ListOAuthConfigs)
	api.POST("/auth/oauth-configs", handler.SaveOAuthConfig)

	// 单机模式：无认证，所有接口直接可用
	api.GET("/sessions", handler.ListSessions)
	api.POST("/sessions", handler.CreateSession)
	api.PATCH("/sessions/:id", handler.UpdateSession)
	api.DELETE("/sessions/:id", handler.DeleteSession)
	api.POST("/sessions/batch-delete", handler.BatchDeleteSessions)
	api.GET("/sessions/:id/messages", handler.ListMessages)
	api.GET("/messages/search", handler.SearchMessages)
	api.PUT("/messages/:id", handler.UpdateMessage)
	api.POST("/messages/:id/feedback", handler.SetMessageFeedback)

	api.POST("/chat", handler.Chat)
	api.POST("/chat/batch", handler.BatchChat)
	api.POST("/chat/abort", handler.AbortChat)
	api.GET("/ws", handler.WsHandler)

	// 挂起任务
	api.GET("/sessions/:id/pending", handler.GetPendingTask)
	api.DELETE("/sessions/:id/pending", handler.ClearPendingTask)

	// 后台任务
	api.GET("/tasks", handler.ListTasks)
	api.DELETE("/tasks/:id", handler.DeleteTask)

	api.GET("/skills", handler.ListSkills)
	api.POST("/skills/upload", handler.UploadSkill)
	api.POST("/skills/batch-upload", handler.BatchUploadSkill)
	api.POST("/skills/generate/stream", handler.GenerateSkillStream)
	api.POST("/skills/generate/confirm", handler.ConfirmGeneratedSkill)
	api.GET("/skills/marketplace", handler.MarketplaceSearch)
	api.GET("/skills/marketplace/categories", handler.MarketplaceCategories)
	api.GET("/skills/marketplace/:namespace/:slug", handler.MarketplaceGetSkill)
	api.POST("/skills/marketplace/install", handler.MarketplaceInstall)
	api.GET("/skills/:id/content", handler.GetSkillContent)
	api.PUT("/skills/:id/content", handler.UpdateSkillContent)
	api.GET("/skills/:id/export", handler.ExportSkill)
	api.POST("/skills/:id/install", handler.InstallSkill)
	api.POST("/skills/:id/uninstall", handler.UninstallSkill)
	api.DELETE("/skills/:id", handler.DeleteSkill)
	api.POST("/skills/batch-delete", handler.BatchDeleteSkills)

	// 知识库
	api.GET("/knowledge", handler.ListKnowledge)
	api.POST("/knowledge", handler.UploadKnowledge)
	api.PUT("/knowledge/:id", handler.UpdateKnowledge)
	api.DELETE("/knowledge/:id", handler.DeleteKnowledge)
	api.POST("/knowledge/batch-delete", handler.BatchDeleteKnowledge)
	api.GET("/knowledge/:id/preview", handler.PreviewKnowledge)
	api.PATCH("/knowledge/:id/category", handler.UpdateKnowledgeItemCategory)
	api.GET("/knowledge/categories", handler.ListKnowledgeCategories)
	api.POST("/knowledge/categories", handler.CreateKnowledgeCategory)
	api.DELETE("/knowledge/categories/:id", handler.DeleteKnowledgeCategory)

	// IM 连接器管理
	api.GET("/im-connectors", handler.ListIMConnectors)
	api.POST("/im-connectors", handler.UpsertIMConnector)
	api.PUT("/im-connectors/:id/enable", handler.EnableIMConnector)
	api.PUT("/im-connectors/:id/disable", handler.DisableIMConnector)
	api.DELETE("/im-connectors/:id", handler.DeleteIMConnector)

	// 模型 / 接入点 / AKSK 档案
	api.GET("/providers", handler.ListProviders)
	api.GET("/api-profiles", handler.ListAPIProfiles)
	api.POST("/api-profiles", handler.UpsertAPIProfile)
	api.DELETE("/api-profiles/:id", handler.DeleteAPIProfile)
	api.POST("/api-profiles/:id/activate", handler.ActivateAPIProfile)
	api.POST("/api-profiles/:id/test", handler.TestAPIProfile)

	// 用量
	api.GET("/usage", handler.GetUsage)
	api.GET("/usage/quota", handler.GetUsageQuota)

	// MCP 服务器
	api.GET("/mcp", handler.ListMCPServers)
	api.POST("/mcp", handler.UpsertMCPServer)
	api.DELETE("/mcp/:id", handler.DeleteMCPServer)
	api.POST("/mcp/:id/toggle", handler.ToggleMCPServer)
	api.GET("/mcp/export", handler.ExportMCPConfig)

	// 智能体工厂
	api.GET("/agents", handler.ListAgents)
	api.GET("/agents/:id", handler.GetAgent)
	api.POST("/agents", handler.UpsertAgent)
	api.DELETE("/agents/:id", handler.DeleteAgent)
	api.POST("/sessions/:id/agent", handler.SetSessionAgent)

	// 定时任务
	api.GET("/scheduled-tasks", handler.ListScheduledTasks)
	api.POST("/scheduled-tasks", handler.CreateScheduledTask)
	api.PUT("/scheduled-tasks/:id", handler.UpdateScheduledTask)
	api.DELETE("/scheduled-tasks/:id", handler.DeleteScheduledTask)
	api.POST("/scheduled-tasks/:id/toggle", handler.ToggleScheduledTask)
	api.POST("/scheduled-tasks/:id/run", handler.TriggerScheduledTask)
	api.GET("/scheduled-tasks/:id/runs", handler.ListScheduledTaskRuns)

	// 长期记忆
	api.GET("/memories", handler.ListMemories)
	api.POST("/memories", handler.CreateMemory)
	api.DELETE("/memories/clear", handler.ClearMemories)
	api.DELETE("/memories/:id", handler.DeleteMemory)
	api.POST("/messages/:id/pin", handler.ToggleMessagePin)

	// 语音识别（转发到 OpenAI 兼容 Whisper API）
	api.POST("/transcribe", handler.TranscribeAudio)

	// ── Project Nexus: Agent-to-Agent Communication ──────────────
	nexusAPI := api.Group("/nexus")
	nexusAPI.GET("/info", handler.NexusInfo)
	nexusAPI.POST("/conversation/invite", handler.NexusReceiveConvInvite)
	nexusAPI.POST("/conversation/accept", handler.NexusReceiveConvAccept)
	nexusAPI.POST("/conversation/reject", handler.NexusReceiveConvReject)
	nexusAPI.POST("/conversation/message", handler.NexusReceiveMessage)
	nexusAPI.POST("/conversation/pause", handler.NexusReceivePause)
	nexusAPI.POST("/conversation/terminate", handler.NexusReceiveTerminate)
	nexusAPI.POST("/conversation/stream-token", handler.NexusReceiveStreamToken)
	nexusAPI.GET("/settings", handler.GetNexusSettings)
	nexusAPI.PUT("/settings", handler.UpdateNexusSettings)

	api.GET("/peers", handler.ListPeers)
	// ── 广域网 (WAN) ───────────────────────────────────────────
	api.GET("/wan/peers", handler.ListWANPeers)
	api.GET("/wan/status", handler.WANStatus)

	api.POST("/a2a-conversations", handler.CreateA2AConversation)
	api.GET("/a2a-conversations", handler.ListA2AConversations)
	api.GET("/a2a-conversations/:id", handler.GetA2AConversation)
	api.POST("/a2a-conversations/:id/pause", handler.PauseA2AConversation)
	api.POST("/a2a-conversations/:id/takeover", handler.TakeoverA2AConversation)
	api.POST("/a2a-conversations/:id/terminate", handler.TerminateA2AConversation)
	api.POST("/a2a-conversations/:id/approve", handler.ApproveA2AConversation)
	api.POST("/a2a-conversations/:id/accept-remote", handler.AcceptRemoteConversation)
	api.POST("/a2a-conversations/:id/reject-remote", handler.RejectRemoteConversation)
	api.DELETE("/a2a-conversations/:id", handler.DeleteA2AConversation)
	api.GET("/agents/:id/nexus-config", handler.GetAgentNexusConfig)
	api.PUT("/agents/:id/nexus-config", handler.UpsertAgentNexusConfig)

	// Electron 启动时下发激活档案明文 token
	api.POST("/runtime/active-secret", handler.SetActiveSecret)

	// Bridge 路由层（OpenAI ↔ Anthropic 转换代理，由 supermemoryai/llm-bridge 实现）
	api.GET("/router/status", handler.GetRouterStatus)
	api.POST("/router/stop", handler.StopRouter)

	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(dist + "/index.html")
	})

	// 启动 Nexus Agent-to-Agent 通信服务
	nexus.Init(handler.RunA2AStreamingTurn, handler.CreateA2ASession, handler.BroadcastWSEvent)
	nexus.SetRelayHandler(buildRelayHandler(r))
	nexus.Global.Start()

	// 启动定时任务调度器
	scheduler.Init(handler.RunClaudeSync, func(taskName, summary string) {
		body := summary
		sessionID := ""
		if idx := strings.LastIndex(summary, "|session_id:"); idx >= 0 {
			body = summary[:idx]
			sessionID = summary[idx+len("|session_id:"):]
		}
		m := map[string]string{
			"title": "定时任务 — " + taskName,
			"body":  body,
		}
		if sessionID != "" {
			m["session_id"] = sessionID
		}
		payload, _ := json.Marshal(m)
		handler.BroadcastWSEvent("desktop_notify", string(payload))
	})
	scheduler.Start()

	log.Printf("[main] desktop mode, listening on :%s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("[main] run error: %v", err)
	}
}

// buildRelayHandler 构建通过信令服务器中继的消息处理函数
// 将 WAN relay 消息转换为对本地 /api/nexus/* 端点的直接调用
func buildRelayHandler(r *gin.Engine) nexus.RelayHandler {
	return func(fromPeerID string, path string, payload json.RawMessage) interface{} {
		fullPath := "/api/nexus" + path
		log.Printf("[relay] received from %s: path=%s payloadLen=%d", fromPeerID, fullPath, len(payload))

		body := payload
		if body == nil {
			body = []byte("{}")
		}

		summary := string(body)
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		log.Printf("[relay] payload: %s", summary)

		w := &relayResponseWriter{headers: http.Header{}, body: []byte{}, status: 200}
		req, err := http.NewRequest("POST", fullPath, strings.NewReader(string(body)))
		if err != nil {
			log.Printf("[relay] create request error: %v", err)
			return map[string]interface{}{"error": err.Error()}
		}
		req.Header.Set("Content-Type", "application/json")

		r.ServeHTTP(w, req)
		log.Printf("[relay] handled %s -> status=%d bodyLen=%d body=%s",
			fullPath, w.status, len(w.body), truncateStr(string(w.body), 200))

		if w.status >= 400 {
			log.Printf("[relay] ERROR from=%s path=%s status=%d body=%s", fromPeerID, fullPath, w.status, string(w.body))
			return map[string]interface{}{"error": string(w.body), "status": w.status}
		}

		var result interface{}
		if len(w.body) > 0 {
			json.Unmarshal(w.body, &result)
		}
		return result
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// relayResponseWriter 实现 http.ResponseWriter 接口，用于捕获中继调用的响应
type relayResponseWriter struct {
	headers http.Header
	body    []byte
	status  int
}

func (w *relayResponseWriter) Header() http.Header { return w.headers }
func (w *relayResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}
func (w *relayResponseWriter) WriteHeader(code int) { w.status = code }
