package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"lingxi-agent/db"
	"lingxi-agent/nexus"

	"github.com/gin-gonic/gin"
)

// CreateA2AConversation POST /api/a2a-conversations — 发起 Agent 对话（直接从发现的 peer 发起，无需建联）
func CreateA2AConversation(c *gin.Context) {
	var body struct {
		LocalAgentID    int64  `json:"local_agent_id"`
		RemotePeerID    string `json:"remote_peer_id"`
		Topic           string `json:"topic"`
		Goal            string `json:"goal"`
		InitialPrompt   string `json:"initial_prompt"`
		MaxRounds       int    `json:"max_rounds"`
		RequireApproval bool   `json:"require_approval"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.MaxRounds <= 0 {
		body.MaxRounds = 10
	}
	if body.RemotePeerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "remote_peer_id 不能为空"})
		return
	}

	agent, _ := db.GetAgent(body.LocalAgentID)
	agentName := "灵犀助理"
	if agent != nil {
		agentName = agent.Name
	}

	conv := &db.A2AConversation{
		LocalAgentID:       body.LocalAgentID,
		RemoteAgentName:    "",
		RemotePeerID:       body.RemotePeerID,
		RemotePeerNickname: "",
		Topic:              body.Topic,
		Goal:               body.Goal,
		InitialPrompt:      body.InitialPrompt,
		MaxRounds:          body.MaxRounds,
		Status:             "pending_remote",
		RequireApproval:    body.RequireApproval,
		InitiatedBy:        "local",
	}
	convID, err := db.CreateA2AConversation(conv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	settings, _ := db.GetNexusSettings()
	myNickname := settings.Nickname
	if myNickname == "" {
		myNickname = "灵犀用户"
	}

	// 发送对话邀请给对方
	invitePayload := map[string]interface{}{
		"conv_id":          convID,
		"peer_id":          nexus.Global.InstanceID(),
		"peer_nickname":    myNickname,
		"agent_name":       agentName,
		"topic":            body.Topic,
		"goal":             body.Goal,
		"max_rounds":       body.MaxRounds,
		"require_approval": body.RequireApproval,
	}

	// 尝试通过 WAN（signaling server）或 LAN 发送邀请
	transport := nexus.GetTransportForPeer(body.RemotePeerID)
	if transport.Type() == "wan" {
		sc := nexus.GetSignalingClient()
		if !sc.IsConnected() {
			db.UpdateA2AConversationStatus(convID, "failed")
			c.JSON(http.StatusBadGateway, gin.H{"error": "未连接信令服务器，无法发送邀请"})
			return
		}
		err = sc.SendConversationInvite(body.RemotePeerID, invitePayload)
	} else {
		_, err = transport.Send("/conversation/invite", invitePayload)
	}

	if err != nil {
		log.Printf("[nexus] send conversation invite failed: %v", err)
		db.UpdateA2AConversationStatus(convID, "failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法发送对话邀请: " + err.Error()})
		return
	}

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"pending_remote"}`, convID))
	c.JSON(http.StatusOK, gin.H{"id": convID})
}

