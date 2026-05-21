package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadAgentAvatar POST /api/agents/upload-avatar
func UploadAgentAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 file 字段"})
		return
	}
	if file.Size > 10<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片过大（10MB 上限）"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".heic":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持图片"})
		return
	}

	dir := uploadsDir()
	os.MkdirAll(dir, 0755)
	name := fmt.Sprintf("a_%d_%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
	fpath := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, fpath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"url":      "/api/uploads/" + name,
		"filename": name,
	})
}
