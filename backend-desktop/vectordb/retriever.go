package vectordb

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"
)

// RetrievalResult 最终检索结果（融合后）
type RetrievalResult struct {
	ChunkID     int64   `json:"chunk_id"`
	KnowledgeID int64   `json:"knowledge_id"`
	FilePath    string  `json:"file_path"`
	ChunkIndex  int     `json:"chunk_index"`
	ChunkText   string  `json:"chunk_text"`
	Score       float64 `json:"score"`
	Source      string  `json:"source"` // "vector" / "keyword" / "hybrid"
}

// HybridSearch 混合检索：向量搜索 + 关键词搜索 + RRF 融合排序
func HybridSearch(query string, topK int, knowledgeIDs []int64) ([]RetrievalResult, error) {
	if VecDB == nil {
		return nil, fmt.Errorf("vectordb not initialized")
	}

	var vectorResults []SearchResult
	var keywordResults []keywordMatch

	// 1. 向量搜索
	embedder, err := GetActiveEmbedder()
	if err == nil {
		embeddings, err := embedder.Embed([]string{query})
		if err == nil && len(embeddings) > 0 && len(embeddings[0]) == EmbeddingDim {
			vectorResults, _ = SearchByVector(embeddings[0], topK*2, knowledgeIDs)
		} else if err != nil {
			slog.Debug("vector search skipped", "err", err)
		}
	} else {
		slog.Debug("embedder unavailable, keyword-only mode", "err", err)
	}

	// 2. 关键词搜索（BM25 简化版）
	keywordResults = keywordSearch(query, topK*2, knowledgeIDs)

	// 3. RRF 融合排序
	results := rrfFusion(vectorResults, keywordResults, topK)

	return results, nil
}

// VectorOnlySearch 仅向量搜索（用于语义搜索 UI）
func VectorOnlySearch(query string, topK int, knowledgeIDs []int64) ([]RetrievalResult, error) {
	if VecDB == nil {
		return nil, fmt.Errorf("vectordb not initialized")
	}

	embedder, err := GetActiveEmbedder()
	if err != nil {
		return nil, err
	}

	embeddings, err := embedder.Embed([]string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 || len(embeddings[0]) != EmbeddingDim {
		return nil, fmt.Errorf("invalid embedding result")
	}

	results, err := SearchByVector(embeddings[0], topK, knowledgeIDs)
	if err != nil {
		return nil, err
	}

	var out []RetrievalResult
	for _, r := range results {
		out = append(out, RetrievalResult{
			ChunkID:     r.ChunkID,
			KnowledgeID: r.KnowledgeID,
			FilePath:    r.FilePath,
			ChunkIndex:  r.ChunkIndex,
			ChunkText:   r.ChunkText,
			Score:       1.0 / (1.0 + r.Distance),
			Source:      "vector",
		})
	}
	return out, nil
}

// ─── 关键词搜索 ─────────────────────────────────────────────────

type keywordMatch struct {
	ChunkID     int64
	KnowledgeID int64
	FilePath    string
	ChunkIndex  int
	ChunkText   string
	Score       float64
}

func keywordSearch(query string, limit int, knowledgeIDs []int64) []keywordMatch {
	keywords := extractSearchKeywords(query)
	if len(keywords) == 0 {
		return nil
	}

	// 构建 SQL 查询
	baseQuery := `SELECT id, knowledge_id, file_path, chunk_index, chunk_text FROM chunks`
	var conditions []string
	var args []interface{}

	if len(knowledgeIDs) > 0 {
		placeholders := make([]string, len(knowledgeIDs))
		for i, id := range knowledgeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "knowledge_id IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := VecDB.Query(baseQuery, args...)
	if err != nil {
		slog.Error("keyword search query failed", "err", err)
		return nil
	}
	defer rows.Close()

	var matches []keywordMatch
	for rows.Next() {
		var m keywordMatch
		if err := rows.Scan(&m.ChunkID, &m.KnowledgeID, &m.FilePath, &m.ChunkIndex, &m.ChunkText); err != nil {
			continue
		}

		score := computeKeywordScore(m.ChunkText, keywords)
		if score > 0 {
			m.Score = score
			matches = append(matches, m)
		}
	}

	// 按分数排序取 Top-K
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// computeKeywordScore 计算关键词匹配分数（简化 BM25）
func computeKeywordScore(text string, keywords []string) float64 {
	textLower := strings.ToLower(text)
	textLen := utf8.RuneCountInString(text)
	if textLen == 0 {
		return 0
	}

	score := 0.0
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		count := strings.Count(textLower, kwLower)
		if count > 0 {
			// 简化 BM25：tf * idf（无文档频率，用词频 + 长度归一化替代）
			tf := float64(count) / (float64(count) + 1.2*(1.0-0.75+0.75*float64(textLen)/500.0))
			score += tf
		}
	}
	return score
}

// extractSearchKeywords 从查询中提取搜索关键词
func extractSearchKeywords(query string) []string {
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true, "有": true,
		"和": true, "就": true, "不": true, "都": true, "一个": true, "也": true,
		"很": true, "到": true, "要": true, "你": true, "会": true, "没有": true,
		"什么": true, "怎么": true, "如何": true, "哪些": true, "哪个": true,
		"吗": true, "吧": true, "呢": true, "啊": true, "还": true, "能": true,
		"可以": true, "请": true, "帮": true, "帮我": true, "告诉": true,
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
		"be": true, "have": true, "has": true, "had": true, "do": true, "does": true,
		"will": true, "would": true, "can": true, "to": true, "of": true, "in": true,
		"for": true, "on": true, "with": true, "it": true, "this": true, "that": true,
		"what": true, "how": true, "which": true, "and": true, "or": true, "but": true,
	}

	fields := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '，' || r == '。' || r == '？' || r == '！' ||
			r == '、' || r == '：' || r == '\n' || r == '\t' ||
			r == ',' || r == '.' || r == '?' || r == '!' || r == ':' || r == ';'
	})

	var keywords []string
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		lower := strings.ToLower(f)
		if f == "" || len([]rune(f)) < 2 || stopWords[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		keywords = append(keywords, f)
	}
	return keywords
}

