package nexus

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"lingxi-agent/db"
)

// StreamForwarder 转发流式 token 到远端（text/thinking 事件）
type StreamForwarder func(event, data string)

// A2AStreamRunner 执行一轮流式 A2A 对话，返回完整文本回复
type A2AStreamRunner func(sessionID int64, message string, agentID int64, forwarder StreamForwarder) (reply string, err error)

// A2ASessionCreator 创建 A2A 专用会话
type A2ASessionCreator func(title string, agentID int64) (sessionID int64, err error)

// BroadcastFunc 由 main 包注入
type BroadcastFunc func(event, data string)

var (
	streamRunner   A2AStreamRunner
	sessionCreator A2ASessionCreator
	broadcast      BroadcastFunc

	pausedConvs sync.Map
	convMutexes sync.Map
)

func getConvMutex(convID int64) *sync.Mutex {
	v, _ := convMutexes.LoadOrStore(convID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Init 注入依赖
func Init(runner A2AStreamRunner, creator A2ASessionCreator, broadcastFn BroadcastFunc) {
	streamRunner = runner
	sessionCreator = creator
	broadcast = broadcastFn
}

// PauseConversation 标记某对话为暂停（中断执行循环）
func PauseConversation(convID int64) {
	if ch, ok := pausedConvs.Load(convID); ok {
		select {
		case ch.(chan struct{}) <- struct{}{}:
		default:
		}
	}
}

// GetTransportForPeer 根据 peer ID 获取传输层（WAN 通过信令中继，LAN 通过 mDNS 发现的 host:port）
func GetTransportForPeer(peerID string) Transport {
	// 先检查是否为 LAN peer（mDNS 发现的）
	peers, _ := db.ListNexusPeers()
	for _, p := range peers {
		if p.ID == peerID {
			return NewLANTransport(p.Host, p.Port)
		}
	}
	// 默认走 WAN
	return NewWANTransport(peerID)
}

// buildStreamForwarder 构建一个将流式 token 转发到远端的函数
func buildStreamForwarder(conv *db.A2AConversation) StreamForwarder {
	transport := GetTransportForPeer(conv.RemotePeerID)

	return func(event, data string) {
		payload := map[string]interface{}{
			"conversation_id": conv.ID,
			"conv_uuid":       conv.ConvUUID,
			"event":           event,
			"data":            data,
		}
		if event == "stream_done" || event == "stream_start" {
			transport.Send("/conversation/stream-token", payload)
		} else {
			go func() {
				transport.Send("/conversation/stream-token", payload)
			}()
		}
	}
}

// RunConversation 启动对话引擎（发起方调用，对方已接受）
func RunConversation(convID int64, sessionID int64) {
	mu := getConvMutex(convID)
	mu.Lock()
	defer mu.Unlock()

	conv, err := db.GetA2AConversation(convID)
	if err != nil {
		log.Printf("[nexus] conversation %d not found: %v", convID, err)
		return
	}

	pauseCh := make(chan struct{}, 1)
	pausedConvs.Store(convID, pauseCh)
	defer pausedConvs.Delete(convID)

	agent, _ := db.GetAgent(conv.LocalAgentID)
	agentName := "灵犀助理"
	if agent != nil {
		agentName = agent.Name
	}

	forwarder := buildStreamForwarder(conv)
	firstMessage := buildA2AFirstMessage(conv, agent)

	reply, err := streamRunner(sessionID, firstMessage, conv.LocalAgentID, forwarder)
	if err != nil {
		log.Printf("[nexus] conv %d first message error: %v", convID, err)
		db.UpdateA2AConversationStatus(convID, "failed")
		saveErrorMessage(convID, agentName, fmt.Sprintf("Agent 执行出错: %v", err))
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"failed","error":"%s"}`, convID, escapeJSON(err.Error())))
		}
		return
	}

	if strings.TrimSpace(reply) == "" {
		log.Printf("[nexus] conv %d: agent returned empty reply for first message", convID)
		saveErrorMessage(convID, agentName, "Agent 未能生成回复，请检查 AI 引擎配置")
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"empty_reply"}`, convID))
		}
		return
	}

	content := strings.TrimSpace(reply)

	msg := &db.A2AMessage{
		ConversationID:  convID,
		Sender:          "local",
		SenderAgentName: agentName,
		MsgType:         "message",
		Content:         content,
		StructuredData:  "{}",
	}
	msgID, _ := db.CreateA2AMessage(msg)
	msg.ID = msgID
	newRound := conv.CurrentRound + 1
	db.UpdateA2AConversationRound(convID, newRound)
	broadcastMessage(msg)

	// 检测 [CLOSE] 标记：Agent 认为对话目标已达成，自动结束
	if isCloseMessage(content) {
		closeConversation(convID, conv)
		return
	}

	if newRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
		}
		return
	}

	// 等待 stream_done 到达对方后再发送完整消息，避免对方 Agent 抢先回复
	time.Sleep(500 * time.Millisecond)

	// 发送回复给对方
	transport := GetTransportForPeer(conv.RemotePeerID)
	sendPayload := map[string]interface{}{
		"conv_uuid":         conv.ConvUUID,
		"sender":            "remote",
		"sender_agent_name": agentName,
		"msg_type":          "message",
		"content":           content,
		"structured_data":   "{}",
	}
	log.Printf("[nexus] conv %d: sending first reply via %s transport, contentLen=%d",
		convID, transport.Type(), len(content))
	if _, err := transport.Send("/conversation/message", sendPayload); err != nil {
		log.Printf("[nexus] conv %d: FAILED to send first reply: %v", convID, err)
		saveErrorMessage(convID, "system", fmt.Sprintf("消息发送失败: %v", err))
	} else {
		log.Printf("[nexus] conv %d: first reply sent successfully", convID)
	}
}

