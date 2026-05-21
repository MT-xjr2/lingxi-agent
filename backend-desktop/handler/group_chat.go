package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lingxi-agent/db"
	"lingxi-agent/grouploop"
	"lingxi-agent/nexus"
)

// ─── HTTP API ────────────────────────────────────────────────────

// CreateGroupChat POST /api/group-chats — 本端创建群聊并向所有成员广播邀请
func CreateGroupChat(c *gin.Context) {
	var body struct {
		Topic string `json:"topic"`
		Goal  string `json:"goal"`
		// 本端成员（多个本地 Agent）
		LocalAgentIDs []int64 `json:"local_agent_ids"`
		// 远端成员：每个 (peer_id, agent_name)
		RemoteMembers []struct {
			PeerID       string `json:"peer_id"`
			PeerNickname string `json:"peer_nickname"`
			AgentName    string `json:"agent_name"`
		} `json:"remote_members"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(body.LocalAgentIDs) == 0 && len(body.RemoteMembers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少添加一个成员"})
		return
	}

	roomUUID := uuid.New().String()
	myInstanceID := nexus.Global.InstanceID()
	settings, _ := db.GetNexusSettings()
	myNickname := settings.Nickname
	if myNickname == "" {
		myNickname = "灵犀用户"
	}

	// 选第一个本地 Agent 作为主持人
	var moderatorAgentID int64
	if len(body.LocalAgentIDs) > 0 {
		moderatorAgentID = body.LocalAgentIDs[0]
	}

	room := &db.GroupChat{
		RoomUUID:         roomUUID,
		Topic:            body.Topic,
		Goal:             body.Goal,
		MaxRounds:        0,
		Status:           "active",
		ModeratorAgentID: moderatorAgentID,
		ModeratorPeerID:  myInstanceID,
		ScheduleMode:     "free",
		CreatedByLocal:   true,
		HostPeerID:       myInstanceID,
	}
	roomID, err := db.CreateGroupChat(room)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	room.ID = roomID

	// 添加本端成员
	for _, aid := range body.LocalAgentIDs {
		agent, _ := db.GetAgent(aid)
		if agent == nil {
			continue
		}
		db.AddGroupMember(&db.GroupMember{
			RoomID:       roomID,
			PeerID:       myInstanceID,
			PeerNickname: myNickname,
			AgentID:      aid,
			AgentName:    agent.Name,
			IsLocal:      true,
			Status:       "joined",
		})
	}

	// 添加远端成员（先 invited，邀请 ack 后改 joined）
	for _, m := range body.RemoteMembers {
		db.AddGroupMember(&db.GroupMember{
			RoomID:       roomID,
			PeerID:       m.PeerID,
			PeerNickname: m.PeerNickname,
			AgentName:    m.AgentName,
			IsLocal:      false,
			Status:       "invited",
		})
	}

	// 向远端成员发送邀请
	memberList := []map[string]interface{}{}
	allMembers, _ := db.ListGroupMembers(roomID)
	for _, mem := range allMembers {
		memberList = append(memberList, map[string]interface{}{
			"peer_id":       mem.PeerID,
			"peer_nickname": mem.PeerNickname,
			"agent_name":    mem.AgentName,
			"is_local":      mem.IsLocal,
		})
	}

	uniquePeers := map[string]bool{}
	for _, m := range body.RemoteMembers {
		if uniquePeers[m.PeerID] {
			continue
		}
		uniquePeers[m.PeerID] = true
		invitePayload := map[string]interface{}{
			"room_uuid":     roomUUID,
			"host_peer_id":  myInstanceID,
			"host_nickname": myNickname,
			"topic":         body.Topic,
			"goal":          body.Goal,
			"members":       memberList,
		}
		if err := nexus.SendGroupInvite(m.PeerID, invitePayload); err != nil {
			// 仅记录失败，不阻塞群创建
			saveSystemGroupMessage(roomID, fmt.Sprintf("无法向 %s 发送邀请: %v", m.PeerNickname, err))
		}
	}

	BroadcastWSEvent("group_status_change", fmt.Sprintf(`{"room_id":%d,"status":"active","reason":"created"}`, roomID))
	grouploop.BootRoom(roomID, true)
	c.JSON(http.StatusOK, gin.H{"id": roomID, "room_uuid": roomUUID})
}

// ListGroupChats GET /api/group-chats
func ListGroupChats(c *gin.Context) {
	rooms, err := db.ListGroupChats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rooms)
}

// GetGroupChatDetail GET /api/group-chats/:id
func GetGroupChatDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	room, err := db.GetGroupChat(id)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}
	members, _ := db.ListGroupMembers(id)
	messages, _ := db.ListGroupMessages(id)
	c.JSON(http.StatusOK, gin.H{
		"room":     room,
		"members":  nexus.EnrichGroupMembers(id, members),
		"messages": messages,
		"human_nickname": nexus.HumanGroupNickname(),
	})
}

// PostGroupMessage POST /api/group-chats/:id/post — 用户在群里发言
// 支持 reply_to_id / images / client_msg_id / mentioned_agents
func PostGroupMessage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Content         string   `json:"content"`
		ReplyToID       int64    `json:"reply_to_id"`
		Images          []string `json:"images"`
		ClientMsgID     string   `json:"client_msg_id"`
		MentionedAgents []string `json:"mentioned_agents"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChat(id)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}

	myInstanceID := nexus.Global.InstanceID()
	settings, _ := db.GetNexusSettings()
	myNickname := settings.Nickname
	if myNickname == "" {
		myNickname = "我"
	}

	imagesJSON := "[]"
	if len(body.Images) > 0 {
		b, _ := json.Marshal(body.Images)
		imagesJSON = string(b)
	}
	mentionsJSON := "[]"
	if len(body.MentionedAgents) > 0 {
		// 转成 [{agent_name:"xxx"}] 兼容旧字段格式
		arr := make([]map[string]string, 0, len(body.MentionedAgents))
		for _, n := range body.MentionedAgents {
			arr = append(arr, map[string]string{"agent_name": n})
		}
		b, _ := json.Marshal(arr)
		mentionsJSON = string(b)
	}

	msg := &db.GroupMessage{
		RoomID:          id,
		SenderPeerID:    myInstanceID,
		SenderAgentID:   0,
		SenderAgentName: myNickname,
		MsgType:         "user_post",
		Content:         body.Content,
		Round:           room.CurrentRound,
		ReplyToID:       body.ReplyToID,
		Images:          imagesJSON,
		ClientMsgID:     body.ClientMsgID,
		MentionedAgents: mentionsJSON,
	}
	// HandleGroupMessage 内部会写入 + 广播 + 推进
	nexus.HandleGroupMessage(id, msg)

	// 广播给远端成员
	nexus.BroadcastGroupMessage(id, "/group/message", map[string]interface{}{
		"room_uuid":         room.RoomUUID,
		"sender_peer_id":    myInstanceID,
		"sender_agent_id":   0,
		"sender_agent_name": myNickname,
		"msg_type":          "user_post",
		"content":           body.Content,
		"mentioned_agents":  mentionsJSON,
		"round":             room.CurrentRound,
		"reply_to_id":       body.ReplyToID,
		"images":            imagesJSON,
		"client_msg_id":     body.ClientMsgID,
	})

	c.JSON(http.StatusOK, gin.H{"ok": true, "message_id": msg.ID})
}

