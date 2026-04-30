package connector

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	streamclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	sdklogger "github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
)

// DingtalkConfig 是钉钉连接器的配置
type DingtalkConfig struct {
	BaseConfig
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// DingtalkConnector 实现钉钉 Stream 模式机器人
type DingtalkConnector struct {
	cfg    DingtalkConfig
	cli    *streamclient.StreamClient
	cancel context.CancelFunc
}

func NewDingtalkConnector(configJSON string) (*DingtalkConnector, error) {
	cfg := DingtalkConfig{BaseConfig: DefaultBaseConfig()}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}
	return &DingtalkConnector{cfg: cfg}, nil
}

func (d *DingtalkConnector) Platform() string { return "dingtalk" }

func (d *DingtalkConnector) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	sdklogger.SetLogger(sdklogger.NewStdTestLoggerWithDebug())

	// 绕过系统代理（Clash/V2Ray 等），避免钉钉流量被转发到内网 IP
	os.Setenv("NO_PROXY", "*.dingtalk.com")
	os.Setenv("no_proxy", "*.dingtalk.com")

	d.cli = streamclient.NewStreamClient(
		streamclient.WithAppCredential(
			streamclient.NewAppCredentialConfig(d.cfg.ClientID, d.cfg.ClientSecret),
		),
	)
	d.cli.RegisterChatBotCallbackRouter(d.onMessage)

	log.Printf("[dingtalk] starting stream client, client_id=%s", d.cfg.ClientID)

	if err := d.cli.Start(ctx); err != nil {
		log.Printf("[dingtalk] stream client connect error: %v", err)
		return err
	}
	log.Printf("[dingtalk] stream client connected, waiting for messages...")

	// 阻塞直到 ctx 被取消（Stop 被调用）
	<-ctx.Done()
	d.cli.Close()
	log.Printf("[dingtalk] stream client closed")
	return nil
}

func (d *DingtalkConnector) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

func (d *DingtalkConnector) onMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	text := data.Text.Content
	log.Printf("[dingtalk] received message from=%s conv=%s text=%s", data.SenderStaffId, data.ConversationId, text)

	sessionWebhook := data.SessionWebhook

	replyFunc := func(reply string) error {
		replier := chatbot.NewChatbotReplier()
		log.Printf("[dingtalk] sending reply via SessionWebhook, len=%d", len(reply))
		err := replier.SimpleReplyMarkdown(ctx, sessionWebhook, []byte("回复"), []byte(reply))
		if err != nil {
			log.Printf("[dingtalk] reply error: %v", err)
		}
		return err
	}

	msg := IMMessage{
		Platform:       "dingtalk",
		UserID:         data.SenderStaffId,
		ConversationID: data.ConversationId,
		Text:           text,
		BaseCfg:        d.cfg.BaseConfig,
		ReplyFunc:      replyFunc,
	}
	// Dispatch 内部会立即回复"收到"，然后异步调用 Claude，
	// 所以这里直接调用即可（不会阻塞超过钉钉 3 秒限制）
	Dispatch(msg)
	return []byte(""), nil
}