// ─── RRF 融合 ───────────────────────────────────────────────────

const rrfK = 60.0 // RRF 常数参数

// rrfFusion 使用 Reciprocal Rank Fusion 融合两路检索结果
func rrfFusion(vectorResults []SearchResult, keywordResults []keywordMatch, topK int) []RetrievalResult {
	type fusionEntry struct {
		result RetrievalResult
		score  float64
	}

	scoreMap := make(map[int64]*fusionEntry)

	// 向量结果：按排名给分
	for rank, r := range vectorResults {
		score := 1.0 / (rrfK + float64(rank+1))
		entry, exists := scoreMap[r.ChunkID]
		if !exists {
			entry = &fusionEntry{
				result: RetrievalResult{
					ChunkID:     r.ChunkID,
					KnowledgeID: r.KnowledgeID,
					FilePath:    r.FilePath,
					ChunkIndex:  r.ChunkIndex,
					ChunkText:   r.ChunkText,
					Source:      "vector",
				},
			}
			scoreMap[r.ChunkID] = entry
		}
		entry.score += score
	}

	// 关键词结果：按排名给分
	for rank, r := range keywordResults {
		score := 1.0 / (rrfK + float64(rank+1))
		entry, exists := scoreMap[r.ChunkID]
		if !exists {
			entry = &fusionEntry{
				result: RetrievalResult{
					ChunkID:     r.ChunkID,
					KnowledgeID: r.KnowledgeID,
					FilePath:    r.FilePath,
					ChunkIndex:  r.ChunkIndex,
					ChunkText:   r.ChunkText,
					Source:      "keyword",
				},
			}
			scoreMap[r.ChunkID] = entry
		} else {
			entry.result.Source = "hybrid"
		}
		entry.score += score
	}

	// 按融合分数排序
	var entries []fusionEntry
	for _, e := range scoreMap {
		e.result.Score = e.score
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	// 取 Top-K
	if len(entries) > topK {
		entries = entries[:topK]
	}

	results := make([]RetrievalResult, len(entries))
	for i, e := range entries {
		results[i] = e.result
	}
	return results
}
