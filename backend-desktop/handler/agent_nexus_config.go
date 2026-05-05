package handler

import (
	"net/http"
	"strconv"

	"lingxi-agent/db"

	"github.com/gin-gonic/gin"
)

// GetAgentNexusConfig GET /api/agents/:id/nexus-config
func GetAgentNexusConfig(c *gin.Context) {
	agentID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	cfg, err := db.GetAgentNexusConfig(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// UpsertAgentNexusConfig PUT /api/agents/:id/nexus-config
func UpsertAgentNexusConfig(c *gin.Context) {
	agentID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body db.AgentNexusConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.AgentID = agentID
	if body.CapabilityTags == "" {
		body.CapabilityTags = "[]"
	}
	if body.AuthLevel == "" {
		body.AuthLevel = "readonly"
	}
	if body.PublicKnowledgeIDs == "" {
		body.PublicKnowledgeIDs = "[]"
	}

	if err := db.UpsertAgentNexusConfig(&body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重启 mDNS 以更新公开 Agent 信息
	go func() {
		// 不需要 import nexus，由 discovery restart 重新拉取
	}()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
