package vectordb

import (
	"database/sql"
	"encoding/binary"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"lingxi-agent/config"
)

var (
	VecDB *sql.DB
	mu    sync.RWMutex
)

const EmbeddingDim = 768

func Init() {
	cfg := config.Get()
	dbPath := filepath.Join(filepath.Dir(cfg.DB.Path), "vectors.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	var err error
	VecDB, err = sql.Open("sqlite3", "file:"+dbPath+"?_journal=WAL&_timeout=5000")
	if err != nil {
		slog.Error("vectordb open error", "err", err)
		return
	}
	VecDB.SetMaxOpenConns(2)

	if err = VecDB.Ping(); err != nil {
		slog.Error("vectordb ping error", "err", err)
		return
	}

	initSchema()
	slog.Info("VectorDB ready", "path", dbPath)
}

func initSchema() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS chunks (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			knowledge_id  INTEGER NOT NULL,
			file_path     TEXT    NOT NULL DEFAULT '',
			chunk_index   INTEGER NOT NULL DEFAULT 0,
			chunk_text    TEXT    NOT NULL DEFAULT '',
			token_count   INTEGER NOT NULL DEFAULT 0,
			embedding     BLOB,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_knowledge ON chunks(knowledge_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_filepath ON chunks(file_path)`,
		`CREATE TABLE IF NOT EXISTS index_status (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			total_docs    INTEGER NOT NULL DEFAULT 0,
			total_chunks  INTEGER NOT NULL DEFAULT 0,
			last_updated  DATETIME,
			is_indexing   INTEGER NOT NULL DEFAULT 0,
			progress      REAL    NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO index_status (id, total_docs, total_chunks) VALUES (1, 0, 0)`,
		`CREATE TABLE IF NOT EXISTS watched_dirs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			dir_path   TEXT    NOT NULL UNIQUE,
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS embedding_config (
			id       INTEGER PRIMARY KEY CHECK (id = 1),
			mode     TEXT NOT NULL DEFAULT 'api',
			api_url  TEXT NOT NULL DEFAULT '',
			model    TEXT NOT NULL DEFAULT 'text-embedding-3-small'
		)`,
		`INSERT OR IGNORE INTO embedding_config (id) VALUES (1)`,
	}

	for _, s := range stmts {
		if _, err := VecDB.Exec(s); err != nil {
			slog.Error("vectordb schema error", "err", err, "sql", s)
		}
	}
}

// SerializeFloat32 将 float32 切片序列化为 little-endian 二进制格式
func SerializeFloat32(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// DeserializeFloat32 从 little-endian 二进制反序列化为 float32 切片
func DeserializeFloat32(buf []byte) []float32 {
	if len(buf) == 0 {
		return nil
	}
	v := make([]float32, len(buf)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return v
}

// CosineSimilarity 计算两个向量的余弦相似度
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// InsertChunk 插入一个文本分块及其向量
func InsertChunk(knowledgeID int64, filePath string, chunkIndex int, chunkText string, tokenCount int, embedding []float32) error {
	mu.Lock()
	defer mu.Unlock()

	var blob []byte
	if len(embedding) == EmbeddingDim {
		blob = SerializeFloat32(embedding)
	}

	_, err := VecDB.Exec(
		`INSERT INTO chunks (knowledge_id, file_path, chunk_index, chunk_text, token_count, embedding) VALUES (?, ?, ?, ?, ?, ?)`,
		knowledgeID, filePath, chunkIndex, chunkText, tokenCount, blob,
	)
	return err
}

// DeleteChunksByKnowledge 删除指定知识库条目的所有分块
func DeleteChunksByKnowledge(knowledgeID int64) error {
	mu.Lock()
	defer mu.Unlock()
	_, err := VecDB.Exec(`DELETE FROM chunks WHERE knowledge_id = ?`, knowledgeID)
	return err
}

// DeleteChunksByFilePath 删除指定文件路径的所有分块
func DeleteChunksByFilePath(filePath string) error {
	mu.Lock()
	defer mu.Unlock()
	_, err := VecDB.Exec(`DELETE FROM chunks WHERE file_path = ?`, filePath)
	return err
}

// SearchByVector 向量相似度搜索（纯 Go 实现，brute-force cosine similarity）
func SearchByVector(queryEmbedding []float32, limit int, knowledgeIDs []int64) ([]SearchResult, error) {
	mu.RLock()
	defer mu.RUnlock()

	if len(queryEmbedding) != EmbeddingDim {
		return nil, nil
	}

	// 构建查询
	query := `SELECT id, knowledge_id, file_path, chunk_index, chunk_text, embedding FROM chunks WHERE embedding IS NOT NULL`
	var args []interface{}

	if len(knowledgeIDs) > 0 {
		query += " AND knowledge_id IN ("
		for i, id := range knowledgeIDs {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
	}

	rows, err := VecDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		result SearchResult
		score  float64
	}
	var candidates []scored

	for rows.Next() {
		var id, knowledgeID int64
		var filePath, chunkText string
		var chunkIndex int
		var blob []byte

		if err := rows.Scan(&id, &knowledgeID, &filePath, &chunkIndex, &chunkText, &blob); err != nil {
			continue
		}

		embedding := DeserializeFloat32(blob)
		if len(embedding) != EmbeddingDim {
			continue
		}

		sim := CosineSimilarity(queryEmbedding, embedding)
		candidates = append(candidates, scored{
			result: SearchResult{
				ChunkID:     id,
				Distance:    1 - sim, // distance = 1 - similarity
				KnowledgeID: knowledgeID,
				FilePath:    filePath,
				ChunkIndex:  chunkIndex,
				ChunkText:   chunkText,
			},
			score: sim,
		})
	}

	// 按相似度降序排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// 取 Top-K
	results := make([]SearchResult, 0, limit)
	for i := 0; i < len(candidates) && i < limit; i++ {
		results = append(results, candidates[i].result)
	}
	return results, nil
}

// GetIndexStatus 获取索引状态
func GetIndexStatus() (IndexStatus, error) {
	var s IndexStatus
	var lastUpdated sql.NullString
	err := VecDB.QueryRow(
		`SELECT total_docs, total_chunks, last_updated, is_indexing, progress FROM index_status WHERE id = 1`,
	).Scan(&s.TotalDocs, &s.TotalChunks, &lastUpdated, &s.IsIndexing, &s.Progress)
	if lastUpdated.Valid {
		s.LastUpdated = lastUpdated.String
	}
	return s, err
}

// UpdateIndexStatus 更新索引状态
func UpdateIndexStatus(totalDocs, totalChunks int, isIndexing bool, progress float64) {
	indexing := 0
	if isIndexing {
		indexing = 1
	}
	VecDB.Exec(
		`UPDATE index_status SET total_docs=?, total_chunks=?, last_updated=CURRENT_TIMESTAMP, is_indexing=?, progress=? WHERE id=1`,
		totalDocs, totalChunks, indexing, progress,
	)
}

// SearchResult 向量搜索结果
type SearchResult struct {
	ChunkID     int64   `json:"chunk_id"`
	Distance    float64 `json:"distance"`
	KnowledgeID int64   `json:"knowledge_id"`
	FilePath    string  `json:"file_path"`
	ChunkIndex  int     `json:"chunk_index"`
	ChunkText   string  `json:"chunk_text"`
}

// IndexStatus 索引状态
type IndexStatus struct {
	TotalDocs   int     `json:"total_docs"`
	TotalChunks int     `json:"total_chunks"`
	LastUpdated string  `json:"last_updated"`
	IsIndexing  bool    `json:"is_indexing"`
	Progress    float64 `json:"progress"`
}

// ─── Watched Dirs CRUD ───────────────────────────────────────────

type WatchedDir struct {
	ID        int64  `json:"id"`
	DirPath   string `json:"dir_path"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func ListWatchedDirs() ([]WatchedDir, error) {
	rows, err := VecDB.Query(`SELECT id, dir_path, enabled, created_at FROM watched_dirs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []WatchedDir
	for rows.Next() {
		var d WatchedDir
		var enabled int
		if err := rows.Scan(&d.ID, &d.DirPath, &enabled, &d.CreatedAt); err != nil {
			continue
		}
		d.Enabled = enabled == 1
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func AddWatchedDir(dirPath string) (int64, error) {
	res, err := VecDB.Exec(`INSERT OR IGNORE INTO watched_dirs (dir_path) VALUES (?)`, dirPath)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func RemoveWatchedDir(id int64) error {
	_, err := VecDB.Exec(`DELETE FROM watched_dirs WHERE id = ?`, id)
	return err
}

func ToggleWatchedDir(id int64, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := VecDB.Exec(`UPDATE watched_dirs SET enabled = ? WHERE id = ?`, e, id)
	return err
}

// ─── Embedding Config ────────────────────────────────────────────

type EmbeddingConfig struct {
	Mode   string `json:"mode"`
	APIURL string `json:"api_url"`
	Model  string `json:"model"`
}

func GetEmbeddingConfig() EmbeddingConfig {
	var c EmbeddingConfig
	VecDB.QueryRow(`SELECT mode, api_url, model FROM embedding_config WHERE id = 1`).Scan(&c.Mode, &c.APIURL, &c.Model)
	return c
}

func SetEmbeddingConfig(c EmbeddingConfig) error {
	_, err := VecDB.Exec(`UPDATE embedding_config SET mode=?, api_url=?, model=? WHERE id=1`, c.Mode, c.APIURL, c.Model)
	return err
}

// ChunkCount 返回当前总分块数
func ChunkCount() int {
	var count int
	VecDB.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&count)
	return count
}

// DocCount 返回已索引的不同文档数
func DocCount() int {
	var count int
	VecDB.QueryRow(`SELECT COUNT(DISTINCT file_path) FROM chunks`).Scan(&count)
	return count
}
