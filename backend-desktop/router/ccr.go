// Package router 负责把 OpenAI 协议的供应商通过 claude-code-router (CCR)
// 暴露成 Anthropic 协议本地端点，给底层 Claude CLI 使用。
//
// 工作流程：
//
//	UI → 激活 protocol=openai 的 profile
//	   → 后端写出 ccr-home/config.json（仅含该 profile）
//	   → spawn ccr start（HOME 隔离到 ccr-home）
//	   → 健康检查 /health
//	   → buildClaudeEnv 把 ANTHROPIC_BASE_URL 指向 127.0.0.1:<port>
//
// 当激活的是 protocol=anthropic 时，CCR 会被 Stop()，恢复直连 Anthropic 上游。
package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── 配置：CCR 二进制位置 / 数据目录 ────────────────────────────────

// CCRBinEnv 环境变量名：可由 Electron 主进程注入指向打包内置的 ccr 包装脚本
const CCRBinEnv = "CCR_BIN"

// CCRHomeEnv 环境变量名：可由 Electron 注入隔离的 ccr-home 目录（默认 $HOME/.lingxi-ccr）
const CCRHomeEnv = "CCR_HOME"

// resolveBin 返回 ccr 可执行文件路径
func resolveBin() string {
	if v := os.Getenv(CCRBinEnv); v != "" {
		return v
	}
	// dev fallback
	if p, err := exec.LookPath("ccr"); err == nil {
		return p
	}
	return "ccr"
}

// resolveHome 返回 CCR 数据目录（隔离，不污染用户的 ~/.claude-code-router/）
func resolveHome() string {
	if v := os.Getenv(CCRHomeEnv); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lingxi-ccr")
}

// ─── 路由档案描述（来自 db.APIProfile + 明文 token）─────────────

// Profile 描述一个待路由的 OpenAI 协议档案
type Profile struct {
	ID          int64  // 档案 ID
	Name        string // 档案名（仅日志）
	BaseURL     string // OpenAI 协议端点（含 /chat/completions）
	Model       string // 模型名
	Token       string // 明文 API key
	Transformer string // 可选：CCR transformer 名（如 deepseek / gemini / tooluse）
}

// ─── 单例：CCR 进程管理 ─────────────────────────────────────────

type manager struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	port      int
	profileID int64 // 当前 CCR 服务的 profile ID
	startedAt time.Time
	lastErr   string
	logTail   []string // 最近若干行 stderr/stdout
}

var mgr = &manager{}

// EnsureRunning 保证 CCR 已为指定 profile 启动并健康，返回本地 baseURL
//
// 如果当前 CCR 已经在为同一 profile 服务，直接复用；否则先 Stop 再 Start。
func EnsureRunning(p Profile) (string, error) {
	if p.Token == "" {
		return "", errors.New("CCR 路由需要 profile 明文 token，但未由 Electron 下发")
	}
	if p.BaseURL == "" {
		return "", errors.New("OpenAI 档案的 base_url 不能为空")
	}
	if p.Model == "" {
		return "", errors.New("OpenAI 档案的 model 不能为空")
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.cmd != nil && mgr.cmd.Process != nil && mgr.profileID == p.ID && mgr.port > 0 {
		// 进程仍在 + 同一 profile，直接复用
		if isAlive(mgr.cmd) && pingHealth(mgr.port, 200*time.Millisecond) {
			return fmt.Sprintf("http://127.0.0.1:%d", mgr.port), nil
		}
		log.Printf("[router] previous ccr unhealthy, restarting")
		stopLocked()
	} else if mgr.cmd != nil {
		log.Printf("[router] switching profile %d → %d, restarting ccr", mgr.profileID, p.ID)
		stopLocked()
	}

	port, err := pickFreePort()
	if err != nil {
		return "", fmt.Errorf("pick free port: %w", err)
	}

	if err := writeConfig(p, port); err != nil {
		return "", fmt.Errorf("write ccr config: %w", err)
	}

	if err := startLocked(p, port); err != nil {
		return "", err
	}

	if err := waitHealthy(port, 8*time.Second); err != nil {
		stopLocked()
		return "", fmt.Errorf("ccr start timeout: %w", err)
	}

	return fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

// Stop 关闭 CCR 进程（用户切回 anthropic 协议时调用）
func Stop() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	stopLocked()
}

// Status 返回 CCR 当前状态供前端展示
type Status struct {
	Running   bool     `json:"running"`
	Port      int      `json:"port,omitempty"`
	ProfileID int64    `json:"profile_id,omitempty"`
	StartedAt string   `json:"started_at,omitempty"`
	LastErr   string   `json:"last_err,omitempty"`
	LogTail   []string `json:"log_tail,omitempty"`
	Bin       string   `json:"bin"`
	Home      string   `json:"home"`
}

func GetStatus() Status {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	s := Status{Bin: resolveBin(), Home: resolveHome(), LastErr: mgr.lastErr}
	if mgr.cmd != nil && mgr.cmd.Process != nil && isAlive(mgr.cmd) {
		s.Running = true
		s.Port = mgr.port
		s.ProfileID = mgr.profileID
		s.StartedAt = mgr.startedAt.Format(time.RFC3339)
	}
	if len(mgr.logTail) > 0 {
		s.LogTail = append([]string(nil), mgr.logTail...)
	}
	return s
}

// ─── 内部：启动 / 停止 / 健康检查 ────────────────────────────────

func startLocked(p Profile, port int) error {
	ctx, cancel := context.WithCancel(context.Background())
	bin := resolveBin()
	home := resolveHome()
	if err := os.MkdirAll(home, 0o755); err != nil {
		cancel()
		return err
	}

	cmd := exec.CommandContext(ctx, bin, "start")
	// 用独立的 HOME，让 CCR 把 config / log 都写到 ccr-home 下
	env := os.Environ()
	env = upsertEnv(env, "HOME", home)
	env = upsertEnv(env, "CCR_HOST", "127.0.0.1")
	env = upsertEnv(env, "CCR_PORT", fmt.Sprintf("%d", port))
	cmd.Env = env

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		mgr.lastErr = err.Error()
		return fmt.Errorf("start ccr: %w", err)
	}
	mgr.cmd = cmd
	mgr.cancel = cancel
	mgr.port = port
	mgr.profileID = p.ID
	mgr.startedAt = time.Now()
	mgr.lastErr = ""
	log.Printf("[router] ccr started pid=%d port=%d profile=%d (%s)", cmd.Process.Pid, port, p.ID, p.Name)

	go pumpLog("ccr/out", stdout)
	go pumpLog("ccr/err", stderr)
	go func() {
		_ = cmd.Wait()
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		if mgr.cmd == cmd {
			log.Printf("[router] ccr exited unexpectedly")
			mgr.cmd = nil
			mgr.cancel = nil
			mgr.port = 0
			mgr.profileID = 0
			mgr.lastErr = "ccr exited"
		}
	}()
	return nil
}

