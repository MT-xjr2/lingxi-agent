package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"lingxi-agent/db"
	"lingxi-agent/vectordb"

	"github.com/gin-gonic/gin"
)

// QuickChat POST /api/chat/quick — Spotlight 快捷对话
func QuickChat(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
		Context struct {
			App         string `json:"app"`
			WindowTitle string `json:"window_title"`
			URL         string `json:"url"`
			ContextType string `json:"context_type"`
		} `json:"context"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	// 构建上下文增强的 prompt
	var sb strings.Builder
	sb.WriteString("你是灵犀 AI 助手的 Spotlight 快捷模式。用户通过全局快捷键唤出了你。\n")
	sb.WriteString("请用简洁、直接的方式回答，不超过 200 字。不需要客套寒暄。\n\n")

	if req.Context.App != "" {
		sb.WriteString(fmt.Sprintf("用户当前正在使用: %s", req.Context.App))
		if req.Context.WindowTitle != "" {
			sb.WriteString(fmt.Sprintf(" - %s", req.Context.WindowTitle))
		}
		sb.WriteString("\n")
	}
	if req.Context.URL != "" {
		sb.WriteString(fmt.Sprintf("当前浏览器 URL: %s\n", req.Context.URL))
	}

	// 尝试从知识库检索相关内容
	if vectordb.VecDB != nil {
		results, err := vectordb.HybridSearch(req.Message, 3, nil)
		if err == nil && len(results) > 0 {
			sb.WriteString("\n[知识库参考资料]\n")
			for i, r := range results {
				excerpt := r.ChunkText
				if len([]rune(excerpt)) > 300 {
					excerpt = string([]rune(excerpt)[:300])
				}
				sb.WriteString(fmt.Sprintf("--- 来源 %d: %s ---\n%s\n\n", i+1, r.FilePath, excerpt))
			}
		}
	}

	systemPrompt := sb.String()
	reply, err := callQuickLLM(systemPrompt, req.Message)
	if err != nil {
		slog.Error("quick chat LLM error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

// callQuickLLM 调用当前激活的 API Profile 做简短回复
func callQuickLLM(systemPrompt, userMessage string) (string, error) {
	profiles, err := db.ListAPIProfiles(true)
	if err != nil || len(profiles) == 0 {
		return "", fmt.Errorf("no API profile configured")
	}

	var profile db.APIProfile
	for _, p := range profiles {
		if p.IsActive {
			profile = p
			break
		}
	}
	if profile.ID == 0 && len(profiles) > 0 {
		profile = profiles[0]
	}

	baseURL := strings.TrimSuffix(profile.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	// 使用 OpenAI 兼容格式
	reqBody := map[string]interface{}{
		"model": profile.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"max_tokens":  500,
		"temperature": 0.7,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if profile.AuthTokenCipher != "" {
		httpReq.Header.Set("Authorization", "Bearer "+profile.AuthTokenCipher)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, string(respBody))
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
	return "无法生成回复", nil
}