// NexusReceiveConvInvite POST /api/nexus/conversation/invite — 接收对方的对话邀请
func NexusReceiveConvInvite(c *gin.Context) {
	var body struct {
		ConvID          int64  `json:"conv_id"`
		PeerID          string `json:"peer_id"`
		PeerNickname    string `json:"peer_nickname"`
		AgentName       string `json:"agent_name"`
		Topic           string `json:"topic"`
		Goal            string `json:"goal"`
		MaxRounds       int    `json:"max_rounds"`
		RequireApproval bool   `json:"require_approval"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newConv := &db.A2AConversation{
		LocalAgentID:       0,
		RemoteAgentName:    body.AgentName,
		RemotePeerID:       body.PeerID,
		RemotePeerNickname: body.PeerNickname,
		Topic:              body.Topic,
		Goal:               body.Goal,
		MaxRounds:          body.MaxRounds,
		Status:             "pending_incoming",
		RequireApproval:    body.RequireApproval,
		InitiatedBy:        "remote",
		RemoteConvID:       body.ConvID,
	}
	localID, err := db.CreateA2AConversation(newConv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"id":            localID,
		"peer_id":       body.PeerID,
		"peer_nickname": body.PeerNickname,
		"agent_name":    body.AgentName,
		"topic":         body.Topic,
		"goal":          body.Goal,
		"max_rounds":    body.MaxRounds,
	})
	BroadcastWSEvent("a2a_conversation_request", string(payload))

	notif, _ := json.Marshal(map[string]string{
		"title": "收到 Agent 对话邀请",
		"body":  fmt.Sprintf("%s 请求与您的 Agent 对话：%s", body.PeerNickname, body.Topic),
	})
	BroadcastWSEvent("desktop_notify", string(notif))

	c.JSON(http.StatusOK, gin.H{"id": localID})
}

// AcceptRemoteConversation POST /api/a2a-conversations/:id/accept-remote — 本地前端接受对话邀请
func AcceptRemoteConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		LocalAgentID int64 `json:"local_agent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if conv.Status != "pending_incoming" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话不在待接受状态"})
		return
	}

	agent, _ := db.GetAgent(body.LocalAgentID)
	localAgentName := "灵犀助理"
	if agent != nil {
		localAgentName = agent.Name
	}

	// 生成共享 UUID
	convUUID := uuid.New().String()

	// 创建本地会话
	sessionTitle := fmt.Sprintf("[A2A] %s", conv.Topic)
	sessionID, err := CreateA2ASession(sessionTitle, body.LocalAgentID)
	if err != nil {
		log.Printf("[nexus] create session for conv %d failed: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}

	db.DB.Exec("UPDATE a2a_conversations SET local_agent_id=?, status='active', local_session_id=?, conv_uuid=? WHERE id=?",
		body.LocalAgentID, sessionID, convUUID, id)

	// 通知发起方
	acceptPayload := map[string]interface{}{
		"conv_id":           conv.RemoteConvID,
		"conv_uuid":         convUUID,
		"remote_conv_id":    id,
		"remote_agent_name": localAgentName,
	}

	transport := nexus.GetTransportForPeer(conv.RemotePeerID)
	if transport.Type() == "wan" {
		sc := nexus.GetSignalingClient()
		sc.SendConversationAccept(conv.RemotePeerID, acceptPayload)
	} else {
		transport.Send("/conversation/accept", acceptPayload)
	}

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"active","session_id":%d}`, id, sessionID))
	c.JSON(http.StatusOK, gin.H{"ok": true, "session_id": sessionID, "conv_uuid": convUUID})
}

// RejectRemoteConversation POST /api/a2a-conversations/:id/reject-remote — 本地前端拒绝对话邀请
func RejectRemoteConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if conv.Status != "pending_incoming" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话不在待接受状态"})
		return
	}

	db.UpdateA2AConversationStatus(id, "rejected")

	rejectPayload := map[string]interface{}{
		"conv_id": conv.RemoteConvID,
	}
	transport := nexus.GetTransportForPeer(conv.RemotePeerID)
	if transport.Type() == "wan" {
		sc := nexus.GetSignalingClient()
		sc.SendConversationReject(conv.RemotePeerID, rejectPayload)
	} else {
		transport.Send("/conversation/reject", rejectPayload)
	}

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"rejected"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveConvAccept POST /api/nexus/conversation/accept — 接收对方接受对话的通知
func NexusReceiveConvAccept(c *gin.Context) {
	var body struct {
		ConvID          int64  `json:"conv_id"`
		ConvUUID        string `json:"conv_uuid"`
		RemoteConvID    int64  `json:"remote_conv_id"`
		RemoteAgentName string `json:"remote_agent_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[nexus] NexusReceiveConvAccept: convID=%d convUUID=%s remoteConvID=%d remoteAgent=%s",
		body.ConvID, body.ConvUUID, body.RemoteConvID, body.RemoteAgentName)

	conv, err := db.GetA2AConversation(body.ConvID)
	if err != nil {
		log.Printf("[nexus] NexusReceiveConvAccept: conv %d not found: %v", body.ConvID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	// 创建本地会话（发起方）
	sessionTitle := fmt.Sprintf("[A2A] %s", conv.Topic)
	sessionID, err := CreateA2ASession(sessionTitle, conv.LocalAgentID)
	if err != nil {
		log.Printf("[nexus] create session for conv %d (initiator) failed: %v", body.ConvID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}

	db.DB.Exec("UPDATE a2a_conversations SET status='active', remote_conv_id=?, remote_agent_name=?, local_session_id=?, conv_uuid=? WHERE id=?",
		body.RemoteConvID, body.RemoteAgentName, sessionID, body.ConvUUID, body.ConvID)

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"active","session_id":%d}`, body.ConvID, sessionID))

	log.Printf("[nexus] NexusReceiveConvAccept: starting RunConversation convID=%d sessionID=%d", body.ConvID, sessionID)
	go nexus.RunConversation(body.ConvID, sessionID)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveConvReject POST /api/nexus/conversation/reject — 接收对方拒绝对话的通知
func NexusReceiveConvReject(c *gin.Context) {
	var body struct {
		ConvID int64 `json:"conv_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.UpdateA2AConversationStatus(body.ConvID, "rejected")
	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"rejected"}`, body.ConvID))

	notif, _ := json.Marshal(map[string]string{
		"title": "对话邀请被拒绝",
		"body":  fmt.Sprintf("对话 #%d 被对方拒绝", body.ConvID),
	})
	BroadcastWSEvent("desktop_notify", string(notif))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── CRUD handler ──────────────────────────────────────────

// ListA2AConversations GET /api/a2a-conversations
func ListA2AConversations(c *gin.Context) {
	convs, err := db.ListA2AConversations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if convs == nil {
		convs = []db.A2AConversation{}
	}
	c.JSON(http.StatusOK, convs)
}

// GetA2AConversation GET /api/a2a-conversations/:id
func GetA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	messages, _ := db.ListA2AMessages(id)
	if messages == nil {
		messages = []db.A2AMessage{}
	}
	c.JSON(http.StatusOK, gin.H{
		"conversation": conv,
		"messages":     messages,
	})
}

// PauseA2AConversation POST /api/a2a-conversations/:id/pause
func PauseA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if conv.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话不在活跃状态"})
		return
	}
	db.UpdateA2AConversationStatus(id, "paused")
	nexus.PauseConversation(id)

	nexus.SendViaPeer(conv.RemotePeerID, "/conversation/pause", map[string]interface{}{
		"conv_uuid": conv.ConvUUID,
	})

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TakeoverA2AConversation POST /api/a2a-conversations/:id/takeover — 人类接管发消息
func TakeoverA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}

	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	msg := &db.A2AMessage{
		ConversationID:  id,
		Sender:          "human",
		SenderAgentName: "人类接管",
		MsgType:         "message",
		Content:         body.Content,
		StructuredData:  "{}",
	}
	msgID, _ := db.CreateA2AMessage(msg)
	msg.ID = msgID

	nexus.SendViaPeer(conv.RemotePeerID, "/conversation/message", map[string]interface{}{
		"conv_uuid":         conv.ConvUUID,
		"sender":            "remote",
		"sender_agent_name": "人类接管",
		"msg_type":          "message",
		"content":           body.Content,
		"structured_data":   "{}",
	})

	payload, _ := json.Marshal(msg)
	BroadcastWSEvent("a2a_message", string(payload))

	c.JSON(http.StatusOK, gin.H{"id": msgID})
}

// TerminateA2AConversation POST /api/a2a-conversations/:id/terminate
func TerminateA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	conv, err := db.GetA2AConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	db.UpdateA2AConversationStatus(id, "terminated")
	nexus.PauseConversation(id)

	nexus.SendViaPeer(conv.RemotePeerID, "/conversation/terminate", map[string]interface{}{
		"conv_uuid": conv.ConvUUID,
	})

	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"terminated"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ApproveA2AConversation POST /api/a2a-conversations/:id/approve
func ApproveA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Approved bool `json:"approved"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Approved {
		db.UpdateA2AConversationStatus(id, "completed")
		BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"completed"}`, id))
	} else {
		db.UpdateA2AConversationStatus(id, "rejected")
		BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"rejected"}`, id))
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── 对外接收消息的 Handler（供对方灵犀实例调用）─────────────────

