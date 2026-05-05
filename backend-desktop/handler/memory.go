package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
	"lingxi-agent/model"
)

// ListMemories GET /api/memories?agent_id=N
func ListMemories(c *gin.Context) {
	agentIDStr := c.DefaultQuery("agent_id", "0")
	agentID, _ := strconv.ParseInt(agentIDStr, 10, 64)

	rows, err := db.DB.Query(
		`SELECT id, agent_id, content, category, created_at FROM memories WHERE agent_id=? ORDER BY created_at DESC LIMIT 200`,
		agentID,
	)
	if err != nil {
		log.Printf("[memory] list error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	memories := make([]model.Memory, 0)
	for rows.Next() {
		var m model.Memory
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Content, &m.Category, &m.CreatedAt); err != nil {
			continue
		}
		memories = append(memories, m)
	}
	c.JSON(http.StatusOK, memories)
}

// CreateMemory POST /api/memories
func CreateMemory(c *gin.Context) {
	var body struct {
		AgentID  int64  `json:"agent_id"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content 不能为空"})
		return
	}
	if body.Category == "" {
		body.Category = "general"
	}

	res, err := db.DB.Exec(
		`INSERT INTO memories (agent_id, content, category) VALUES (?,?,?)`,
		body.AgentID, body.Content, body.Category,
	)
	if err != nil {
		log.Printf("[memory] create error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeleteMemory DELETE /api/memories/:id
func DeleteMemory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if _, err := db.DB.Exec(`DELETE FROM memories WHERE id=?`, id); err != nil {
		log.Printf("[memory] delete error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ClearMemories DELETE /api/memories?agent_id=N
func ClearMemories(c *gin.Context) {
	agentIDStr := c.DefaultQuery("agent_id", "0")
	agentID, _ := strconv.ParseInt(agentIDStr, 10, 64)
	if _, err := db.DB.Exec(`DELETE FROM memories WHERE agent_id=?`, agentID); err != nil {
		log.Printf("[memory] clear error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetRelevantMemories 为对话注入相关记忆（内部调用）
func GetRelevantMemories(agentID int64, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.DB.Query(
		`SELECT content FROM memories WHERE agent_id=? ORDER BY created_at DESC LIMIT ?`,
		agentID, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			result = append(result, content)
		}
	}
	return result
}

// ToggleMessagePin POST /api/messages/:id/pin
func ToggleMessagePin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var body struct {
		Pinned bool `json:"pinned"`
	}
	_ = c.ShouldBindJSON(&body)
	pinnedInt := 0
	if body.Pinned {
		pinnedInt = 1
	}
	if _, err := db.DB.Exec(`UPDATE messages SET pinned=? WHERE id=?`, pinnedInt, id); err != nil {
		log.Printf("[memory] toggle pin error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
