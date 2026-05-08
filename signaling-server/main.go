package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─── 数据结构 ────────────────────────────────────────────────────

// writeMsg 是写入通道中的消息载体，区分 JSON 消息和 WebSocket 控制帧
type writeMsg struct {
	json    *SignalMessage // JSON 消息（二选一）
	msgType int           // 控制帧类型（websocket.PingMessage 等）
	data    []byte        // 控制帧数据
}

type PeerInfo struct {
	InstanceID string  `json:"instance_id"`
	Nickname   string  `json:"nickname"`
	UserID     string  `json:"user_id,omitempty"`
	AvatarURL  string  `json:"avatar_url,omitempty"`
	Agents     []Agent `json:"agents"`
	Platform   string  `json:"platform,omitempty"`
	DeviceName string  `json:"device_name,omitempty"`
	LocalIP    string  `json:"local_ip,omitempty"`
	LocalPort  int     `json:"local_port,omitempty"`
	conn       *websocket.Conn
	lastSeen   time.Time
	writeCh    chan writeMsg
	closeCh    chan struct{}
	closeOnce  sync.Once
}

// enqueue 向写入通道发送消息，通道满或已关闭时静默丢弃
func (p *PeerInfo) enqueue(msg writeMsg) {
	select {
	case p.writeCh <- msg:
	case <-p.closeCh:
	default:
		log.Printf("[peer] write channel full, dropping message for %s", p.InstanceID)
	}
}

// enqueueJSON 便捷方法：发送 JSON 消息
func (p *PeerInfo) enqueueJSON(msg SignalMessage) {
	p.enqueue(writeMsg{json: &msg})
}

// writeLoop 是每个 peer 唯一的写协程，串行化所有写操作
func (p *PeerInfo) writeLoop() {
	defer p.conn.Close()
	for {
		select {
		case <-p.closeCh:
			return
		case wm := <-p.writeCh:
			if wm.json != nil {
				if err := p.conn.WriteJSON(wm.json); err != nil {
					log.Printf("[peer] writeLoop WriteJSON error for %s: %v", p.InstanceID, err)
					return
				}
			} else {
				if err := p.conn.WriteMessage(wm.msgType, wm.data); err != nil {
					log.Printf("[peer] writeLoop WriteMessage error for %s: %v", p.InstanceID, err)
					return
				}
			}
		}
	}
}

// close 关闭 peer 的写入通道（幂等）
func (p *PeerInfo) close() {
	p.closeOnce.Do(func() {
		close(p.closeCh)
	})
}

type Agent struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	CapabilityTags []string `json:"capability_tags"`
	AuthLevel      string   `json:"auth_level"`
}

