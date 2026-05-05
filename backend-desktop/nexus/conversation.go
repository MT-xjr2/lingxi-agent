package nexus

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"lingxi-agent/db"
)

// StreamForwarder 转发流式 token 到远端（text/thinking 事件）
type StreamForwarder func(event, data string)

// A2AStreamRunner 执行一轮流式 A2A 对话，返回完整文本回复
// forwarder 可为 nil，非 nil 时会转发 text/thinking token 到远端
type A2AStreamRunner func(sessionID int64, message string, agentID int64, forwarder StreamForwarder) (reply string, err error)

// A2ASessionCreator 创建 A2A 专用会话
type A2ASessionCreator func(title string, agentID int64) (sessionID int64, err error)

// BroadcastFunc 由 main 包注入
type BroadcastFunc func(event, data string)

var (
	streamRunner   A2AStreamRunner
	sessionCreator A2ASessionCreator
	broadcast      BroadcastFunc

	pausedConvs sync.Map // convID -> chan struct{}
	convMutexes sync.Map // convID -> *sync.Mutex
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

// buildStreamForwarder 构建一个将流式 token 转发到远端的函数
func buildStreamForwarder(conv *db.A2AConversation) StreamForwarder {
	contact, _ := db.GetNexusContactByPeerID(conv.RemotePeerID)
	if contact == nil {
		return nil
	}
	remoteURL := fmt.Sprintf("http://%s/api/nexus/conversation/stream-token",
		net.JoinHostPort(contact.Host, fmt.Sprintf("%d", contact.Port)))
	token := contact.SharedSecret

	outConvID := conv.ID
	if conv.RemoteConvID > 0 {
		outConvID = conv.RemoteConvID
	}

	return func(event, data string) {
		payload := map[string]interface{}{
			"conversation_id": outConvID,
			"event":           event,
			"data":            data,
		}
		go func() {
			httpPost(remoteURL, payload, token)
		}()
	}
}

// RunConversation 启动对话引擎（发起方调用，对方已接受）
func RunConversation(convID int64, sessionID int64, remoteHost string, remotePort int, token string) {
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
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"failed"}`, convID))
		}
		return
	}

	msgType, content, structured := parseAgentReply(reply)

	msg := &db.A2AMessage{
		ConversationID:  convID,
		Sender:          "local",
		SenderAgentName: agentName,
		MsgType:         msgType,
		Content:         content,
		StructuredData:  structured,
	}
	msgID, _ := db.CreateA2AMessage(msg)
	msg.ID = msgID
	newRound := conv.CurrentRound + 1
	db.UpdateA2AConversationRound(convID, newRound)
	broadcastMessage(msg)

	if msgType == "close" {
		finishConversation(convID)
		return
	}

	if newRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 对话已暂停",
				"body":  fmt.Sprintf("对话「%s」已达到最大轮次 %d", conv.Topic, conv.MaxRounds),
			})
			broadcast("desktop_notify", string(payload))
		}
		return
	}

	remoteURL := fmt.Sprintf("http://%s/api/nexus/conversation/message",
		net.JoinHostPort(remoteHost, fmt.Sprintf("%d", remotePort)))
	outConvID := convID
	if conv.RemoteConvID > 0 {
		outConvID = conv.RemoteConvID
	}
	log.Printf("[nexus] conv %d: sending first reply to remote: url=%s outConvID=%d contentLen=%d",
		convID, remoteURL, outConvID, len(content))
	if err := sendToRemote(remoteURL, outConvID, "remote", agentName, msgType, content, structured, token); err != nil {
		log.Printf("[nexus] conv %d: FAILED to send first reply to remote: %v", convID, err)
	} else {
		log.Printf("[nexus] conv %d: first reply sent to remote successfully", convID)
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

	if conv.CurrentRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 对话已暂停",
				"body":  fmt.Sprintf("对话「%s」已达到最大轮次 %d", conv.Topic, conv.MaxRounds),
			})
			broadcast("desktop_notify", string(payload))
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
		return
	}

	msgType, content, structured := parseAgentReply(reply)

	if msgType == "handoff" {
		db.UpdateA2AConversationStatus(convID, "paused")
		msg := &db.A2AMessage{
			ConversationID:  convID,
			Sender:          "local",
			SenderAgentName: agentName,
			MsgType:         "handoff",
			Content:         content,
			StructuredData:  structured,
		}
		hID, _ := db.CreateA2AMessage(msg)
		msg.ID = hID
		broadcastMessage(msg)
		if broadcast != nil {
			broadcast("a2a_handoff", fmt.Sprintf(`{"id":%d,"content":"%s"}`, convID, escapeJSON(content)))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 请求人类介入",
				"body":  content,
			})
			broadcast("desktop_notify", string(payload))
		}
		return
	}

	newRound := conv.CurrentRound + 1
	db.UpdateA2AConversationRound(convID, newRound)

	msg := &db.A2AMessage{
		ConversationID:  convID,
		Sender:          "local",
		SenderAgentName: agentName,
		MsgType:         msgType,
		Content:         content,
		StructuredData:  structured,
	}
	mID, _ := db.CreateA2AMessage(msg)
	msg.ID = mID
	broadcastMessage(msg)

	if msgType == "close" {
		finishConversation(convID)
		return
	}

	if newRound >= conv.MaxRounds {
		db.UpdateA2AConversationStatus(convID, "paused")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused","reason":"max_rounds"}`, convID))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 对话已暂停",
				"body":  fmt.Sprintf("对话「%s」已达到最大轮次 %d", conv.Topic, conv.MaxRounds),
			})
			broadcast("desktop_notify", string(payload))
		}
		return
	}

	contact, _ := db.GetNexusContactByPeerID(conv.RemotePeerID)
	if contact != nil {
		remoteURL := fmt.Sprintf("http://%s/api/nexus/conversation/message",
			net.JoinHostPort(contact.Host, fmt.Sprintf("%d", contact.Port)))
		outConvID := convID
		if conv.RemoteConvID > 0 {
			outConvID = conv.RemoteConvID
		}
		log.Printf("[nexus] conv %d: sending reply to remote: url=%s outConvID=%d contentLen=%d",
			convID, remoteURL, outConvID, len(content))
		if err := sendToRemote(remoteURL, outConvID, "remote", agentName, msgType, content, structured, contact.SharedSecret); err != nil {
			log.Printf("[nexus] conv %d: FAILED to send reply to remote: %v", convID, err)
		}
	} else {
		log.Printf("[nexus] conv %d: cannot send reply - contact not found for peer %s", convID, conv.RemotePeerID)
	}
}

