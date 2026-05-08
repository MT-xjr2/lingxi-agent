package nexus

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lingxi-agent/db"
)

// SignalMessage 与信令服务器的通信协议
type SignalMessage struct {
	Type string          `json:"type"`
	From string          `json:"from,omitempty"`
	To   string          `json:"to,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// RemotePeer 远程（广域网）发现的节点
type RemotePeer struct {
	InstanceID string        `json:"instance_id"`
	Nickname   string        `json:"nickname"`
	UserID     string        `json:"user_id"`
	AvatarURL  string        `json:"avatar_url"`
	Agents     []PublicAgent `json:"agents"`
	Platform   string        `json:"platform"`
	DeviceName string        `json:"device_name"`
	LocalIP    string        `json:"local_ip"`
	LocalPort  int           `json:"local_port"`
}

// SignalingClient 连接信令服务器的客户端
type SignalingClient struct {
	mu           sync.Mutex
	conn         *websocket.Conn
	serverURL    string
	running      bool
	stopCh       chan struct{}
	remotePeers  []RemotePeer
	remotePeerMu sync.RWMutex
	handlers     map[string]func(SignalMessage)
	handlerMu    sync.RWMutex
}

var sigClient = &SignalingClient{
	handlers: make(map[string]func(SignalMessage)),
}

// GetSignalingClient 返回全局信令客户端
func GetSignalingClient() *SignalingClient {
	return sigClient
}

// Start 连接信令服务器
func (sc *SignalingClient) Start(serverURL string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.running {
		return
	}
	sc.serverURL = serverURL
	sc.running = true
	sc.stopCh = make(chan struct{})
	go sc.connectLoop()
	log.Printf("[signaling] client starting, server=%s", serverURL)
}

// StartWithSecret 兼容旧接口，忽略 secret 参数
func (sc *SignalingClient) StartWithSecret(serverURL, secret string) {
	sc.Start(serverURL)
}

// Stop 断开信令服务器
func (sc *SignalingClient) Stop() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !sc.running {
		return
	}
	sc.running = false
	close(sc.stopCh)
	if sc.conn != nil {
		sc.conn.Close()
	}
	log.Printf("[signaling] client stopped")
}

// IsConnected 是否已连接信令服务器
func (sc *SignalingClient) IsConnected() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn != nil
}

// ListRemotePeers 获取远程节点列表
func (sc *SignalingClient) ListRemotePeers() []RemotePeer {
	sc.remotePeerMu.RLock()
	defer sc.remotePeerMu.RUnlock()
	out := make([]RemotePeer, len(sc.remotePeers))
	copy(out, sc.remotePeers)
	return out
}

// OnMessage 注册消息处理回调
func (sc *SignalingClient) OnMessage(msgType string, handler func(SignalMessage)) {
	sc.handlerMu.Lock()
	sc.handlers[msgType] = handler
	sc.handlerMu.Unlock()
}

// RefreshPeers 请求信令服务器更新在线节点列表
func (sc *SignalingClient) RefreshPeers() {
	sc.sendMsg(SignalMessage{Type: "list_peers"})
}

// SendRelay 通过信令服务器中继消息到目标 peer
func (sc *SignalingClient) SendRelay(targetPeerID string, path string, payload interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"path":    path,
		"payload": payload,
	})
	if err != nil {
		return err
	}
	return sc.sendMsg(SignalMessage{
		Type: "relay",
		To:   targetPeerID,
		Data: data,
	})
}

// SendConversationInvite 发送对话邀请
func (sc *SignalingClient) SendConversationInvite(targetPeerID string, payload interface{}) error {
	data, _ := json.Marshal(payload)
	return sc.sendMsg(SignalMessage{
		Type: "conversation_invite",
		To:   targetPeerID,
		Data: data,
	})
}

// SendConversationAccept 发送对话接受
func (sc *SignalingClient) SendConversationAccept(targetPeerID string, payload interface{}) error {
	data, _ := json.Marshal(payload)
	return sc.sendMsg(SignalMessage{
		Type: "conversation_accept",
		To:   targetPeerID,
		Data: data,
	})
}

// SendConversationReject 发送对话拒绝
func (sc *SignalingClient) SendConversationReject(targetPeerID string, payload interface{}) error {
	data, _ := json.Marshal(payload)
	return sc.sendMsg(SignalMessage{
		Type: "conversation_reject",
		To:   targetPeerID,
		Data: data,
	})
}

func (sc *SignalingClient) connectLoop() {
	attempt := 0
	for {
		select {
		case <-sc.stopCh:
			return
		default:
		}

		sc.broadcastConnectionStatus(false)
		err := sc.connect()
		if err != nil {
			attempt++
			delay := sc.backoffDelay(attempt)
			log.Printf("[signaling] connect error: %v, retrying in %v (attempt %d)", err, delay, attempt)
			select {
			case <-sc.stopCh:
				return
			case <-time.After(delay):
			}
		} else {
			attempt = 0
		}
	}
}

func (sc *SignalingClient) backoffDelay(attempt int) time.Duration {
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	if attempt-1 < len(delays) {
		return delays[attempt-1]
	}
	return 30 * time.Second
}

func (sc *SignalingClient) broadcastConnectionStatus(connected bool) {
	if broadcast != nil {
		status := "disconnected"
		if connected {
			status = "connected"
		}
		broadcast("wan_connection_status", fmt.Sprintf(`{"connected":%v,"status":"%s"}`, connected, status))
	}
}

func (sc *SignalingClient) connect() error {
	u, err := url.Parse(sc.serverURL)
	if err != nil {
		return err
	}
	useWSS := u.Scheme == "https" || u.Scheme == "wss"
	if useWSS {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/ws"

	dialer := *websocket.DefaultDialer
	if useWSS {
		dialer.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		dialer.HandshakeTimeout = 15 * time.Second
	}
	header := http.Header{}
	header.Set("User-Agent", "lingxi-signaling/2.0")

	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	sc.mu.Lock()
	sc.conn = conn
	sc.mu.Unlock()

	sc.broadcastConnectionStatus(true)
	sc.register()

	done := make(chan struct{})
	defer close(done)
	go func() {
		pingTicker := time.NewTicker(25 * time.Second)
		heartbeatTicker := time.NewTicker(3 * time.Minute)
		defer pingTicker.Stop()
		defer heartbeatTicker.Stop()
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				sc.mu.Lock()
				if sc.conn != nil {
					if err := sc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Printf("[signaling] ping write failed: %v, closing connection", err)
						sc.conn.Close()
						sc.conn = nil
						sc.mu.Unlock()
						return
					}
				}
				sc.mu.Unlock()
			case <-heartbeatTicker.C:
				if err := sc.sendMsg(SignalMessage{Type: "heartbeat"}); err != nil {
					log.Printf("[signaling] heartbeat send failed: %v", err)
					return
				}
			}
		}
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			sc.mu.Lock()
			sc.conn = nil
			sc.mu.Unlock()
			sc.broadcastConnectionStatus(false)
			return fmt.Errorf("read: %w", err)
		}
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))

		var msg SignalMessage
		if json.Unmarshal(msgBytes, &msg) != nil {
			continue
		}
		sc.handleMessage(msg)
	}
}

func (sc *SignalingClient) register() {
	settings, _ := db.GetNexusSettings()
	nickname := settings.Nickname
	if nickname == "" {
		nickname = "灵犀用户"
	}

	configs, _ := db.ListPublicAgentConfigs()
	agents := make([]PublicAgent, 0)
	for _, cfg := range configs {
		var tags []string
		json.Unmarshal([]byte(cfg.CapabilityTags), &tags)
		if tags == nil {
			tags = []string{}
		}
		agents = append(agents, PublicAgent{
			ID:             cfg.AgentID,
			Name:           cfg.PublicName,
			CapabilityTags: tags,
			AuthLevel:      cfg.AuthLevel,
		})
	}

	var userID, avatarURL string
	user, err := db.GetCurrentUser()
	if err == nil && user != nil {
		userID = fmt.Sprintf("%s_%s", user.Provider, user.ProviderID)
		avatarURL = user.AvatarURL
		if nickname == "灵犀用户" && user.Nickname != "" {
			nickname = user.Nickname
		}
	}

	hostname, _ := os.Hostname()
	localIP := getLocalIPForSignaling()

	regData, _ := json.Marshal(map[string]interface{}{
		"instance_id": Global.InstanceID(),
		"nickname":    nickname,
		"user_id":     userID,
		"avatar_url":  avatarURL,
		"agents":      agents,
		"platform":    runtime.GOOS,
		"device_name": hostname,
		"local_ip":    localIP,
		"local_port":  settings.ListenPort,
	})

	sc.sendMsg(SignalMessage{
		Type: "register",
		Data: regData,
	})

	time.AfterFunc(500*time.Millisecond, func() {
		sc.RefreshPeers()
	})
}

func getLocalIPForSignaling() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1"
}

func (sc *SignalingClient) handleMessage(msg SignalMessage) {
	switch msg.Type {
	case "peers_list":
		var peers []RemotePeer
		json.Unmarshal(msg.Data, &peers)
		myID := Global.InstanceID()
		filtered := make([]RemotePeer, 0, len(peers))
		for _, p := range peers {
			if p.InstanceID != myID {
				filtered = append(filtered, p)
			}
		}
		sc.remotePeerMu.Lock()
		sc.remotePeers = filtered
		sc.remotePeerMu.Unlock()
		if broadcast != nil {
			data, _ := json.Marshal(filtered)
			broadcast("wan_peers_updated", string(data))
		}

	case "peer_online":
		var peerData RemotePeer
		if json.Unmarshal(msg.Data, &peerData) == nil && peerData.InstanceID != "" && peerData.InstanceID != Global.InstanceID() {
			sc.remotePeerMu.Lock()
			found := false
			for i, p := range sc.remotePeers {
				if p.InstanceID == peerData.InstanceID {
					sc.remotePeers[i] = peerData
					found = true
					break
				}
			}
			if !found {
				sc.remotePeers = append(sc.remotePeers, peerData)
			}
			sc.remotePeerMu.Unlock()

			if broadcast != nil {
				data, _ := json.Marshal(peerData)
				broadcast("wan_peer_online", string(data))
			}
		}

	case "peer_offline":
		var peerData RemotePeer
		if json.Unmarshal(msg.Data, &peerData) == nil && peerData.InstanceID != "" {
			sc.remotePeerMu.Lock()
			for i, p := range sc.remotePeers {
				if p.InstanceID == peerData.InstanceID {
					sc.remotePeers = append(sc.remotePeers[:i], sc.remotePeers[i+1:]...)
					break
				}
			}
			sc.remotePeerMu.Unlock()

			if broadcast != nil {
				data, _ := json.Marshal(peerData)
				broadcast("wan_peer_offline", string(data))
			}
		}

	case "delivery_failed":
		var failData struct {
			To           string `json:"to"`
			Reason       string `json:"reason"`
			OriginalType string `json:"original_type"`
		}
		if json.Unmarshal(msg.Data, &failData) == nil {
			log.Printf("[signaling] delivery_failed: to=%s reason=%s type=%s", failData.To, failData.Reason, failData.OriginalType)
			if broadcast != nil {
				broadcast("wan_delivery_failed", string(msg.Data))
				notif, _ := json.Marshal(map[string]string{
					"title": "消息投递失败",
					"body":  fmt.Sprintf("无法将消息发送给对方: %s", failData.Reason),
				})
				broadcast("desktop_notify", string(notif))
			}
		}

	case "conversation_invite":
		log.Printf("[signaling] received conversation_invite from=%s, dataLen=%d, data=%s",
			msg.From, len(msg.Data), truncateLog(string(msg.Data), 500))
		// 调用 relayHandler 在本地创建对话记录，handler 内部会 broadcast a2a_conversation_request 给前端
		if relayHandler != nil {
			result := relayHandler(msg.From, "/conversation/invite", msg.Data)
			log.Printf("[signaling] conversation_invite relayHandler result: %v", result)
			// 如果 relayHandler 失败，则手动 broadcast 通知前端（降级方案）
			if resultMap, ok := result.(map[string]interface{}); ok {
				if errMsg, hasErr := resultMap["error"]; hasErr {
					log.Printf("[signaling] ERROR: conversation_invite handler failed: %v", errMsg)
					if broadcast != nil {
						payload := make(map[string]interface{})
						json.Unmarshal(msg.Data, &payload)
						payload["from_peer_id"] = msg.From
						data, _ := json.Marshal(payload)
						broadcast("a2a_conversation_invite", string(data))
					}
				}
			}
		} else {
			log.Printf("[signaling] ERROR: relayHandler is nil for conversation_invite!")
			if broadcast != nil {
				payload := make(map[string]interface{})
				json.Unmarshal(msg.Data, &payload)
				payload["from_peer_id"] = msg.From
				data, _ := json.Marshal(payload)
				broadcast("a2a_conversation_invite", string(data))
			}
		}

	case "conversation_accept":
		log.Printf("[signaling] received conversation_accept from=%s, dataLen=%d", msg.From, len(msg.Data))
		if relayHandler != nil {
			result := relayHandler(msg.From, "/conversation/accept", msg.Data)
			log.Printf("[signaling] conversation_accept relayHandler result: %v", result)
		} else {
			log.Printf("[signaling] ERROR: relayHandler is nil for conversation_accept!")
		}

	case "conversation_reject":
		log.Printf("[signaling] received conversation_reject from=%s, dataLen=%d", msg.From, len(msg.Data))
		if relayHandler != nil {
			result := relayHandler(msg.From, "/conversation/reject", msg.Data)
			log.Printf("[signaling] conversation_reject relayHandler result: %v", result)
		} else {
			log.Printf("[signaling] ERROR: relayHandler is nil for conversation_reject!")
		}

	case "relay":
		sc.handleRelayMessage(msg)

	case "registered", "heartbeat_ack":
		// 正常确认消息
	}

	// 调用注册的 handler
	sc.handlerMu.RLock()
	handler := sc.handlers[msg.Type]
	sc.handlerMu.RUnlock()
	if handler != nil {
		handler(msg)
	}
}

func (sc *SignalingClient) handleRelayMessage(msg SignalMessage) {
	var relayData struct {
		Path    string          `json:"path"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(msg.Data, &relayData); err != nil {
		log.Printf("[signaling] handleRelayMessage: unmarshal error: %v, dataLen=%d", err, len(msg.Data))
		return
	}

	log.Printf("[signaling] relay from=%s path=%s payloadLen=%d", msg.From, relayData.Path, len(relayData.Payload))

	if relayHandler != nil {
		response := relayHandler(msg.From, relayData.Path, relayData.Payload)
		if response != nil {
			respData, _ := json.Marshal(map[string]interface{}{
				"path":    relayData.Path + "_response",
				"payload": response,
			})
			sc.sendMsg(SignalMessage{
				Type: "relay",
				To:   msg.From,
				Data: respData,
			})
		}
	} else {
		log.Printf("[signaling] WARNING: relayHandler is nil, cannot process relay message")
	}
}

func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (sc *SignalingClient) sendMsg(msg SignalMessage) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.conn == nil {
		return fmt.Errorf("not connected")
	}
	return sc.conn.WriteJSON(msg)
}

// RelayHandler 处理通过信令服务器中继的 Nexus 协议消息
type RelayHandler func(fromPeerID string, path string, payload json.RawMessage) interface{}

var relayHandler RelayHandler

// SetRelayHandler 注入中继消息处理函数
func SetRelayHandler(h RelayHandler) {
	relayHandler = h
}