type SignalMessage struct {
	Type string          `json:"type"`
	From string          `json:"from,omitempty"`
	To   string          `json:"to,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// ─── Hub 管理所有连接 ────────────────────────────────────────────

type Hub struct {
	mu    sync.RWMutex
	peers map[string]*PeerInfo
}

var hub = &Hub{peers: make(map[string]*PeerInfo)}

var allowedOrigins = os.Getenv("ALLOWED_ORIGINS")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		if allowedOrigins == "" {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, o := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(o) == origin {
				return true
			}
		}
		return origin == ""
	},
}

func (h *Hub) register(p *PeerInfo) {
	h.mu.Lock()
	if old, ok := h.peers[p.InstanceID]; ok {
		old.close()
	}
	h.peers[p.InstanceID] = p
	h.mu.Unlock()
	log.Printf("[hub] registered: %s (%s) platform=%s device=%s ip=%s port=%d",
		p.InstanceID, p.Nickname, p.Platform, p.DeviceName, p.LocalIP, p.LocalPort)

	h.broadcastPeerEvent("peer_online", p)
}

func (h *Hub) unregister(id string) {
	h.mu.Lock()
	p := h.peers[id]
	delete(h.peers, id)
	h.mu.Unlock()
	log.Printf("[hub] unregistered: %s", id)

	if p != nil {
		p.close()
		h.broadcastPeerEvent("peer_offline", p)
	}
}

func (h *Hub) broadcastPeerEvent(eventType string, p *PeerInfo) {
	data := jsonRaw(map[string]interface{}{
		"instance_id": p.InstanceID,
		"nickname":    p.Nickname,
		"user_id":     p.UserID,
		"avatar_url":  p.AvatarURL,
		"agents":      p.Agents,
		"platform":    p.Platform,
		"device_name": p.DeviceName,
		"local_ip":    p.LocalIP,
		"local_port":  p.LocalPort,
	})
	msg := SignalMessage{Type: eventType, Data: data}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, peer := range h.peers {
		if id == p.InstanceID {
			continue
		}
		peer.enqueueJSON(msg)
	}
}

func (h *Hub) getPeer(id string) *PeerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peers[id]
}

func (h *Hub) listOnlinePeers() []map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(h.peers))
	for _, p := range h.peers {
		out = append(out, map[string]interface{}{
			"instance_id": p.InstanceID,
			"nickname":    p.Nickname,
			"user_id":     p.UserID,
			"avatar_url":  p.AvatarURL,
			"agents":      p.Agents,
			"platform":    p.Platform,
			"device_name": p.DeviceName,
			"local_ip":    p.LocalIP,
			"local_port":  p.LocalPort,
		})
	}
	return out
}

// sendTo 通过写入通道将消息投递给目标 peer，不再直接写连接
func (h *Hub) sendTo(targetID string, msg SignalMessage) error {
	h.mu.RLock()
	p := h.peers[targetID]
	h.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("peer_offline")
	}
	select {
	case <-p.closeCh:
		return fmt.Errorf("peer_offline")
	default:
	}
	p.enqueueJSON(msg)
	return nil
}

// ─── WebSocket 处理 ──────────────────────────────────────────────

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	var peer *PeerInfo

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ws] recovered panic in handleWS: %v", r)
		}
		if peer != nil {
			hub.unregister(peer.InstanceID)
		}
		conn.Close()
	}()

	// safeWrite 通过写入通道发送消息，完全避免 concurrent write
	safeWrite := func(msg SignalMessage) {
		if peer == nil {
			return
		}
		peer.enqueueJSON(msg)
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		var msg SignalMessage
		if json.Unmarshal(msgBytes, &msg) != nil {
			continue
		}

		switch msg.Type {
		case "register":
			var reg struct {
				InstanceID string  `json:"instance_id"`
				Nickname   string  `json:"nickname"`
				UserID     string  `json:"user_id"`
				AvatarURL  string  `json:"avatar_url"`
				Agents     []Agent `json:"agents"`
				Platform   string  `json:"platform"`
				DeviceName string  `json:"device_name"`
				LocalIP    string  `json:"local_ip"`
				LocalPort  int     `json:"local_port"`
			}
			json.Unmarshal(msg.Data, &reg)
			if reg.InstanceID == "" {
				continue
			}
			peer = &PeerInfo{
				InstanceID: reg.InstanceID,
				Nickname:   reg.Nickname,
				UserID:     reg.UserID,
				AvatarURL:  reg.AvatarURL,
				Agents:     reg.Agents,
				Platform:   reg.Platform,
				DeviceName: reg.DeviceName,
				LocalIP:    reg.LocalIP,
				LocalPort:  reg.LocalPort,
				conn:       conn,
				lastSeen:   time.Now(),
				writeCh:    make(chan writeMsg, 256),
				closeCh:    make(chan struct{}),
			}
			go peer.writeLoop()
			hub.register(peer)
			safeWrite(SignalMessage{Type: "registered", Data: jsonRaw(map[string]string{"status": "ok"})})

			// ping ticker 通过写入通道发送，不再直接写连接
			go func(p *PeerInfo) {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-p.closeCh:
						return
					case <-ticker.C:
						p.enqueue(writeMsg{msgType: websocket.PingMessage, data: nil})
					}
				}
			}(peer)

		case "list_peers":
			peers := hub.listOnlinePeers()
			safeWrite(SignalMessage{Type: "peers_list", Data: jsonRaw(peers)})

		case "relay":
			if msg.To == "" || peer == nil {
				continue
			}
			msg.From = peer.InstanceID
			log.Printf("[ws] relay from=%s to=%s dataLen=%d", peer.InstanceID, msg.To, len(msg.Data))
			if err := hub.sendTo(msg.To, msg); err != nil {
				log.Printf("[ws] relay delivery failed: from=%s to=%s reason=%v", peer.InstanceID, msg.To, err)
				safeWrite(SignalMessage{
					Type: "delivery_failed",
					Data: jsonRaw(map[string]string{"to": msg.To, "reason": "peer_offline", "original_type": "relay"}),
				})
			}

		case "conversation_invite":
			if msg.To == "" || peer == nil {
				continue
			}
			msg.From = peer.InstanceID
			log.Printf("[ws] conversation_invite from=%s to=%s", peer.InstanceID, msg.To)
			if err := hub.sendTo(msg.To, msg); err != nil {
				safeWrite(SignalMessage{
					Type: "delivery_failed",
					Data: jsonRaw(map[string]string{"to": msg.To, "reason": "peer_offline", "original_type": "conversation_invite"}),
				})
			}

		case "conversation_accept":
			if msg.To == "" || peer == nil {
				continue
			}
			msg.From = peer.InstanceID
			log.Printf("[ws] conversation_accept from=%s to=%s", peer.InstanceID, msg.To)
			if err := hub.sendTo(msg.To, msg); err != nil {
				safeWrite(SignalMessage{
					Type: "delivery_failed",
					Data: jsonRaw(map[string]string{"to": msg.To, "reason": "peer_offline", "original_type": "conversation_accept"}),
				})
			}

		case "conversation_reject":
			if msg.To == "" || peer == nil {
				continue
			}
			msg.From = peer.InstanceID
			log.Printf("[ws] conversation_reject from=%s to=%s", peer.InstanceID, msg.To)
			if err := hub.sendTo(msg.To, msg); err != nil {
				safeWrite(SignalMessage{
					Type: "delivery_failed",
					Data: jsonRaw(map[string]string{"to": msg.To, "reason": "peer_offline", "original_type": "conversation_reject"}),
				})
			}

		case "heartbeat":
			if peer != nil {
				peer.lastSeen = time.Now()
			}
			safeWrite(SignalMessage{Type: "heartbeat_ack"})
		}
	}
}

func jsonRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ─── HTTP 端点 ───────────────────────────────────────────────────

func handlePeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hub.listOnlinePeers())
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	hub.mu.RLock()
	count := len(hub.peers)
	hub.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "online": count})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/api/peers", handlePeers)
	http.HandleFunc("/health", handleHealth)

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	if tlsCert != "" && tlsKey != "" {
		log.Printf("[signaling] server starting on :%s (TLS/WSS)", port)
		if err := http.ListenAndServeTLS(":"+port, tlsCert, tlsKey, nil); err != nil {
			log.Fatalf("[signaling] server error: %v", err)
		}
	} else {
		log.Printf("[signaling] server starting on :%s (plain WS, set TLS_CERT & TLS_KEY for WSS)", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("[signaling] server error: %v", err)
		}
	}
}
