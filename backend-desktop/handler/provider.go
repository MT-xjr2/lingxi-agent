package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
)

// ─── 激活档案运行时缓存 ──────────────────────────────────────────
//
// Electron 在启动时通过 POST /api/runtime/active-secret 把激活档案的明文 token
// 一次性下发到后端进程内存（不落盘），后端 spawn claude CLI 时优先使用该明文。
// 切换激活档案时，前端调用 /api/api-profiles/:id/activate，由 Electron 监听
// profile_changed WS 事件后再次下发新的明文。

type activeProfileRuntime struct {
	mu       sync.RWMutex
	id       int64
	model    string
	baseURL  string
	token    string // 明文，仅内存
}

var activeRuntime activeProfileRuntime

// SetActiveSecret 由 Electron 通过 IPC HTTP 调用，下发当前激活档案明文
func SetActiveSecret(c *gin.Context) {
	var body struct {
		ID      int64  `json:"id"`
		Model   string `json:"model"`
		BaseURL string `json:"base_url"`
		Token   string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	activeRuntime.mu.Lock()
	activeRuntime.id = body.ID
	activeRuntime.model = body.Model
	activeRuntime.baseURL = body.BaseURL
	activeRuntime.token = body.Token
	activeRuntime.mu.Unlock()
	log.Printf("[provider] active secret set: id=%d model=%s base=%s tokenLen=%d",
		body.ID, body.Model, body.BaseURL, len(body.Token))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// activeRuntimeSnapshot 在 chat.go buildClaudeEnv 中调用
func activeRuntimeSnapshot() (id int64, model, baseURL, token string) {
	activeRuntime.mu.RLock()
	defer activeRuntime.mu.RUnlock()
	return activeRuntime.id, activeRuntime.model, activeRuntime.baseURL, activeRuntime.token
}

// ─── HTTP 接口 ───────────────────────────────────────────────────

// ListProviders GET /api/providers
func ListProviders(c *gin.Context) {
	list, err := db.ListProviders()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, list)
}

// ListAPIProfiles GET /api/api-profiles
func ListAPIProfiles(c *gin.Context) {
	includeCipher := c.Query("include_cipher") == "1"
	list, err := db.ListAPIProfiles(includeCipher)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, list)
}

// UpsertAPIProfile POST /api/api-profiles
// body: { id?, name, provider_id, base_url, model, auth_token_cipher, auth_token_mask, extra }
func UpsertAPIProfile(c *gin.Context) {
	var body struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		ProviderID      int64  `json:"provider_id"`
		BaseURL         string `json:"base_url"`
		Model           string `json:"model"`
		AuthTokenCipher string `json:"auth_token_cipher"`
		AuthTokenMask   string `json:"auth_token_mask"`
		Extra           string `json:"extra"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" || body.ProviderID == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	if body.Extra == "" {
		body.Extra = "{}"
	}
	// 如果是更新且未提供新的 cipher，则保留旧值
	if body.ID > 0 && body.AuthTokenCipher == "" {
		old, err := db.GetAPIProfile(body.ID, true)
		if err == nil {
			body.AuthTokenCipher = old.AuthTokenCipher
			if body.AuthTokenMask == "" {
				body.AuthTokenMask = old.AuthTokenMask
			}
		}
	}
	ap := &db.APIProfile{
		ID:              body.ID,
		Name:            body.Name,
		ProviderID:      body.ProviderID,
		BaseURL:         body.BaseURL,
		Model:           body.Model,
		AuthTokenCipher: body.AuthTokenCipher,
		AuthTokenMask:   body.AuthTokenMask,
		Extra:           body.Extra,
	}
	id, err := db.UpsertAPIProfile(ap)
	if err != nil {
		log.Printf("[provider] upsert error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeleteAPIProfile DELETE /api/api-profiles/:id
func DeleteAPIProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := db.DeleteAPIProfile(id); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// ActivateAPIProfile POST /api/api-profiles/:id/activate
func ActivateAPIProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := db.ActivateAPIProfile(id); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	// 通知 Electron 重新下发明文
	ap, _ := db.GetAPIProfile(id, false)
	payload, _ := json.Marshal(map[string]interface{}{
		"id":           id,
		"name":         ap.Name,
		"model":        ap.Model,
		"base_url":     ap.BaseURL,
		"provider_id":  ap.ProviderID,
		"requires_secret_refresh": true,
	})
	globalHub.BroadcastAll("profile_changed", string(payload))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestAPIProfile POST /api/api-profiles/:id/test
// body: { token? }  token 由前端解密后临时传入用于真实请求；不传则使用当前激活内存 token（仅当 id 是激活档案时）
func TestAPIProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	_ = c.ShouldBindJSON(&body)

	ap, err := db.GetAPIProfile(id, false)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	token := body.Token
	if token == "" {
		actID, _, _, t := activeRuntimeSnapshot()
		if actID == id {
			token = t
		}
	}
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "缺少 token：请先保存档案或刷新激活档案"})
		return
	}

	baseURL := strings.TrimRight(ap.BaseURL, "/")
	if baseURL == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "base_url 为空"})
		return
	}

	// 发起一次最小 messages 请求，使用 anthropic 协议
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      ap.Model,
		"max_tokens": 16,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})
	httpReq, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", token)
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, gin.H{
			"ok":     false,
			"status": resp.StatusCode,
			"error":  truncateStr(string(bodyBytes), 400),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"status":  resp.StatusCode,
		"latency": fmt.Sprintf("%v", time.Now()), // 占位
	})
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
