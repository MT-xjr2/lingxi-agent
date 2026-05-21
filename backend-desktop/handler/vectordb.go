package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"lingxi-agent/vectordb"

	"github.com/gin-gonic/gin"
)

// ReindexKnowledge POST /api/knowledge/reindex - 全量重建向量索引
func ReindexKnowledge(c *gin.Context) {
	if vectordb.IsIndexing() {
		c.JSON(http.StatusConflict, gin.H{"error": "indexing already in progress"})
		return
	}

	kbDir := knowledgeDir()
	go vectordb.ReindexAll(kbDir)

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "reindex started"})
}

// GetIndexStatus GET /api/knowledge/index-status - 获取索引状态
func GetIndexStatus(c *gin.Context) {
	status, err := vectordb.GetIndexStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// SemanticSearch GET /api/knowledge/search?q=xxx&limit=10 - 语义搜索
func SemanticSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	results, err := vectordb.HybridSearch(query, limit, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if results == nil {
		results = []vectordb.RetrievalResult{}
	}
	c.JSON(http.StatusOK, gin.H{"results": results, "total": len(results)})
}

// ListWatchedDirs GET /api/knowledge/watched-dirs
func ListWatchedDirs(c *gin.Context) {
	dirs, err := vectordb.ListWatchedDirs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if dirs == nil {
		dirs = []vectordb.WatchedDir{}
	}
	c.JSON(http.StatusOK, dirs)
}

// AddWatchedDir POST /api/knowledge/watched-dirs
func AddWatchedDir(c *gin.Context) {
	var body struct {
		DirPath string `json:"dir_path"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.DirPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dir_path is required"})
		return
	}

	// 验证目录存在
	info, err := os.Stat(body.DirPath)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "directory does not exist"})
		return
	}

	// 转为绝对路径
	absPath, _ := filepath.Abs(body.DirPath)

	id, err := vectordb.AddWatchedDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 触发该目录的初始索引
	go indexWatchedDirectory(absPath)

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// RemoveWatchedDir DELETE /api/knowledge/watched-dirs/:id
func RemoveWatchedDir(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := vectordb.RemoveWatchedDir(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetEmbeddingConfig GET /api/knowledge/embedding-config
func GetEmbeddingConfig(c *gin.Context) {
	cfg := vectordb.GetEmbeddingConfig()
	c.JSON(http.StatusOK, cfg)
}

// SetEmbeddingConfig PUT /api/knowledge/embedding-config
func SetEmbeddingConfig(c *gin.Context) {
	var cfg vectordb.EmbeddingConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := vectordb.SetEmbeddingConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// indexWatchedDirectory 遍历目录索引所有支持的文件
func indexWatchedDirectory(dirPath string) {
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !vectordb.IsSupportedFile(path) {
			return nil
		}
		// 跳过隐藏文件和特殊目录
		if isHidden(path) {
			return nil
		}
		vectordb.IndexWatchedFile(path)
		return nil
	})
}

func isHidden(path string) bool {
	parts := filepath.SplitList(path)
	for _, p := range parts {
		if len(p) > 0 && p[0] == '.' {
			return true
		}
	}
	base := filepath.Base(path)
	return len(base) > 0 && base[0] == '.'
}
