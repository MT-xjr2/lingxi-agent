package connector

import "context"

// SessionMode 决定 IM 消息如何映射到 Claude session
type SessionMode string

const (
	// SessionModePerGroup 同一个群共享一个 session（默认）
	SessionModePerGroup SessionMode = "per_group"
	// SessionModePerUser 同一个用户（跨群）共享一个 session
	SessionModePerUser SessionMode = "per_user"
	// SessionModePerGroupUser 同一个群内同一个用户独立 session
	SessionModePerGroupUser SessionMode = "per_group_user"
	// SessionModeStateless 每条消息独立，不保留上下文
	SessionModeStateless SessionMode = "stateless"
)

// BaseConfig 是所有平台连接器共用的会话管理配置
type BaseConfig struct {
	// SessionMode 会话粒度，默认 per_group
	SessionMode SessionMode `json:"session_mode"`
	// SessionTTLHours 不活跃多少小时后自动开启新 session，0 表示永不重置，默认 24
	SessionTTLHours int `json:"session_ttl_hours"`
}

// DefaultBaseConfig 返回默认会话配置
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		SessionMode:     SessionModePerGroup,
		SessionTTLHours: 24,
	}
}

// IMMessage 是各平台消息的统一抽象
type IMMessage struct {
	Platform       string // "dingtalk" | "feishu" | "wecom"
	UserID         string // 发送者 ID
	ConversationID string // 会话/群 ID（用于区分多用户上下文）
	Text           string // 消息正文
	BaseCfg        BaseConfig
	// ReplyFunc 由各平台连接器实现，dispatcher 调用它发送回复
	ReplyFunc func(text string) error
}

// Connector 是每个 IM 平台连接器必须实现的接口
type Connector interface {
	// Platform 返回平台标识，如 "dingtalk"
	Platform() string
	// Start 启动连接器（建立长连接或注册 Webhook 路由），阻塞直到 ctx 取消
	Start(ctx context.Context) error
	// Stop 停止连接器，释放资源
	Stop()
}