func finishConversation(convID int64) {
	conv, _ := db.GetA2AConversation(convID)
	if conv == nil {
		return
	}

	summary := generateSummary(convID)
	decisions := extractDecisions(convID)
	db.UpdateA2AConversationSummary(convID, summary, decisions)

	if conv.RequireApproval {
		db.UpdateA2AConversationStatus(convID, "pending_approval")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"pending_approval"}`, convID))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 对话待审批",
				"body":  fmt.Sprintf("对话「%s」已完成，等待您的审批", conv.Topic),
			})
			broadcast("desktop_notify", string(payload))
		}
	} else {
		db.UpdateA2AConversationStatus(convID, "completed")
		if broadcast != nil {
			broadcast("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"completed"}`, convID))
			payload, _ := json.Marshal(map[string]string{
				"title": "Agent 对话完成",
				"body":  fmt.Sprintf("对话「%s」已完成", conv.Topic),
			})
			broadcast("desktop_notify", string(payload))
		}
	}
}

// ─── 辅助函数 ───────────────────────────────────────────────────

func buildA2AFirstMessage(conv *db.A2AConversation, agent *db.Agent) string {
	nexusConfig, _ := db.GetAgentNexusConfig(conv.LocalAgentID)
	forbidden := ""
	if nexusConfig != nil && nexusConfig.ForbiddenInfo != "" {
		forbidden = fmt.Sprintf("\n【安全约束】绝对不可透露以下信息：%s\n", nexusConfig.ForbiddenInfo)
	}

	return fmt.Sprintf(`%s这是一场 Agent 间对话的开始。请根据以下信息发出你的第一条消息。

【对话主题】%s
【对话目标】%s
【初始指令】%s
【最大轮次】%d

请开始对话，发出你的第一条消息。`,
		forbidden, conv.Topic, conv.Goal, conv.InitialPrompt, conv.MaxRounds)
}

