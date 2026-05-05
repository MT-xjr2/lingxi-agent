package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"lingxi-agent/db"
	"lingxi-agent/nexus"

	"github.com/gin-gonic/gin"
)

// ─── 建联请求/响应 ──────────────────────────────────────────────

// SendConnectRequest POST /api/contacts/request — 本地前端发起建联
func SendConnectRequest(c *gin.Context) {
	var body struct {
		PeerID   string `json:"peer_id"`
		Nickname string `json:"nickname"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否已经建联或已发送请求
	existing, _ := db.GetNexusContactByPeerID(body.PeerID)
	if existing != nil {
		switch existing.Status {
		case "connected":
			c.JSON(http.StatusOK, gin.H{"id": existing.ID, "status": "already_connected", "message": "已经建联"})
			return
		case "pending", "pending_incoming":
			c.JSON(http.StatusOK, gin.H{"id": existing.ID, "status": "already_pending", "message": "建联请求已发送，等待对方响应"})
			return
		}
	}

	secret := generatePSK()

	id, err := db.CreateNexusContact(&db.NexusContact{
		PeerID:       body.PeerID,
		Nickname:     body.Nickname,
		Host:         body.Host,
		Port:         body.Port,
		Status:       "pending",
		SharedSecret: secret,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	settings, _ := db.GetNexusSettings()
	nickname := settings.Nickname
	if nickname == "" {
		nickname = "灵犀用户"
	}

	payload := map[string]interface{}{
		"instance_id":   nexus.Global.InstanceID(),
		"nickname":      nickname,
		"host":          getLocalIP(),
		"port":          settings.ListenPort,
		"shared_secret": secret,
	}

	url := fmt.Sprintf("http://%s/api/nexus/connect-request", net.JoinHostPort(body.Host, fmt.Sprintf("%d", body.Port)))
	_, err = nexus.PostJSON(url, payload, "")
	if err != nil {
		db.UpdateNexusContactStatus(id, "failed", secret)
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接到对方: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "status": "pending"})
}

// NexusConnectRequest POST /api/nexus/connect-request — 接收来自其他实例的建联请求
func NexusConnectRequest(c *gin.Context) {
	var body struct {
		InstanceID   string `json:"instance_id"`
		Nickname     string `json:"nickname"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		SharedSecret string `json:"shared_secret"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, _ := db.GetNexusContactByPeerID(body.InstanceID)
	if existing != nil && existing.Status == "connected" {
		c.JSON(http.StatusOK, gin.H{"status": "already_connected"})
		return
	}

	id, err := db.CreateNexusContact(&db.NexusContact{
		PeerID:       body.InstanceID,
		Nickname:     body.Nickname,
		Host:         body.Host,
		Port:         body.Port,
		Status:       "pending_incoming",
		SharedSecret: body.SharedSecret,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// WS 通知前端弹窗
	payload, _ := json.Marshal(map[string]interface{}{
		"id":          id,
		"peer_id":     body.InstanceID,
		"nickname":    body.Nickname,
		"host":        body.Host,
		"port":        body.Port,
	})
	BroadcastWSEvent("nexus_connect_request", string(payload))

	c.JSON(http.StatusOK, gin.H{"status": "pending"})
}

// RespondConnect POST /api/contacts/:id/respond — 本地前端响应建联请求
func RespondConnect(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Accept bool `json:"accept"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := db.GetNexusContact(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	if body.Accept {
		db.UpdateNexusContactStatus(id, "connected", contact.SharedSecret)

		settings, _ := db.GetNexusSettings()
		nickname := settings.Nickname
		if nickname == "" {
			nickname = "灵犀用户"
		}

		payload := map[string]interface{}{
			"instance_id": nexus.Global.InstanceID(),
			"nickname":    nickname,
			"accepted":    true,
		}
		url := fmt.Sprintf("http://%s/api/nexus/connect-respond", net.JoinHostPort(contact.Host, fmt.Sprintf("%d", contact.Port)))
		nexus.PostJSON(url, payload, contact.SharedSecret)
	} else {
		db.UpdateNexusContactStatus(id, "rejected", "")

		payload := map[string]interface{}{
			"instance_id": nexus.Global.InstanceID(),
			"accepted":    false,
		}
		url := fmt.Sprintf("http://%s/api/nexus/connect-respond", net.JoinHostPort(contact.Host, fmt.Sprintf("%d", contact.Port)))
		nexus.PostJSON(url, payload, "")
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// NexusConnectRespond POST /api/nexus/connect-respond — 接收建联响应
func NexusConnectRespond(c *gin.Context) {
	var body struct {
		InstanceID string `json:"instance_id"`
		Nickname   string `json:"nickname"`
		Accepted   bool   `json:"accepted"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := db.GetNexusContactByPeerID(body.InstanceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	if body.Accepted {
		// 更新对方昵称（对方可能在同意时返回了新的昵称）
		if body.Nickname != "" && body.Nickname != contact.Nickname {
			db.UpdateNexusContactNickname(contact.ID, body.Nickname)
		}
		db.UpdateNexusContactStatus(contact.ID, "connected", contact.SharedSecret)
	} else {
		db.UpdateNexusContactStatus(contact.ID, "rejected", "")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"peer_id":  body.InstanceID,
		"accepted": body.Accepted,
	})
	BroadcastWSEvent("nexus_connect_response", string(payload))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListContacts GET /api/contacts
func ListContacts(c *gin.Context) {
	contacts, err := db.ListNexusContacts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contacts == nil {
		contacts = []db.NexusContact{}
	}
	// 不暴露 shared_secret 给前端
	type safeContact struct {
		ID        int64  `json:"id"`
		PeerID    string `json:"peer_id"`
		Nickname  string `json:"nickname"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]safeContact, 0, len(contacts))
	for _, ct := range contacts {
		out = append(out, safeContact{
			ID:        ct.ID,
			PeerID:    ct.PeerID,
			Nickname:  ct.Nickname,
			Host:      ct.Host,
			Port:      ct.Port,
			Status:    ct.Status,
			CreatedAt: ct.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, out)
}

// DeleteContact DELETE /api/contacts/:id
func DeleteContact(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := db.DeleteNexusContact(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── 工具函数 ───────────────────────────────────────────────────

func generatePSK() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getLocalIP() string {
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
