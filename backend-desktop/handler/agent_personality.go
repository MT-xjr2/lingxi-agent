package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"lingxi-agent/db"
)

// GetAgentPersonality GET /api/agents/:id/personality
// 未配置时返回默认人格（不是 404）
func GetAgentPersonality(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 agent_id"})
		return
	}
	p, err := db.GetPersonality(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// UpsertAgentPersonality PUT /api/agents/:id/personality
func UpsertAgentPersonality(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 agent_id"})
		return
	}
	var p db.AgentPersonality
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p.AgentID = id
	if err := db.UpsertPersonality(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 触发智能体缓存失效
	apiCache.Invalidate("agents")
	out, _ := db.GetPersonality(id)
	c.JSON(http.StatusOK, out)
}
