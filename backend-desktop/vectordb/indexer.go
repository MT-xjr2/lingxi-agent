package vectordb

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lingxi-agent/db"
)

// BroadcastFn 由 main 包注入，用于实时推送索引进度
var BroadcastFn func(event, data string)

var indexMu sync.Mutex
var indexRunning bool

// IsIndexing 返回是否正在索引
func IsIndexing() bool {
	indexMu.Lock()
	defer indexMu.Unlock()
	return indexRunning
}

// ReindexAll 全量重建索引
func ReindexAll(kbDir string) error {
	indexMu.Lock()
	if indexRunning {
		indexMu.Unlock()
		return fmt.Errorf("indexing already in progress")
	}
	indexRunning = true
	indexMu.Unlock()

	defer func() {
		indexMu.Lock()
		indexRunning = false
		indexMu.Unlock()
	}()

	slog.Info("starting full reindex")
	broadcastProgress("reindex_start", 0, "正在重建索引...")

	// 获取所有知识库条目
	items, err := db.ListKnowledge()
	if err != nil {
		return err
	}

	total := len(items)
	if total == 0 {
		UpdateIndexStatus(0, 0, false, 1.0)
		broadcastProgress("reindex_done", 1.0, "索引为空")
		return nil
	}

	UpdateIndexStatus(total, 0, true, 0)

	// 获取嵌入器
	embedder, err := GetActiveEmbedder()
	if err != nil {
		slog.Error("embedder unavailable, indexing without vectors", "err", err)
	}

	totalChunks := 0
	for i, item := range items {
		kid, _ := item["id"].(int64)
		filePath, _ := item["file_path"].(string)

		// 删除旧的分块
		DeleteChunksByKnowledge(kid)

		// 读取文件内容
		content := readKnowledgeFile(kbDir, filePath)
		if content == "" {
			continue
		}

		// 分块
		chunks := ChunkText(content, DefaultChunkSize, DefaultChunkOverlap)
		if len(chunks) == 0 {
			continue
		}

		// 批量嵌入
		var embeddings [][]float32
		if embedder != nil {
			texts := make([]string, len(chunks))
			for j, c := range chunks {
				texts[j] = c.Text
			}
			embeddings, err = embedder.Embed(texts)
			if err != nil {
				slog.Warn("embedding failed for file, storing chunks without vectors", "file", filePath, "err", err)
				embeddings = nil
			}
		}

		// 写入数据库
		for j, c := range chunks {
			var emb []float32
			if embeddings != nil && j < len(embeddings) {
				emb = embeddings[j]
			}
			if err := InsertChunk(kid, filePath, c.Index, c.Text, c.TokenCount, emb); err != nil {
				slog.Error("insert chunk failed", "file", filePath, "chunk", j, "err", err)
			}
			totalChunks++
		}

		progress := float64(i+1) / float64(total)
		UpdateIndexStatus(total, totalChunks, true, progress)
		broadcastProgress("reindex_progress", progress, fmt.Sprintf("已索引 %d/%d 文档", i+1, total))
	}

	UpdateIndexStatus(total, totalChunks, false, 1.0)
	broadcastProgress("reindex_done", 1.0, fmt.Sprintf("索引完成：%d 文档，%d 分块", total, totalChunks))
	slog.Info("reindex complete", "docs", total, "chunks", totalChunks)
	return nil
}

// IndexSingleFile 索引单个文件（用于增量更新）
func IndexSingleFile(kbDir string, knowledgeID int64, filePath string) error {
	// 删除旧分块
	DeleteChunksByKnowledge(knowledgeID)

	content := readKnowledgeFile(kbDir, filePath)
	if content == "" {
		return nil
	}

	chunks := ChunkText(content, DefaultChunkSize, DefaultChunkOverlap)
	if len(chunks) == 0 {
		return nil
	}

	embedder, _ := GetActiveEmbedder()
	var embeddings [][]float32
	if embedder != nil {
		texts := make([]string, len(chunks))
		for j, c := range chunks {
			texts[j] = c.Text
		}
		var err error
		embeddings, err = embedder.Embed(texts)
		if err != nil {
			slog.Warn("embedding failed", "file", filePath, "err", err)
		}
	}

	for j, c := range chunks {
		var emb []float32
		if embeddings != nil && j < len(embeddings) {
			emb = embeddings[j]
		}
		if err := InsertChunk(knowledgeID, filePath, c.Index, c.Text, c.TokenCount, emb); err != nil {
			return err
		}
	}

	// 更新统计
	UpdateIndexStatus(DocCount(), ChunkCount(), false, 1.0)
	return nil
}