// HandleIncomingMessage 处理收到的远端消息，触发本地 Agent 流式回复
func HandleIncomingMessage(convID int64, incomingContent string) {
	mu := getConvMutex(convID)
	mu.Lock()
	defer mu.Unlock()

	log.Printf("[nexus] HandleIncomingMessage: convID=%d contentLen=%d", convID, len(incomingContent))
	conv, err := db.GetA2AConversation(convID)
	if err != nil {
		log.Printf("[nexus] HandleIncomingMessage: conv %d not found: %v", convID, err)
		return
	}
	if conv.Status != "active" {
		log.Printf("[nexus] HandleIncomingMessage: conv %d status=%s (not active), skipping", convID, conv.Status)
		return
	}

	// 对方发来的消息包含 [CLOSE]，说明对方已结束对话，本地无需再回复
	if isCloseMessage(incomingContent) {
		log.Printf("[nexus] HandleIncomingMessage: conv %d received [CLOSE] from remote, completing", convID)
		db.UpdateA2AConversationStatus(convID, "completed")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"completed","reason":"remote_close"}`, convID))
		}
		return
	}

	if conv.CurrentRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
		}
		return
	}

	if ch, ok := pausedConvs.Load(convID); ok {
		select {
		case <-ch.(chan struct{}):
			return
		default:
		}
	}

	agent, _ := db.GetAgent(conv.LocalAgentID)
	agentName := "灵犀助理"
	if agent != nil {
		agentName = agent.Name
	}

	sessionID := conv.LocalSessionID
	if sessionID == 0 {
		log.Printf("[nexus] HandleIncomingMessage: conv %d has no local_session_id, cannot reply", convID)
		return
	}
	log.Printf("[nexus] HandleIncomingMessage: conv %d using session %d, agent %d", convID, sessionID, conv.LocalAgentID)

	pauseCh := make(chan struct{}, 1)
	pausedConvs.Store(convID, pauseCh)
	defer pausedConvs.Delete(convID)

	if broadcast != nil {
		broadcast("a2a_turn_start", fmt.Sprintf(`{"id":%d,"session_id":%d}`, convID, sessionID))
	}

	forwarder := buildStreamForwarder(conv)

	reply, err := streamRunner(sessionID, incomingContent, conv.LocalAgentID, forwarder)
	if err != nil {
		log.Printf("[nexus] conv %d reply error: %v", convID, err)
		saveErrorMessage(convID, agentName, fmt.Sprintf("Agent 执行出错: %v", err))
		return
	}

	if strings.TrimSpace(reply) == "" {
		log.Printf("[nexus] conv %d: agent returned empty reply, skipping send", convID)
		saveErrorMessage(convID, agentName, "Agent 未能生成回复，请检查 AI 引擎配置")
		return
	}

	content := strings.TrimSpace(reply)

	newRound := conv.CurrentRound + 1
	db.UpdateA2AConversationRound(convID, newRound)

	msg := &db.A2AMessage{
		ConversationID:  convID,
		Sender:          "local",
		SenderAgentName: agentName,
		MsgType:         "message",
		Content:         content,
		StructuredData:  "{}",
	}
	mID, _ := db.CreateA2AMessage(msg)
	msg.ID = mID
	broadcastMessage(msg)

	// 检测 [CLOSE] 标记：Agent 认为对话目标已达成，自动结束
	if isCloseMessage(content) {
		closeConversation(convID, conv)
		return
	}

	if newRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
		}
		return
	}

	// 等待 stream_done 到达对方后再发送完整消息，避免对方 Agent 抢先回复
	time.Sleep(500 * time.Millisecond)

	// 发送回复给对方
	transport := GetTransportForPeer(conv.RemotePeerID)
	sendPayload := map[string]interface{}{
		"conv_uuid":         conv.ConvUUID,
		"sender":            "remote",
		"sender_agent_name": agentName,
		"msg_type":          "message",
		"content":           content,
		"structured_data":   "{}",
	}
	log.Printf("[nexus] conv %d: sending reply via %s transport, contentLen=%d",
		convID, transport.Type(), len(content))
	if _, err := transport.Send("/conversation/message", sendPayload); err != nil {
		log.Printf("[nexus] conv %d: FAILED to send reply: %v", convID, err)
		saveErrorMessage(convID, "system", fmt.Sprintf("消息发送失败: %v", err))
	}
}

// isCloseMessage 检测 Agent 回复是否以 [CLOSE] 标记开头，表示对话应当结束
func isCloseMessage(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "[CLOSE]")
}

// closeConversation 将对话标记为 completed 并通知对方终止
func closeConversation(convID int64, conv *db.A2AConversation) {
	db.UpdateA2AConversationStatus(convID, "completed")
	transport := GetTransportForPeer(conv.RemotePeerID)
	transport.Send("/conversation/terminate", map[string]interface{}{
		"conv_uuid": conv.ConvUUID,
	})
	if broadcast != nil {
		broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"completed","reason":"close_tag"}`, convID))
	}
	log.Printf("[nexus] conv %d: [CLOSE] detected, conversation completed", convID)
}