func parseAgentReply(reply string) (msgType, content, structured string) {
	reply = strings.TrimSpace(reply)

	markers := map[string]string{
		"[PROPOSAL]": "proposal",
		"[DECISION]": "decision",
		"[HANDOFF]":  "handoff",
		"[CLOSE]":    "close",
	}

	for marker, mType := range markers {
		if strings.HasPrefix(reply, marker) {
			content = strings.TrimSpace(reply[len(marker):])
			if idx := strings.Index(content, "{"); idx >= 0 {
				jsonPart := content[idx:]
				if isValidJSON(jsonPart) {
					return mType, content, jsonPart
				}
			}
			return mType, content, "{}"
		}
	}

	for marker, mType := range markers {
		if strings.HasSuffix(reply, marker) {
			content = strings.TrimSpace(reply[:len(reply)-len(marker)])
			return mType, content, "{}"
		}
	}

	return "message", reply, "{}"
}

func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func generateSummary(convID int64) string {
	messages, _ := db.ListA2AMessages(convID)
	if len(messages) == 0 {
		return "对话无内容"
	}

	conv, _ := db.GetA2AConversation(convID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("对话主题：%s\n", conv.Topic))
	sb.WriteString(fmt.Sprintf("对话目标：%s\n", conv.Goal))
	sb.WriteString(fmt.Sprintf("总轮次：%d\n\n", conv.CurrentRound))

	for _, m := range messages {
		if m.MsgType == "decision" || m.MsgType == "close" {
			sb.WriteString(fmt.Sprintf("【%s】%s\n", m.MsgType, m.Content))
		}
	}

	if sb.Len() < 50 {
		last := messages[len(messages)-1]
		sb.WriteString(fmt.Sprintf("\n最终消息：%s", last.Content))
	}

	return sb.String()
}

func extractDecisions(convID int64) string {
	messages, _ := db.ListA2AMessages(convID)
	decisions := make([]map[string]string, 0)
	for _, m := range messages {
		if m.MsgType == "decision" {
			decisions = append(decisions, map[string]string{
				"content":    m.Content,
				"structured": m.StructuredData,
			})
		}
	}
	b, _ := json.Marshal(decisions)
	return string(b)
}

func sendToRemote(url string, convID int64, sender, agentName, msgType, content, structured, token string) error {
	return sendToRemoteWithMeta(url, convID, sender, agentName, msgType, content, structured, token, nil)
}

func sendToRemoteWithMeta(url string, convID int64, sender, agentName, msgType, content, structured, token string, meta map[string]interface{}) error {
	payload := map[string]interface{}{
		"conversation_id":   convID,
		"sender":            sender,
		"sender_agent_name": agentName,
		"msg_type":          msgType,
		"content":           content,
		"structured_data":   structured,
	}
	if meta != nil {
		payload["conv_meta"] = meta
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		_, err := httpPost(url, payload, token)
		if err == nil {
			if attempt > 0 {
				log.Printf("[nexus] sendToRemote succeeded on attempt %d: url=%s convID=%d", attempt+1, url, convID)
			}
			return nil
		}
		lastErr = err
		log.Printf("[nexus] sendToRemote attempt %d failed: url=%s convID=%d err=%v", attempt+1, url, convID, err)
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}
	return lastErr
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
