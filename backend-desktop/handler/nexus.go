package handler

import (
	"encoding/json"
	"net/http"

	"lingxi-agent/db"
	"lingxi-agent/nexus"

	"github.com/gin-gonic/gin"
)

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

	oldSettings, _ := db.GetNexusSettings()

	if err := db.UpdateNexusSettings(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nexus.Global.Restart()

	// 处理 WAN 启停
	sc := nexus.GetSignalingClient()
	if body.WANEnabled && body.SignalingURL != "" {
		if !oldSettings.WANEnabled || oldSettings.SignalingURL != body.SignalingURL {
			sc.Stop()
			sc.Start(body.SignalingURL)
		}
	} else if !body.WANEnabled && oldSettings.WANEnabled {
		sc.Stop()
	}

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

// ─── 广域网 (WAN) API ─────────────────────────────────────────

// ListWANPeers GET /api/wan/peers — 返回信令服务器上的远程节点列表
func ListWANPeers(c *gin.Context) {
	sc := nexus.GetSignalingClient()
	if !sc.IsConnected() {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	sc.RefreshPeers()
	peers := sc.ListRemotePeers()
	c.JSON(http.StatusOK, peers)
}

// WANStatus GET /api/wan/status — 广域网连接状态
func WANStatus(c *gin.Context) {
	sc := nexus.GetSignalingClient()
	settings, _ := db.GetNexusSettings()
	c.JSON(http.StatusOK, gin.H{
		"enabled":       settings.WANEnabled,
		"signaling_url": settings.SignalingURL,
		"connected":     sc.IsConnected(),
	})
}
