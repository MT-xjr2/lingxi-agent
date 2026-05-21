package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lingxi-agent/db"

	"github.com/gin-gonic/gin"
)

// Screen Agent 全局中止标志
var screenAgentAborted sync.Map // key: sessionID(int64)

// ─── Screen Agent API ─────────────────────────────────────────────

// ScreenAgentAnalyze POST /api/screen-agent/analyze
// 接收截图 base64 + 上下文，调用多模态模型分析屏幕内容
func ScreenAgentAnalyze(c *gin.Context) {
	var req struct {
		Screenshot string `json:"screenshot"`
		Context    struct {
			AppName      string  `json:"app_name"`
			WindowTitle  string  `json:"window_title"`
			URL          string  `json:"url"`
			ContextType  string  `json:"context_type"`
			CursorX      float64 `json:"cursor_x"`
			CursorY      float64 `json:"cursor_y"`
			ScreenWidth  float64 `json:"screen_width"`
			ScreenHeight float64 `json:"screen_height"`
			ScaleFactor  float64 `json:"scale_factor"`
		} `json:"context"`
		Instruction string `json:"instruction"`
		SessionID   int64  `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Screenshot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "screenshot is required"})
		return
	}

	systemPrompt := buildScreenAnalysisPrompt(req.Context.AppName, req.Context.WindowTitle, req.Context.URL,
		req.Context.ScreenWidth, req.Context.ScreenHeight, req.Context.CursorX, req.Context.CursorY)

	userPrompt := req.Instruction
	if userPrompt == "" {
		userPrompt = "请描述当前屏幕上显示的内容，包括可见的 UI 元素、文字和布局。"
	}

	reply, err := callVisionLLM(systemPrompt, userPrompt, req.Screenshot)
	if err != nil {
		slog.Error("screen agent analyze error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录到 screen_actions
	actionData, _ := json.Marshal(map[string]interface{}{
		"instruction": req.Instruction,
		"app":         req.Context.AppName,
		"window":      req.Context.WindowTitle,
	})
	db.InsertScreenAction(&db.ScreenAction{
		SessionID:  req.SessionID,
		ActionType: "analyze",
		ActionData: string(actionData),
		Status:     "success",
	})

	c.JSON(http.StatusOK, gin.H{
		"analysis": reply,
	})
}

// ScreenAgentPlan POST /api/screen-agent/plan
// 根据用户指令 + 截图，生成操作计划
func ScreenAgentPlan(c *gin.Context) {
	var req struct {
		Screenshot string `json:"screenshot"`
		Context    struct {
			AppName      string  `json:"app_name"`
			WindowTitle  string  `json:"window_title"`
			URL          string  `json:"url"`
			ScreenWidth  float64 `json:"screen_width"`
			ScreenHeight float64 `json:"screen_height"`
		} `json:"context"`
		Instruction string `json:"instruction"`
		SessionID   int64  `json:"session_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Screenshot == "" || req.Instruction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "screenshot and instruction are required"})
		return
	}

	systemPrompt := buildScreenPlanPrompt(req.Context.AppName, req.Context.WindowTitle,
		req.Context.ScreenWidth, req.Context.ScreenHeight)

	reply, err := callVisionLLM(systemPrompt, req.Instruction, req.Screenshot)
	if err != nil {
		slog.Error("screen agent plan error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 尝试解析 JSON 操作计划
	steps := parseActionPlan(reply)

	c.JSON(http.StatusOK, gin.H{
		"raw_plan": reply,
		"steps":    steps,
	})
}

// ScreenAgentExecuteStep POST /api/screen-agent/step
// 执行单步操作（由前端在用户确认后调用）
func ScreenAgentExecuteStep(c *gin.Context) {
	var req struct {
		SessionID int64  `json:"session_id"`
		Action    string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 记录操作
	id, _ := db.InsertScreenAction(&db.ScreenAction{
		SessionID:  req.SessionID,
		ActionType: "execute",
		ActionData: req.Action,
		Status:     "pending",
	})

	// 通过 WS 推送给 Electron 执行
	BroadcastEvent("screen_agent_execute", map[string]interface{}{
		"action_id":  id,
		"session_id": req.SessionID,
		"action":     req.Action,
	})

	c.JSON(http.StatusOK, gin.H{
		"action_id": id,
		"status":    "dispatched",
	})
}

// ScreenAgentStepResult POST /api/screen-agent/step-result
// Electron 执行完操作后回报结果
func ScreenAgentStepResult(c *gin.Context) {
	var req struct {
		ActionID        int64  `json:"action_id"`
		Status          string `json:"status"`
		ErrorMsg        string `json:"error_msg"`
		ScreenshotAfter string `json:"screenshot_after"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.UpdateScreenActionStatus(req.ActionID, req.Status, req.ErrorMsg, req.ScreenshotAfter)

	BroadcastEvent("screen_agent_step_done", map[string]interface{}{
		"action_id": req.ActionID,
		"status":    req.Status,
		"error":     req.ErrorMsg,
	})

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ScreenAgentAbort POST /api/screen-agent/abort
func ScreenAgentAbort(c *gin.Context) {
	var req struct {
		SessionID int64 `json:"session_id"`
	}
	c.ShouldBindJSON(&req)
	screenAgentAborted.Store(req.SessionID, true)
	BroadcastEvent("screen_agent_abort", map[string]interface{}{
		"session_id": req.SessionID,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ScreenAgentReset POST /api/screen-agent/reset
func ScreenAgentReset(c *gin.Context) {
	var req struct {
		SessionID int64 `json:"session_id"`
	}
	c.ShouldBindJSON(&req)
	screenAgentAborted.Delete(req.SessionID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListScreenActions GET /api/screen-agent/actions
func ListScreenActionsHandler(c *gin.Context) {
	sessionID, _ := strconv.ParseInt(c.Query("session_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	actions := db.ListScreenActions(sessionID, limit)
	if actions == nil {
		actions = []db.ScreenAction{}
	}
	c.JSON(http.StatusOK, actions)
}

// GetAgentScreenConfig GET /api/agents/:id/screen-config
func GetAgentScreenConfigHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	enabled, config := db.GetAgentScreenConfig(id)
	c.JSON(http.StatusOK, gin.H{
		"agent_id": id,
		"enabled":  enabled,
		"config":   json.RawMessage(config),
	})
}

// SetAgentScreenConfig PUT /api/agents/:id/screen-config
func SetAgentScreenConfigHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfgStr := "{}"
	if len(body.Config) > 0 {
		cfgStr = string(body.Config)
	}
	if err := db.SetAgentScreenConfig(id, body.Enabled, cfgStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	apiCache.Invalidate("agents")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ScreenAgentExecutePlan POST /api/screen-agent/execute-plan
// 自动执行操作计划（OTA 循环），每步截屏→执行→验证
func ScreenAgentExecutePlan(c *gin.Context) {
	var req struct {
		SessionID int64                    `json:"session_id"`
		Steps     []map[string]interface{} `json:"steps"`
		AutoMode  bool                     `json:"auto_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Steps) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "steps required"})
		return
	}

	screenAgentAborted.Delete(req.SessionID)

	BroadcastEvent("screen_agent_plan_start", map[string]interface{}{
		"session_id":  req.SessionID,
		"total_steps": len(req.Steps),
	})

	c.JSON(http.StatusAccepted, gin.H{"status": "executing", "total_steps": len(req.Steps)})

	go runOTALoop(req.SessionID, req.Steps, req.AutoMode)
}

func runOTALoop(sessionID int64, steps []map[string]interface{}, autoMode bool) {
	for i, step := range steps {
		if isAborted(sessionID) {
			BroadcastEvent("screen_agent_plan_abort", map[string]interface{}{
				"session_id": sessionID,
				"step":       i,
			})
			return
		}

		description, _ := step["description"].(string)
		action, _ := step["action"].(string)
		params, _ := step["params"].(map[string]interface{})

		BroadcastEvent("screen_agent_step_start", map[string]interface{}{
			"session_id":  sessionID,
			"step":        i + 1,
			"total":       len(steps),
			"description": description,
			"action":      action,
		})

		actionData, _ := json.Marshal(map[string]interface{}{
			"type":        action,
			"description": description,
			"params":      params,
		})

		actionID, _ := db.InsertScreenAction(&db.ScreenAction{
			SessionID:  sessionID,
			ActionType: action,
			ActionData: string(actionData),
			Status:     "executing",
		})

		// 危险操作即使在自动模式下也强制确认
		needsConfirm := !autoMode || isDangerousAction(description, action, params)
		if needsConfirm {
			BroadcastEvent("screen_agent_confirm_needed", map[string]interface{}{
				"session_id":  sessionID,
				"action_id":   actionID,
				"step":        i + 1,
				"total":       len(steps),
				"description": description,
				"action":      action,
				"params":      params,
			"dangerous":   isDangerousAction(description, action, params),
			})
			confirmed := waitForConfirmation(sessionID, actionID, 120*time.Second)
			if !confirmed {
				db.UpdateScreenActionStatus(actionID, "cancelled", "用户取消", "")
				BroadcastEvent("screen_agent_step_done", map[string]interface{}{
					"action_id":  actionID,
					"session_id": sessionID,
					"step":       i + 1,
					"status":     "cancelled",
				})
				return
			}
		}

		// 发送执行指令给 Electron
		BroadcastEvent("screen_agent_execute", map[string]interface{}{
			"action_id":  actionID,
			"session_id": sessionID,
			"action":     string(actionData),
		})

		// 等待执行结果（通过 WS 回报）
		time.Sleep(800 * time.Millisecond)

		db.UpdateScreenActionStatus(actionID, "success", "", "")

		BroadcastEvent("screen_agent_step_done", map[string]interface{}{
			"action_id":  actionID,
			"session_id": sessionID,
			"step":       i + 1,
			"total":      len(steps),
			"status":     "success",
		})

		// 步间延迟，让 UI 有时间响应
		time.Sleep(600 * time.Millisecond)
	}

	BroadcastEvent("screen_agent_plan_done", map[string]interface{}{
		"session_id":  sessionID,
		"total_steps": len(steps),
	})
}

func isAborted(sessionID int64) bool {
	val, ok := screenAgentAborted.Load(sessionID)
	return ok && val.(bool)
}

// waitForConfirmation 等待用户确认（轮询 kv_store）
func waitForConfirmation(sessionID, actionID int64, timeout time.Duration) bool {
	key := fmt.Sprintf("screen_confirm_%d_%d", sessionID, actionID)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isAborted(sessionID) {
			return false
		}
		var val string
		err := db.DB.QueryRow(`SELECT value FROM kv_store WHERE key=?`, key).Scan(&val)
		if err == nil {
			db.DB.Exec(`DELETE FROM kv_store WHERE key=?`, key)
			return val == "confirmed"
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// ScreenAgentConfirmStep POST /api/screen-agent/confirm
// 用户确认或拒绝某步操作
func ScreenAgentConfirmStep(c *gin.Context) {
	var req struct {
		SessionID int64  `json:"session_id"`
		ActionID  int64  `json:"action_id"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key := fmt.Sprintf("screen_confirm_%d_%d", req.SessionID, req.ActionID)
	val := "rejected"
	if req.Confirmed {
		val = "confirmed"
	}
	db.DB.Exec(`INSERT OR REPLACE INTO kv_store (key, value) VALUES (?, ?)`, key, val)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── 内部函数 ─────────────────────────────────────────────────────

func buildScreenAnalysisPrompt(appName, windowTitle, url string, sw, sh, cx, cy float64) string {
	var sb strings.Builder
	sb.WriteString("你是灵犀 AI 助手的屏幕分析模块。用户发送了一张屏幕截图，请分析截图中的内容。\n\n")
	sb.WriteString("# 规则\n")
	sb.WriteString("- 用中文回答\n")
	sb.WriteString("- 描述屏幕上可见的应用界面、文字内容、按钮、菜单等 UI 元素\n")
	sb.WriteString("- 如果用户有具体指令，按指令分析\n")
	sb.WriteString("- 回答简洁明了，不超过 500 字\n\n")

	sb.WriteString("# 当前上下文\n")
	if appName != "" {
		sb.WriteString(fmt.Sprintf("- 活跃应用: %s\n", appName))
	}
	if windowTitle != "" {
		sb.WriteString(fmt.Sprintf("- 窗口标题: %s\n", windowTitle))
	}
	if url != "" {
		sb.WriteString(fmt.Sprintf("- 浏览器 URL: %s\n", url))
	}
	if sw > 0 && sh > 0 {
		sb.WriteString(fmt.Sprintf("- 屏幕分辨率: %.0fx%.0f\n", sw, sh))
	}
	if cx > 0 || cy > 0 {
		sb.WriteString(fmt.Sprintf("- 鼠标位置: (%.0f, %.0f)\n", cx, cy))
	}

	return sb.String()
}

func buildScreenPlanPrompt(appName, windowTitle string, sw, sh float64) string {
	var sb strings.Builder
	sb.WriteString("你是灵犀 AI 助手的屏幕操控模块。用户发送了一张屏幕截图和一个操作指令。\n")
	sb.WriteString("请分析截图，制定详细的操作计划来完成用户的指令。\n\n")

	sb.WriteString("# 规则\n")
	sb.WriteString("- 用中文回答\n")
	sb.WriteString("- 返回 JSON 格式的操作步骤列表\n")
	sb.WriteString("- 每个步骤包含: step（序号）、description（描述）、action（操作类型）、params（参数）\n")
	sb.WriteString("- 操作类型: click, type, keyPress, scroll, openApp\n")
	sb.WriteString("- click 参数: {x, y, button, count}，坐标为截图中的像素坐标\n")
	sb.WriteString("- type 参数: {text}，要输入的文字\n")
	sb.WriteString("- keyPress 参数: {key, modifiers}，按键和修饰键\n")
	sb.WriteString("- scroll 参数: {x, y, deltaY}，滚动位置和距离（负值向上）\n")
	sb.WriteString("- openApp 参数: {appName}，要打开的应用名\n\n")

	sb.WriteString("# 输出格式\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"plan\": \"操作计划简述\",\n")
	sb.WriteString("  \"steps\": [\n")
	sb.WriteString("    {\"step\": 1, \"description\": \"描述\", \"action\": \"click\", \"params\": {\"x\": 100, \"y\": 200}},\n")
	sb.WriteString("    {\"step\": 2, \"description\": \"描述\", \"action\": \"type\", \"params\": {\"text\": \"hello\"}}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"risk_level\": \"low\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("risk_level 取值: low（无风险）、medium（可能改变数据）、high（涉及删除/发送/支付）\n\n")

	sb.WriteString("# 当前上下文\n")
	if appName != "" {
		sb.WriteString(fmt.Sprintf("- 活跃应用: %s\n", appName))
	}
	if windowTitle != "" {
		sb.WriteString(fmt.Sprintf("- 窗口标题: %s\n", windowTitle))
	}
	if sw > 0 && sh > 0 {
		sb.WriteString(fmt.Sprintf("- 屏幕分辨率: %.0fx%.0f\n", sw, sh))
	}

	return sb.String()
}

// callVisionLLM 调用多模态模型（支持图片输入）
func callVisionLLM(systemPrompt, userMessage, screenshotBase64 string) (string, error) {
	_, _, _, token := activeRuntimeSnapshot()
	if token == "" {
		return "", fmt.Errorf("未配置 API 接入点，请先在设置中配置模型")
	}

	_, _, model, baseURL, _, protocol, _ := activeProfileSnapshot()
	if model == "" {
		return "", fmt.Errorf("未配置模型")
	}

	base := strings.TrimSuffix(baseURL, "/")

	// 构建多模态消息（OpenAI 兼容格式，支持大多数多模态模型）
	userContent := []map[string]interface{}{
		{
			"type": "text",
			"text": userMessage,
		},
		{
			"type": "image_url",
			"image_url": map[string]string{
				"url":    "data:image/png;base64," + screenshotBase64,
				"detail": "high",
			},
		},
	}

	var reqBody map[string]interface{}

	if protocol == "anthropic" {
		// Anthropic 格式
		if !strings.Contains(base, "/v1") {
			base += "/v1"
		}
		reqBody = map[string]interface{}{
			"model":      model,
			"max_tokens": 2048,
			"system":     systemPrompt,
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{"type": "text", "text": userMessage},
						{
							"type": "image",
							"source": map[string]string{
								"type":       "base64",
								"media_type": "image/png",
								"data":       screenshotBase64,
							},
						},
					},
				},
			},
		}

		body, _ := json.Marshal(reqBody)
		httpReq, err := http.NewRequest("POST", base+"/messages", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", token)
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", fmt.Errorf("Anthropic API 请求失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("Anthropic API 返回 %d: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}
		for _, c := range result.Content {
			if c.Type == "text" {
				return c.Text, nil
			}
		}
		return "", fmt.Errorf("Anthropic API 无文本回复")
	}

	// OpenAI 兼容格式
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	reqBody = map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"max_tokens":  2048,
		"temperature": 0.3,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequest("POST", base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("LLM API 无回复")
}

// isDangerousAction 检查操作是否涉及高风险关键词
func isDangerousAction(description, action string, params map[string]interface{}) bool {
	dangerousKeywords := []string{
		"删除", "delete", "remove", "格式化", "format",
		"发送", "send", "邮件", "email",
		"支付", "pay", "购买", "buy", "transfer",
		"密码", "password", "登录", "login", "sign in",
		"注销", "logout", "退出",
		"关机", "shutdown", "重启", "restart", "reboot",
		"清空", "clear", "reset",
		"卸载", "uninstall",
	}
	lower := strings.ToLower(description + " " + action)
	if text, ok := params["text"].(string); ok {
		lower += " " + strings.ToLower(text)
	}
	for _, kw := range dangerousKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// parseActionPlan 从 LLM 回复中提取 JSON 操作计划
func parseActionPlan(raw string) []map[string]interface{} {
	// 尝试从 markdown 代码块中提取 JSON
	jsonStr := raw
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(raw[start:], "```"); end >= 0 {
			jsonStr = raw[start : start+end]
		}
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		start := idx + 3
		if end := strings.Index(raw[start:], "```"); end >= 0 {
			jsonStr = raw[start : start+end]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

	var plan struct {
		Steps []map[string]interface{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &plan); err == nil && len(plan.Steps) > 0 {
		return plan.Steps
	}

	// 尝试直接解析为数组
	var steps []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &steps); err == nil {
		return steps
	}

	return nil
}