// ListGroupMessagesPagedHandler GET /api/group-chats/:id/messages?before=<id>&limit=<n>
// 用于下拉加载更早消息
func ListGroupMessagesPagedHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := db.ListGroupMessagesPaged(id, before, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

// RecallGroupMessageHandler POST /api/group-chats/:id/messages/:msgId/recall
// 用户撤回自己 2 分钟内的消息（Agent 撤回由后端内部触发）
func RecallGroupMessageHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	msgID, _ := strconv.ParseInt(c.Param("msgId"), 10, 64)
	room, _ := db.GetGroupChat(id)
	if room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}
	myInstanceID := nexus.Global.InstanceID()
	settings, _ := db.GetNexusSettings()
	myNickname := settings.Nickname
	if myNickname == "" {
		myNickname = "我"
	}

	err := db.RecallGroupMessage(msgID, myInstanceID, myNickname, 120)
	if err == db.ErrTooLate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "超过 2 分钟，无法撤回"})
		return
	}
	if err == db.ErrNotOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能撤回自己的消息"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nexus.BroadcastGroupRecall(id, msgID)
	// 跨实例转发
	nexus.BroadcastGroupMessage(id, "/group/recall", map[string]interface{}{
		"room_uuid":      room.RoomUUID,
		"message_id":     msgID,
		"by_peer_id":     myInstanceID,
		"by_agent_name":  myNickname,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// LeaveGroupChat POST /api/group-chats/:id/leave — 本端退群
func LeaveGroupChat(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	room, err := db.GetGroupChat(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}
	myInstanceID := nexus.Global.InstanceID()
	// 移除本端所有 agent
	members, _ := db.ListGroupMembers(id)
	for _, m := range members {
		if m.IsLocal {
			db.UpdateGroupMemberStatus(id, m.PeerID, m.AgentName, "left")
		}
	}
	// 通知其他成员
	nexus.BroadcastGroupMessage(id, "/group/leave", map[string]interface{}{
		"room_uuid": room.RoomUUID,
		"peer_id":   myInstanceID,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PauseGroupChat POST /api/group-chats/:id/pause — 停止 Agent 自动发言（可恢复）
func PauseGroupChat(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	grouploop.StopRoom(id)
	db.UpdateGroupChatStatus(id, "paused")
	BroadcastWSEvent("group_status_change", fmt.Sprintf(`{"room_id":%d,"status":"paused","reason":"manual_pause"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "paused"})
}

// ResumeGroupChat POST /api/group-chats/:id/resume — 恢复 Agent 自动发言
func ResumeGroupChat(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	room, err := db.GetGroupChat(id)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}
	db.UpdateGroupChatStatus(id, "active")
	grouploop.BootRoom(id, false)
	BroadcastWSEvent("group_status_change", fmt.Sprintf(`{"room_id":%d,"status":"active","reason":"manual_resume"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "active"})
}

// TerminateGroupChat POST /api/group-chats/:id/terminate
func TerminateGroupChat(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	room, _ := db.GetGroupChat(id)
	grouploop.StopRoom(id)
	db.UpdateGroupChatStatus(id, "completed")
	if room != nil {
		nexus.BroadcastGroupMessage(id, "/group/leave", map[string]interface{}{
			"room_uuid": room.RoomUUID,
			"peer_id":   nexus.Global.InstanceID(),
			"terminate": true,
		})
	}
	BroadcastWSEvent("group_status_change", fmt.Sprintf(`{"room_id":%d,"status":"completed","reason":"manual_terminate"}`, id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteGroupChatHandler DELETE /api/group-chats/:id
func DeleteGroupChatHandler(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	grouploop.StopRoom(id)
	if err := db.DeleteGroupChat(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AcceptGroupInvite POST /api/group-chats/:id/accept — 接受邀请并把本端 Agent 加入
func AcceptGroupInvite(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		LocalAgentIDs []int64 `json:"local_agent_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChat(id)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}

	myInstanceID := nexus.Global.InstanceID()
	settings, _ := db.GetNexusSettings()
	myNickname := settings.Nickname
	if myNickname == "" {
		myNickname = "灵犀用户"
	}

	// 添加本端 Agent
	for _, aid := range body.LocalAgentIDs {
		agent, _ := db.GetAgent(aid)
		if agent == nil {
			continue
		}
		db.AddGroupMember(&db.GroupMember{
			RoomID:       id,
			PeerID:       myInstanceID,
			PeerNickname: myNickname,
			AgentID:      aid,
			AgentName:    agent.Name,
			IsLocal:      true,
			Status:       "joined",
		})

		// 通知群主：我加入了
		nexus.SendGroupJoinAck(room.HostPeerID, map[string]interface{}{
			"room_uuid":     room.RoomUUID,
			"peer_id":       myInstanceID,
			"peer_nickname": myNickname,
			"agent_id":      aid,
			"agent_name":    agent.Name,
		})
	}

	// 让群主转发本端加入消息给其他成员（host 收到 join_ack 后会广播）
	BroadcastWSEvent("group_member_joined", fmt.Sprintf(`{"room_id":%d,"peer_id":"%s"}`, id, myInstanceID))
	if room.Status != "active" {
		db.UpdateGroupChatStatus(id, "active")
	}
	grouploop.BootRoom(id, len(body.LocalAgentIDs) > 0)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RejectGroupInvite POST /api/group-chats/:id/reject
func RejectGroupInvite(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	room, _ := db.GetGroupChat(id)
	if room != nil {
		myInstanceID := nexus.Global.InstanceID()
		nexus.SendGroupJoinAck(room.HostPeerID, map[string]interface{}{
			"room_uuid": room.RoomUUID,
			"peer_id":   myInstanceID,
			"reject":    true,
		})
	}
	db.UpdateGroupChatStatus(id, "rejected")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Nexus 接收侧（其他 peer 通过 transport 发到本端的接口）─────────

// NexusReceiveGroupInvite POST /api/nexus/group/invite
func NexusReceiveGroupInvite(c *gin.Context) {
	var body struct {
		RoomUUID     string                   `json:"room_uuid"`
		HostPeerID   string                   `json:"host_peer_id"`
		HostNickname string                   `json:"host_nickname"`
		Topic        string                   `json:"topic"`
		Goal         string                   `json:"goal"`
		MaxRounds    int                      `json:"max_rounds"`
		ScheduleMode string                   `json:"schedule_mode"`
		Members      []map[string]interface{} `json:"members"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否已经收到过这个邀请
	existing, _ := db.GetGroupChatByUUID(body.RoomUUID)
	if existing != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	// 创建本地副本（remote 创建，等待用户接受）
	room := &db.GroupChat{
		RoomUUID:        body.RoomUUID,
		Topic:           body.Topic,
		Goal:            body.Goal,
		MaxRounds:       body.MaxRounds,
		Status:          "pending",
		ScheduleMode:    body.ScheduleMode,
		ModeratorPeerID: body.HostPeerID,
		HostPeerID:      body.HostPeerID,
		CreatedByLocal:  false,
	}
	roomID, err := db.CreateGroupChat(room)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 落库其他成员（含群主）
	for _, m := range body.Members {
		peerID, _ := m["peer_id"].(string)
		peerNick, _ := m["peer_nickname"].(string)
		agentName, _ := m["agent_name"].(string)
		isLocalRaw, _ := m["is_local"].(bool)
		// 注意：host 视角的 is_local 是相对 host 的；从我们这里看，他们都是远端
		_ = isLocalRaw
		status := "joined"
		if peerID == "" || agentName == "" {
			continue
		}
		db.AddGroupMember(&db.GroupMember{
			RoomID:       roomID,
			PeerID:       peerID,
			PeerNickname: peerNick,
			AgentName:    agentName,
			IsLocal:      false,
			Status:       status,
		})
	}

	// WS 通知前端：弹出邀请卡片
	BroadcastWSEvent("group_invite_received", fmt.Sprintf(
		`{"room_id":%d,"room_uuid":"%s","host_nickname":"%s","topic":"%s","goal":"%s"}`,
		roomID, body.RoomUUID, body.HostNickname, body.Topic, body.Goal))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveGroupJoinAck POST /api/nexus/group/join_ack
func NexusReceiveGroupJoinAck(c *gin.Context) {
	var body struct {
		RoomUUID     string `json:"room_uuid"`
		PeerID       string `json:"peer_id"`
		PeerNickname string `json:"peer_nickname"`
		AgentID      int64  `json:"agent_id"`
		AgentName    string `json:"agent_name"`
		Reject       bool   `json:"reject"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChatByUUID(body.RoomUUID)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}

	if body.Reject {
		db.UpdateGroupMemberStatus(room.ID, body.PeerID, body.AgentName, "rejected")
	} else {
		// 远端 Agent 加入：记录或更新
		members, _ := db.ListGroupMembers(room.ID)
		exists := false
		for _, m := range members {
			if m.PeerID == body.PeerID && m.AgentName == body.AgentName {
				db.UpdateGroupMemberStatus(room.ID, body.PeerID, body.AgentName, "joined")
				exists = true
				break
			}
		}
		if !exists {
			db.AddGroupMember(&db.GroupMember{
				RoomID:       room.ID,
				PeerID:       body.PeerID,
				PeerNickname: body.PeerNickname,
				AgentName:    body.AgentName,
				IsLocal:      false,
				Status:       "joined",
			})
		}
	}

	BroadcastWSEvent("group_member_joined", fmt.Sprintf(`{"room_id":%d,"peer_id":"%s","agent_name":"%s","reject":%v}`,
		room.ID, body.PeerID, body.AgentName, body.Reject))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveGroupMessage POST /api/nexus/group/message
func NexusReceiveGroupMessage(c *gin.Context) {
	var body struct {
		RoomUUID        string `json:"room_uuid"`
		SenderPeerID    string `json:"sender_peer_id"`
		SenderAgentID   int64  `json:"sender_agent_id"`
		SenderAgentName string `json:"sender_agent_name"`
		MsgType         string `json:"msg_type"`
		Content         string `json:"content"`
		MentionedAgents string `json:"mentioned_agents"`
		Round           int    `json:"round"`
		ReplyToID       int64  `json:"reply_to_id"`
		Images          string `json:"images"`
		ClientMsgID     string `json:"client_msg_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChatByUUID(body.RoomUUID)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "群聊不存在"})
		return
	}
	if body.MsgType == "" {
		body.MsgType = "message"
	}
	if body.MentionedAgents == "" {
		body.MentionedAgents = "[]"
	}
	if body.Images == "" {
		body.Images = "[]"
	}
	msg := &db.GroupMessage{
		RoomID:          room.ID,
		SenderPeerID:    body.SenderPeerID,
		SenderAgentID:   body.SenderAgentID,
		SenderAgentName: body.SenderAgentName,
		MsgType:         body.MsgType,
		Content:         body.Content,
		MentionedAgents: body.MentionedAgents,
		Round:           body.Round,
		ReplyToID:       body.ReplyToID,
		Images:          body.Images,
		ClientMsgID:     body.ClientMsgID,
	}
	nexus.HandleGroupMessage(room.ID, msg)

	// 如果本端是群主，把消息广播给其他成员（除发送者外）
	if room.CreatedByLocal {
		members, _ := db.ListGroupMembers(room.ID)
		for _, m := range members {
			if m.IsLocal || m.PeerID == body.SenderPeerID {
				continue
			}
			go func(peerID string) {
				nexus.SendGroupMessage(peerID, map[string]interface{}{
					"room_uuid":         room.RoomUUID,
					"sender_peer_id":    body.SenderPeerID,
					"sender_agent_id":   body.SenderAgentID,
					"sender_agent_name": body.SenderAgentName,
					"msg_type":          body.MsgType,
					"content":           body.Content,
					"mentioned_agents":  body.MentionedAgents,
					"round":             body.Round,
					"reply_to_id":       body.ReplyToID,
					"images":            body.Images,
					"client_msg_id":     body.ClientMsgID,
				})
			}(m.PeerID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveGroupRecall POST /api/nexus/group/recall
// 其他实例发来的"消息已撤回"通知
func NexusReceiveGroupRecall(c *gin.Context) {
	var body struct {
		RoomUUID     string `json:"room_uuid"`
		MessageID    int64  `json:"message_id"`
		ByPeerID     string `json:"by_peer_id"`
		ByAgentName  string `json:"by_agent_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChatByUUID(body.RoomUUID)
	if err != nil || room == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	// 跨实例撤回不校验所有者（远端已经校验过）
	if err := db.RecallGroupMessage(body.MessageID, "", "", 0); err != nil {
		// 容忍找不到消息的情况（远端消息可能 ID 不一致；后续 message_id 同步策略可优化）
	}
	nexus.BroadcastGroupRecall(room.ID, body.MessageID)

	// host 转发给其他 peer
	if room.CreatedByLocal {
		members, _ := db.ListGroupMembers(room.ID)
		for _, m := range members {
			if m.IsLocal || m.PeerID == body.ByPeerID {
				continue
			}
			go func(peerID string) {
				nexus.SendGroupRecall(peerID, map[string]interface{}{
					"room_uuid":     room.RoomUUID,
					"message_id":    body.MessageID,
					"by_peer_id":    body.ByPeerID,
					"by_agent_name": body.ByAgentName,
				})
			}(m.PeerID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveGroupLeave POST /api/nexus/group/leave
func NexusReceiveGroupLeave(c *gin.Context) {
	var body struct {
		RoomUUID  string `json:"room_uuid"`
		PeerID    string `json:"peer_id"`
		Terminate bool   `json:"terminate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := db.GetGroupChatByUUID(body.RoomUUID)
	if err != nil || room == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	if body.Terminate {
		grouploop.StopRoom(room.ID)
		db.UpdateGroupChatStatus(room.ID, "completed")
		BroadcastWSEvent("group_status_change", fmt.Sprintf(`{"room_id":%d,"status":"completed","reason":"host_terminate"}`, room.ID))
	} else {
		// 标记该 peer 所有 agent 为 left
		members, _ := db.ListGroupMembers(room.ID)
		for _, m := range members {
			if m.PeerID == body.PeerID {
				db.UpdateGroupMemberStatus(room.ID, m.PeerID, m.AgentName, "left")
			}
		}
		BroadcastWSEvent("group_member_left", fmt.Sprintf(`{"room_id":%d,"peer_id":"%s"}`, room.ID, body.PeerID))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusReceiveGroupStreamToken POST /api/nexus/group/stream_token
func NexusReceiveGroupStreamToken(c *gin.Context) {
	var body struct {
		RoomUUID        string `json:"room_uuid"`
		RoomID          int64  `json:"room_id"`
		SenderPeerID    string `json:"sender_peer_id"`
		SenderAgentName string `json:"sender_agent_name"`
		Event           string `json:"event"`
		Data            string `json:"data"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 转发给前端实时显示
	payload, _ := json.Marshal(body)
	BroadcastWSEvent("group_stream_token_remote", string(payload))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── 辅助 ─────────────────────────────────────────────────────────

func saveSystemGroupMessage(roomID int64, content string) {
	msg := &db.GroupMessage{
		RoomID:          roomID,
		SenderPeerID:    "system",
		SenderAgentName: "系统",
		MsgType:         "system",
		Content:         content,
	}
	db.CreateGroupMessage(msg)
	BroadcastWSEvent("group_message", string(mustJSON(msg)))
}

// CreateGroupSession 群聊会话创建器（注入到 nexus.InitGroup）
func CreateGroupSession(title string, agentID int64) (int64, error) {
	res, err := db.DB.Exec(`INSERT INTO sessions (title, agent_id, is_a2a) VALUES (?, ?, 1)`, title, agentID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ModeratorLLM 主持人决策（同步调 active LLM，复用 callActiveLLM）
func ModeratorLLM(prompt string) string {
	return callActiveLLM(prompt)
}
