package handler

import (
	"net/http"
	"time"

	"lingxi-agent/evolution"

	"github.com/gin-gonic/gin"
)

// GetEvolutionScannerConfig GET /api/evolution/scanner-config
func GetEvolutionScannerConfig(c *gin.Context) {
	cfg := evolution.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"enabled":              cfg.Enabled,
		"scan_interval_hours":  int(cfg.ScanInterval / time.Hour),
		"min_session_messages": cfg.MinSessionMessages,
		"cooldown_hours":       cfg.CooldownHours,
		"quiet_start":          cfg.QuietStart,
		"quiet_end":            cfg.QuietEnd,
	})
}

// UpdateEvolutionScannerConfig PUT /api/evolution/scanner-config
func UpdateEvolutionScannerConfig(c *gin.Context) {
	var body struct {
		Enabled            *bool `json:"enabled"`
		ScanIntervalHours  *int  `json:"scan_interval_hours"`
		MinSessionMessages *int  `json:"min_session_messages"`
		CooldownHours      *int  `json:"cooldown_hours"`
		QuietStart         *int  `json:"quiet_start"`
		QuietEnd           *int  `json:"quiet_end"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg := evolution.GetConfig()
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	}
	if body.ScanIntervalHours != nil && *body.ScanIntervalHours > 0 {
		cfg.ScanInterval = time.Duration(*body.ScanIntervalHours) * time.Hour
	}
	if body.MinSessionMessages != nil {
		cfg.MinSessionMessages = *body.MinSessionMessages
	}
	if body.CooldownHours != nil {
		cfg.CooldownHours = *body.CooldownHours
	}
	if body.QuietStart != nil {
		cfg.QuietStart = *body.QuietStart
	}
	if body.QuietEnd != nil {
		cfg.QuietEnd = *body.QuietEnd
	}
	evolution.SetConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
