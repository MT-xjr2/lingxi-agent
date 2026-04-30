package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"lingxi-agent/connector"
	"lingxi-agent/db"
)

// ListIMConnectors GET /api/im-connectors
func ListIMConnectors(c *gin.Context) {
	list, err := db.ListIMConnectors()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 附加运行状态
	type item struct {
		db.IMConnector
		Running bool `json:"running"`
	}
	result := make([]item, 0, len(list))
	for _, conn := range list {
		result = append(result, item{
			IMConnector: conn,
			Running:     connector.GlobalManager != nil && connector.GlobalManager.IsRunning(conn.Platform),
		})
	}
	c.JSON(http.StatusOK, result)
}

// UpsertIMConnector POST /api/im-connectors
// body: {"platform":"dingtalk","config":{"client_id":"...","client_secret":"..."}}
func UpsertIMConnector(c *gin.Context) {
	var body struct {
		Platform string      `json:"platform" binding:"required"`
		Config   interface{} `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configJSON, err := json.Marshal(body.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config"})
		return
	}

	if err := db.UpsertIMConnector(body.Platform, string(configJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// EnableIMConnector PUT /api/im-connectors/:platform/enable
func EnableIMConnector(c *gin.Context) {
	platform := c.Param("platform")
	conn, err := db.GetIMConnector(platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connector not found"})
		return
	}
	if err := db.SetIMConnectorEnabled(platform, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if connector.GlobalManager != nil {
		if err := connector.GlobalManager.Start(platform, conn.Config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db updated but start failed: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "running": true})
}

// DisableIMConnector PUT /api/im-connectors/:platform/disable
func DisableIMConnector(c *gin.Context) {
	platform := c.Param("platform")
	if err := db.SetIMConnectorEnabled(platform, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if connector.GlobalManager != nil {
		connector.GlobalManager.Stop(platform)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "running": false})
}

// DeleteIMConnector DELETE /api/im-connectors/:platform
func DeleteIMConnector(c *gin.Context) {
	platform := c.Param("platform")
	if connector.GlobalManager != nil {
		connector.GlobalManager.Stop(platform)
	}
	if err := db.DeleteIMConnector(platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