// NexusReceiveMessage POST /api/nexus/conversation/message
func NexusReceiveMessage(c *gin.Context) {
	var body struct {
		ConvUUID        string `json:"conv_uuid"`
		ConversationID  int64  `json:"conversation_id"`
		Sender          string `json:"sender"`
		SenderAgentName string `json:"sender_agent_name"`
		MsgType         string `json:"msg_type"`
		Content         string `json:"content"`
		StructuredData  string `json:"structured_data"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Printf("[nexus] NexusReceiveMessage: bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contentPreview := body.Content
	if len(contentPreview) > 200 {
		contentPreview = contentPreview[:200] + "..."
	}
	log.Printf("[nexus] NexusReceiveMessage: convUUID=%s convID=%d sender=%s msgType=%s contentLen=%d",
		body.ConvUUID, body.ConversationID, body.Sender, body.MsgType, len(body.Content))

	// 通过 conv_uuid 或 remote_conv_id 查找本地对话
	var localConvID int64
	if body.ConvUUID != "" {
		conv, err := db.GetA2AConversationByUUID(body.ConvUUID)
		if err == nil && conv != nil {
			localConvID = conv.ID
		}
	}
	if localConvID == 0 && body.ConversationID > 0 {
		existingConv, _ := db.GetA2AConversationByRemoteID(body.ConversationID)
		if existingConv != nil {
			localConvID = existingConv.ID
		} else {
			direct, _ := db.GetA2AConversation(body.ConversationID)
			if direct != nil {
				localConvID = direct.ID
			}
		}
	}

	if localConvID == 0 {
		log.Printf("[nexus] NexusReceiveMessage: WARNING - no local conversation found for convUUID=%s convID=%d", body.ConvUUID, body.ConversationID)
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	msg := &db.A2AMessage{
		ConversationID:  localConvID,
		Sender:          body.Sender,
		SenderAgentName: body.SenderAgentName,
		MsgType:         body.MsgType,
		Content:         body.Content,
		StructuredData:  body.StructuredData,
	}
	if msg.StructuredData == "" {
		msg.StructuredData = "{}"
	}
	msgID, err := db.CreateA2AMessage(msg)
	if err != nil {
		log.Printf("[nexus] NexusReceiveMessage: create message error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	msg.ID = msgID

	payload, _ := json.Marshal(msg)
	BroadcastWSEvent("a2a_message", string(payload))

	if body.Sender == "remote" {
		log.Printf("[nexus] NexusReceiveMessage: triggering HandleIncomingMessage for conv %d", localConvID)
		go nexus.HandleIncomingMessage(localConvID, body.Content)
	}

	c.JSON(http.StatusOK, gin.H{"id": msgID})
}

// NexusReceivePause POST /api/nexus/conversation/pause
func NexusReceivePause(c *gin.Context) {
	var body struct {
		ConvUUID       string `json:"conv_uuid"`
		ConversationID int64  `json:"conversation_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	localID := resolveLocalConvID(body.ConvUUID, body.ConversationID)
	if localID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	db.UpdateA2AConversationStatus(localID, "paused")
	nexus.PauseConversation(localID)
	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"paused"}`, localID))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveTerminate POST /api/nexus/conversation/terminate
func NexusReceiveTerminate(c *gin.Context) {
	var body struct {
		ConvUUID       string `json:"conv_uuid"`
		ConversationID int64  `json:"conversation_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	localID := resolveLocalConvID(body.ConvUUID, body.ConversationID)
	if localID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	db.UpdateA2AConversationStatus(localID, "terminated")
	nexus.PauseConversation(localID)
	BroadcastWSEvent("a2a_status_change", fmt.Sprintf(`{"id":%d,"status":"terminated"}`, localID))

	payload, _ := json.Marshal(map[string]string{
		"title": "Agent 对话已终止",
		"body":  fmt.Sprintf("对话 #%d 被对方终止", localID),
	})
	BroadcastWSEvent("desktop_notify", string(payload))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveStreamToken POST /api/nexus/conversation/stream-token
func NexusReceiveStreamToken(c *gin.Context) {
	var body struct {
		ConvUUID       string `json:"conv_uuid"`
		ConversationID int64  `json:"conversation_id"`
		Event          string `json:"event"`
		Data           string `json:"data"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	localConvID := resolveLocalConvID(body.ConvUUID, body.ConversationID)
	if localConvID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	p, _ := json.Marshal(map[string]interface{}{
		"conversation_id": localConvID,
		"event":           body.Event,
		"data":            body.Data,
	})
	BroadcastWSEvent("a2a_remote_stream", string(p))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteA2AConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := db.DeleteA2AConversation(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// resolveLocalConvID 将 conv_uuid 或 conversation_id 映射为本地的 ID
func resolveLocalConvID(convUUID string, convID int64) int64 {
	if convUUID != "" {
		conv, err := db.GetA2AConversationByUUID(convUUID)
		if err == nil && conv != nil {
			return conv.ID
		}
	}
	if convID > 0 {
		existing, _ := db.GetA2AConversationByRemoteID(convID)
		if existing != nil {
			return existing.ID
		}
		direct, _ := db.GetA2AConversation(convID)
		if direct != nil {
			return direct.ID
		}
	}
	return 0
}