func stopLocked() {
	if mgr.cancel != nil {
		mgr.cancel()
	}
	if mgr.cmd != nil && mgr.cmd.Process != nil {
		// 先尝试优雅退出
		_ = mgr.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = mgr.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = mgr.cmd.Process.Kill()
		}
	}
	mgr.cmd = nil
	mgr.cancel = nil
	mgr.port = 0
	mgr.profileID = 0
}

func pumpLog(tag string, r io.ReadCloser) {
	defer r.Close()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			line := strings.TrimRight(string(buf[:n]), "\n")
			log.Printf("[%s] %s", tag, line)
			mgr.mu.Lock()
			mgr.logTail = append(mgr.logTail, line)
			if len(mgr.logTail) > 50 {
				mgr.logTail = mgr.logTail[len(mgr.logTail)-50:]
			}
			mgr.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func isAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	// Signal(0) 在 unix 下用于探测进程是否存活而不发送实际信号
	return cmd.Process.Signal(zeroSignal) == nil
}

func waitHealthy(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pingHealth(port, 300*time.Millisecond) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("ccr did not become healthy on :%d within %v", port, timeout)
}

func pingHealth(port int, timeout time.Duration) bool {
	cli := &http.Client{Timeout: timeout}
	// CCR 兼容若干路径，先尝试 /health，失败时尝试 /v1/messages 的 OPTIONS
	for _, path := range []string{"/health", "/api/health", "/"} {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
		resp, err := cli.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return true
			}
		}
	}
	return false
}

func pickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func upsertEnv(env []string, key, val string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// ─── CCR 配置文件渲染 ───────────────────────────────────────────

// writeConfig 在 ccr-home 下写出 .claude-code-router/config.json
//
// CCR 实际读取的路径是 $HOME/.claude-code-router/config.json
func writeConfig(p Profile, port int) error {
	home := resolveHome()
	dir := filepath.Join(home, ".claude-code-router")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	providerName := strings.ReplaceAll(p.Name, " ", "_")
	if providerName == "" {
		providerName = fmt.Sprintf("profile_%d", p.ID)
	}

	provider := map[string]interface{}{
		"name":         providerName,
		"api_base_url": p.BaseURL,
		"api_key":      p.Token,
		"models":       []string{p.Model},
	}
	if p.Transformer != "" {
		provider["transformer"] = map[string]interface{}{"use": []string{p.Transformer}}
	}

	cfg := map[string]interface{}{
		"HOST":          "127.0.0.1",
		"PORT":          port,
		"LOG":           true,
		"LOG_LEVEL":     "info",
		"API_TIMEOUT_MS": 600000,
		"Providers": []interface{}{provider},
		"Router": map[string]interface{}{
			"default":     fmt.Sprintf("%s,%s", providerName, p.Model),
			"background":  fmt.Sprintf("%s,%s", providerName, p.Model),
			"think":       fmt.Sprintf("%s,%s", providerName, p.Model),
			"longContext": fmt.Sprintf("%s,%s", providerName, p.Model),
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
