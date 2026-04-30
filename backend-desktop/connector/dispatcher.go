package connector

import (
	"fmt"
	"log"
	"strings"

	"lingxi-agent/db"
)

// ClaudeRunner 是调用 Claude 的函数签名，由外部注入以避免循环依赖
type ClaudeRunner func(message string, sessionID int64) (reply string, usedSessionID int64, err error)

var runClaude ClaudeRunner

// SetClaudeRunner 由 main 包在启动时注入 handler.RunClaudeSync
func SetClaudeRunner(fn ClaudeRunner) {
	runClaude = fn
}

// Dispatch 接收来自任意平台的 IMMessage，立即回复"收到"，
// 然后在后台 goroutine 中调用 Claude 并发送完整结果。
func Dispatch(msg IMMessage) {
	if strings.TrimSpace(msg.Text) == "" {
		return
	}
	if runClaude == nil {
		log.Printf("[connector] ClaudeRunner not set, dropping message")
		return
	}

	// 立即回复"收到"，让用户知道消息已被接收
	if msg.ReplyFunc != nil {
		if err := msg.ReplyFunc("收到，正在为您分析问题，请稍候..."); err != nil {
			log.Printf("[connector] ack reply error: %v", err)
		}
	}

	// 异步执行 Claude 调用，完成后发送完整结果
	go func() {
		cfg := msg.BaseCfg
		if cfg.SessionMode == "" {
			cfg.SessionMode = SessionModePerGroup
		}
		if cfg.SessionTTLHours == 0 && cfg.SessionMode != SessionModeStateless {
			cfg.SessionTTLHours = 24
		}

		scopeKey := computeScopeKey(msg, cfg.SessionMode)
		log.Printf("[connector] dispatch platform=%s mode=%s scope=%s", msg.Platform, cfg.SessionMode, scopeKey)

		var sessionID int64
		if cfg.SessionMode != SessionModeStateless {
			title := buildSessionTitle(msg)
			sid, err := db.GetOrCreateIMSession(msg.Platform, scopeKey, title, cfg.SessionTTLHours)
			if err != nil {
				log.Printf("[connector] GetOrCreateIMSession error: %v", err)
				if msg.ReplyFunc != nil {
					_ = msg.ReplyFunc("抱歉，初始化会话失败，请稍后再试。")
				}
				return
			}
			sessionID = sid
		}

		reply, _, err := runClaude(msg.Text, sessionID)
		if err != nil {
			log.Printf("[connector] RunClaudeSync error: %v", err)
			if msg.ReplyFunc != nil {
				_ = msg.ReplyFunc("抱歉，处理消息时出现错误，请稍后再试。")
			}
			return
		}

		if cfg.SessionMode != SessionModeStateless && scopeKey != "" {
			db.TouchIMSession(msg.Platform, scopeKey)
		}

		reply = filterStateMarkers(reply)
		if reply == "" {
			reply = "好的，已处理完成。"
		}

		if msg.ReplyFunc != nil {
			if err := msg.ReplyFunc(reply); err != nil {
				log.Printf("[connector] ReplyFunc error: %v", err)
			}
		}
	}()
}

// computeScopeKey 根据 session_mode 计算会话的唯一键
func computeScopeKey(msg IMMessage, mode SessionMode) string {
	switch mode {
	case SessionModePerGroup:
		// 单聊时 ConversationID 就是用户 ID，群聊时是群 ID，两者都适用
		return msg.ConversationID
	case SessionModePerUser:
		return msg.UserID
	case SessionModePerGroupUser:
		return fmt.Sprintf("%s:%s", msg.ConversationID, msg.UserID)
	default:
		return ""
	}
}

// buildSessionTitle 为新建 session 生成标题
func buildSessionTitle(msg IMMessage) string {
	title := fmt.Sprintf("[%s] %s", msg.Platform, msg.ConversationID)
	runes := []rune(title)
	if len(runes) > 30 {
		return string(runes[:30]) + "…"
	}
	return title
}

// filterStateMarkers 移除 AI 输出中的内部状态标记 JSON 片段
func filterStateMarkers(text string) string {
	var result strings.Builder
	remaining := text
	for {
		idx := strings.Index(remaining, "{")
		if idx < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		depth, end := 0, -1
		for i := idx; i < len(remaining); i++ {
			switch remaining[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			result.WriteString(remaining[idx:])
			break
		}
		fragment := remaining[idx : end+1]
		// 只过滤含 "state" 字段的状态标记，其他 JSON 保留
		if strings.Contains(fragment, `"state"`) {
			remaining = remaining[end+1:]
			continue
		}
		result.WriteString(fragment)
		remaining = remaining[end+1:]
	}
	return strings.TrimSpace(result.String())
}