// ─── 辅助函数 ───────────────────────────────────────────────────

func buildA2AFirstMessage(conv *db.A2AConversation, agent *db.Agent) string {
	nexusConfig, _ := db.GetAgentNexusConfig(conv.LocalAgentID)
	forbidden := ""
	if nexusConfig != nil && nexusConfig.ForbiddenInfo != "" {
		forbidden = fmt.Sprintf("\n【安全约束】绝对不可透露以下信息：%s\n", nexusConfig.ForbiddenInfo)
	}

	return fmt.Sprintf(`%s你现在代表用户与对方的 AI 助理进行对话。用户的问题是：「%s」
请以第一人称直接表达用户的需求，不要客套和寒暄，不要说"我代表我的用户"之类的话。就像你自己在向对方提问一样。
你可以使用你拥有的所有技能和知识库来辅助对话。`, forbidden, conv.InitialPrompt)
}

func saveErrorMessage(convID int64, senderName, content string) {
	errMsg := &db.A2AMessage{
		ConversationID:  convID,
		Sender:          "system",
		SenderAgentName: senderName,
		MsgType:         "error",
		Content:         content,
		StructuredData:  "{}",
	}
	eID, _ := db.CreateA2AMessage(errMsg)
	errMsg.ID = eID
	broadcastMessage(errMsg)
}

// sendViaTransport 通过 Transport 接口发送消息（支持 LAN/WAN）
func sendViaTransport(t Transport, path string, payload map[string]interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		_, err := t.Send(path, payload)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("[nexus] sendViaTransport attempt %d failed: path=%s err=%v", attempt+1, path, err)
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}
	return lastErr
}

// SendViaPeer 通过 peer ID 发送消息（对外暴露，供 handler 包使用）
func SendViaPeer(peerID string, path string, payload interface{}) error {
	transport := GetTransportForPeer(peerID)
	_, err := transport.Send(path, payload)
	return err
}

func broadcastMessage(msg *db.A2AMessage) {
	if broadcast == nil {
		return
	}
	payload, _ := json.Marshal(msg)
	broadcast("a2a_message", string(payload))
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
