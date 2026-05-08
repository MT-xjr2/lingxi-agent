// Package usage 提供面向不同 LLM 供应商的账户额度/用量查询适配器。
//
// 当前支持：
//   - dashscope_anthropic DashScope (Anthropic Compatible)
//   - deepseek_anthropic  DeepSeek
//   - anthropic_official  Anthropic 官方（暂仅返回 unavailable）
//   - 其它：根据 provider.UsageAPIMeta 中的 endpoint/headers 自动转发
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Quota 是统一的对外结构
type Quota struct {
	Available  bool                   `json:"available"`
	Provider   string                 `json:"provider"`
	Currency   string                 `json:"currency,omitempty"`
	Balance    string                 `json:"balance,omitempty"`
	Granted    string                 `json:"granted,omitempty"`
	Used       string                 `json:"used,omitempty"`
	ExpireAt   string                 `json:"expire_at,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"`
	FetchedAt  time.Time              `json:"fetched_at"`
}

// FetchQuota 根据 providerCode + meta(JSON) 调用对应供应商账户接口
func FetchQuota(providerCode, meta, baseURL, token string) (*Quota, error) {
	switch providerCode {
	case "dashscope_anthropic", "bailian_anthropic":
		return fetchDashscope(baseURL, token, meta)
	case "deepseek_anthropic":
		return fetchDeepseek(baseURL, token, meta)
	case "anthropic_official":
		return &Quota{Available: false, Provider: providerCode, FetchedAt: time.Now()},
			errors.New("Anthropic 官方暂不开放账户额度查询接口")
	default:
		return fetchGeneric(providerCode, meta, token)
	}
}

// ─── DashScope ──────────────────────────────────────────────────
// 真实接口：GET https://dashscope.aliyuncs.com/api/v1/account/balance
// 不同账号开通情况下返回结构略有差异，这里宽松解析
func fetchDashscope(_ string, token string, _ string) (*Quota, error) {
	endpoint := "https://dashscope.aliyuncs.com/api/v1/account/balance"
	body, err := getJSON(endpoint, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return nil, err
	}
	q := &Quota{Available: true, Provider: "dashscope_anthropic", Currency: "CNY", Raw: body, FetchedAt: time.Now()}
	if data, ok := body["data"].(map[string]interface{}); ok {
		if v, ok := data["balance"]; ok {
			q.Balance = fmt.Sprintf("%v", v)
		}
		if v, ok := data["total_quota"]; ok {
			q.Granted = fmt.Sprintf("%v", v)
		}
		if v, ok := data["used_quota"]; ok {
			q.Used = fmt.Sprintf("%v", v)
		}
	}
	return q, nil
}

// ─── DeepSeek ───────────────────────────────────────────────────
// GET https://api.deepseek.com/user/balance  Authorization: Bearer <key>
// 文档形式：{ "balance_infos": [ { "currency":"CNY", "total_balance":"...", "granted_balance":"...", "topped_up_balance":"..." } ] }
func fetchDeepseek(_ string, token string, _ string) (*Quota, error) {
	endpoint := "https://api.deepseek.com/user/balance"
	body, err := getJSON(endpoint, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return nil, err
	}
	q := &Quota{Available: true, Provider: "deepseek_anthropic", Raw: body, FetchedAt: time.Now()}
	if arr, ok := body["balance_infos"].([]interface{}); ok && len(arr) > 0 {
		if first, ok := arr[0].(map[string]interface{}); ok {
			if v, ok := first["currency"].(string); ok {
				q.Currency = v
			}
			if v, ok := first["total_balance"]; ok {
				q.Balance = fmt.Sprintf("%v", v)
			}
			if v, ok := first["granted_balance"]; ok {
				q.Granted = fmt.Sprintf("%v", v)
			}
		}
	}
	return q, nil
}

// ─── 通用：根据 provider.UsageAPIMeta 转发 ─────────────────────
//
// meta 形如：{ "usage": { "endpoint":"...", "auth_header":"Authorization", "auth_prefix":"Bearer " } }
func fetchGeneric(providerCode, meta, token string) (*Quota, error) {
	if meta == "" {
		return &Quota{Available: false, Provider: providerCode, FetchedAt: time.Now()},
			errors.New("provider 未配置 usage_api_meta")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(meta), &m); err != nil {
		return nil, fmt.Errorf("usage_api_meta 解析失败: %w", err)
	}
	u, _ := m["usage"].(map[string]interface{})
	if u == nil {
		return &Quota{Available: false, Provider: providerCode, FetchedAt: time.Now()},
			errors.New("provider 未声明 usage 字段")
	}
	endpoint, _ := u["endpoint"].(string)
	if endpoint == "" {
		return nil, errors.New("usage.endpoint 为空")
	}
	authHeader, _ := u["auth_header"].(string)
	if authHeader == "" {
		authHeader = "Authorization"
	}
	authPrefix, _ := u["auth_prefix"].(string)
	body, err := getJSON(endpoint, map[string]string{
		authHeader: authPrefix + token,
	})
	if err != nil {
		return nil, err
	}
	return &Quota{
		Available: true, Provider: providerCode, Raw: body,
		FetchedAt: time.Now(),
	}, nil
}

// ─── 工具 ────────────────────────────────────────────────────────

func getJSON(url string, headers map[string]string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateForErr(string(bs)))
	}
	var m map[string]interface{}
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return m, nil
}

func truncateForErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
