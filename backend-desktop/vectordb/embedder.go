package vectordb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"lingxi-agent/db"
)

// Embedder 嵌入接口
type Embedder interface {
	Embed(texts []string) ([][]float32, error)
	Dimension() int
}

// APIEmbedder 通过 API 调用远端嵌入模型
type APIEmbedder struct {
	BaseURL string
	Model   string
	APIKey  string
}

// GetActiveEmbedder 根据配置返回当前激活的嵌入器
func GetActiveEmbedder() (Embedder, error) {
	cfg := GetEmbeddingConfig()

	// 从已激活的 API Profile 获取密钥和 base_url
	profile := getActiveProfile()
	if profile == nil {
		return nil, fmt.Errorf("no active API profile configured")
	}

	baseURL := cfg.APIURL
	if baseURL == "" {
		baseURL = profile.BaseURL
	}
	model := cfg.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	// 确保 base URL 正确
	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	return &APIEmbedder{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  profile.APIKey,
	}, nil
}

func (e *APIEmbedder) Dimension() int {
	return EmbeddingDim
}

// Embed 批量嵌入文本，调用 OpenAI 兼容的 /embeddings 端点
func (e *APIEmbedder) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 分批处理（每批最多 20 条，避免超限）
	const batchSize = 20
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := e.embedBatch(batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch %d-%d failed: %w", i, end, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (e *APIEmbedder) embedBatch(texts []string) ([][]float32, error) {
	url := e.BaseURL + "/embeddings"

	reqBody := map[string]interface{}{
		"model": e.Model,
		"input": texts,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			embedding := make([]float32, len(d.Embedding))
			for i, v := range d.Embedding {
				embedding[i] = float32(v)
			}
			// 如果维度不匹配，做截断或补零
			embedding = normalizeEmbedding(embedding, EmbeddingDim)
			embeddings[d.Index] = embedding
		}
	}

	return embeddings, nil
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// normalizeEmbedding 将向量调整为目标维度（截断或补零）
func normalizeEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) == targetDim {
		return embedding
	}
	if len(embedding) > targetDim {
		return embedding[:targetDim]
	}
	result := make([]float32, targetDim)
	copy(result, embedding)
	return result
}

// getActiveProfile 从数据库获取当前激活的 API Profile
func getActiveProfile() *apiProfile {
	profiles, err := db.ListAPIProfiles(true)
	if err != nil {
		slog.Error("list API profiles failed", "err", err)
		return nil
	}

	for _, p := range profiles {
		if p.IsActive {
			return &apiProfile{
				BaseURL: p.BaseURL,
				APIKey:  p.AuthTokenCipher,
			}
		}
	}

	// 回退到第一个配置
	if len(profiles) > 0 {
		return &apiProfile{
			BaseURL: profiles[0].BaseURL,
			APIKey:  profiles[0].AuthTokenCipher,
		}
	}

	return nil
}

type apiProfile struct {
	BaseURL string
	APIKey  string
}