// IndexWatchedFile 索引监控目录中的文件（无 knowledge_id，用 file_path 作为唯一标识）
func IndexWatchedFile(absPath string) error {
	content := readFileContent(absPath)
	if content == "" {
		return nil
	}

	// 删除该路径的旧分块
	DeleteChunksByFilePath(absPath)

	chunks := ChunkText(content, DefaultChunkSize, DefaultChunkOverlap)
	if len(chunks) == 0 {
		return nil
	}

	embedder, _ := GetActiveEmbedder()
	var embeddings [][]float32
	if embedder != nil {
		texts := make([]string, len(chunks))
		for j, c := range chunks {
			texts[j] = c.Text
		}
		var err error
		embeddings, err = embedder.Embed(texts)
		if err != nil {
			slog.Warn("embedding failed for watched file", "path", absPath, "err", err)
		}
	}

	for j, c := range chunks {
		var emb []float32
		if embeddings != nil && j < len(embeddings) {
			emb = embeddings[j]
		}
		// 使用 knowledge_id = -1 标记监控目录文件
		if err := InsertChunk(-1, absPath, c.Index, c.Text, c.TokenCount, emb); err != nil {
			return err
		}
	}

	UpdateIndexStatus(DocCount(), ChunkCount(), false, 1.0)
	return nil
}

// RemoveWatchedFile 删除监控文件的索引
func RemoveWatchedFile(absPath string) error {
	return DeleteChunksByFilePath(absPath)
}

// ─── 辅助函数 ───────────────────────────────────────────────────

func readKnowledgeFile(kbDir, relPath string) string {
	absPath := filepath.Join(kbDir, relPath)
	return readFileContent(absPath)
}

func readFileContent(absPath string) string {
	ext := strings.ToLower(filepath.Ext(absPath))

	// 二进制格式优先读取已提取的文本
	if ext == ".pdf" || ext == ".docx" {
		extractedPath := absPath + ".extracted.txt"
		if _, err := os.Stat(extractedPath); err == nil {
			absPath = extractedPath
		} else {
			return ""
		}
	}

	// 检查是否为支持的文本格式
	supportedExts := map[string]bool{
		".md": true, ".txt": true, ".csv": true, ".tsv": true,
		".json": true, ".go": true, ".py": true, ".js": true,
		".ts": true, ".jsx": true, ".tsx": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".rs": true,
		".rb": true, ".php": true, ".swift": true, ".kt": true,
		".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".html": true, ".css": true, ".sql": true, ".sh": true,
		".extracted.txt": true,
	}

	if ext != ".extracted.txt" && !supportedExts[ext] {
		return ""
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}

	content := string(data)
	// 限制单文件最大处理量（100KB）
	if len(content) > 100*1024 {
		content = content[:100*1024]
	}
	return content
}

func broadcastProgress(event string, progress float64, message string) {
	if BroadcastFn == nil {
		return
	}
	data := fmt.Sprintf(`{"event":"%s","progress":%f,"message":"%s","time":"%s"}`,
		event, progress, message, time.Now().Format("15:04:05"))
	BroadcastFn("index_progress", data)
}

// isSupportedFile 检查文件是否应该被索引
func IsSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supported := map[string]bool{
		".md": true, ".txt": true, ".csv": true, ".tsv": true,
		".json": true, ".go": true, ".py": true, ".js": true,
		".ts": true, ".jsx": true, ".tsx": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".rs": true,
		".rb": true, ".php": true, ".swift": true, ".kt": true,
		".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".html": true, ".css": true, ".sql": true, ".sh": true,
		".pdf": true, ".docx": true,
	}
	return supported[ext]
}
