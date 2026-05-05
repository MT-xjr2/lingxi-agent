package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"lingxi-agent/db"
	"lingxi-agent/nexus"

	"github.com/gin-gonic/gin"
)

// NexusTokenAuth 验证 X-Nexus-Token（跳过公开端点：/info、/connect-request）
func NexusTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/info") || strings.HasSuffix(path, "/connect-request") || strings.HasSuffix(path, "/connect-respond") {
			c.Next()
			return
		}
		// 内部管理端点（前端调用本地后端）不需要 token
		if strings.HasSuffix(path, "/settings") {
			c.Next()
			return
		}

		token := c.GetHeader("X-Nexus-Token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Nexus-Token"})
			c.Abort()
			return
		}
		if !db.NexusContactExistsBySecret(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid nexus token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ─── 对外暴露的 API（供其他灵犀实例调用）─────────────────────────

// NexusInfo GET /api/nexus/info — 返回本实例公开信息
func NexusInfo(c *gin.Context) {
	settings, _ := db.GetNexusSettings()
	configs, _ := db.ListPublicAgentConfigs()

	agents := make([]nexus.PublicAgent, 0, len(configs))
	for _, cfg := range configs {
		var tags []string
		json.Unmarshal([]byte(cfg.CapabilityTags), &tags)
		if tags == nil {
			tags = []string{}
		}
		name := cfg.PublicName
		if name == "" {
			a, err := db.GetAgent(cfg.AgentID)
			if err == nil {
				name = a.Name
			}
		}
		agents = append(agents, nexus.PublicAgent{
			ID:             cfg.AgentID,
			Name:           name,
			CapabilityTags: tags,
			AuthLevel:      cfg.AuthLevel,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"instance_id": nexus.Global.InstanceID(),
		"nickname":    settings.Nickname,
		"agents":      agents,
	})
}

// ─── 内部管理 API（前端调用本地后端）─────────────────────────────

// GetNexusSettings GET /api/nexus/settings
func GetNexusSettings(c *gin.Context) {
	settings, err := db.GetNexusSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateNexusSettings PUT /api/nexus/settings
func UpdateNexusSettings(c *gin.Context) {
	var body db.NexusSettings
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.ListenPort == 0 {
		body.ListenPort = 3001
	}
	if err := db.UpdateNexusSettings(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nexus.Global.Restart()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListPeers GET /api/peers — 返回局域网内发现的实例
func ListPeers(c *gin.Context) {
	peers, err := db.ListNexusPeers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if peers == nil {
		peers = []db.NexusPeer{}
	}
	c.JSON(http.StatusOK, peers)
}
