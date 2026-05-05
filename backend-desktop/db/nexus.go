package db

import (
	"database/sql"
	"time"
)

// ─── Nexus Settings ─────────────────────────────────────────────

type NexusSettings struct {
	Visible    bool   `json:"visible"`
	Nickname   string `json:"nickname"`
	ListenPort int    `json:"listen_port"`
}

func GetNexusSettings() (*NexusSettings, error) {
	var s NexusSettings
	var visible int
	err := DB.QueryRow(`SELECT visible, nickname, listen_port FROM nexus_settings WHERE id=1`).
		Scan(&visible, &s.Nickname, &s.ListenPort)
	if err != nil {
		return &NexusSettings{Visible: true, ListenPort: 3001}, nil
	}
	s.Visible = visible == 1
	return &s, nil
}

func UpdateNexusSettings(s *NexusSettings) error {
	v := 0
	if s.Visible {
		v = 1
	}
	_, err := DB.Exec(`UPDATE nexus_settings SET visible=?, nickname=?, listen_port=? WHERE id=1`,
		v, s.Nickname, s.ListenPort)
	return err
}

// ─── Nexus Peers ────────────────────────────────────────────────

type NexusPeer struct {
	ID         string    `json:"id"`
	Nickname   string    `json:"nickname"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	AgentsJSON string    `json:"agents_json"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

func UpsertNexusPeer(p *NexusPeer) error {
	_, err := DB.Exec(`
		INSERT INTO nexus_peers (id, nickname, host, port, agents_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			nickname=excluded.nickname,
			host=excluded.host,
			port=excluded.port,
			agents_json=excluded.agents_json,
			last_seen_at=excluded.last_seen_at
	`, p.ID, p.Nickname, p.Host, p.Port, p.AgentsJSON, time.Now())
	return err
}

func ListNexusPeers() ([]NexusPeer, error) {
	rows, err := DB.Query(`SELECT id, nickname, host, port, agents_json, last_seen_at FROM nexus_peers ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NexusPeer
	for rows.Next() {
		var p NexusPeer
		if err := rows.Scan(&p.ID, &p.Nickname, &p.Host, &p.Port, &p.AgentsJSON, &p.LastSeenAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func DeleteNexusPeer(id string) {
	DB.Exec(`DELETE FROM nexus_peers WHERE id=?`, id)
}

func CleanStalePeers(before time.Time) {
	DB.Exec(`DELETE FROM nexus_peers WHERE last_seen_at < ?`, before)
}

// ─── Nexus Contacts ─────────────────────────────────────────────

type NexusContact struct {
	ID           int64     `json:"id"`
	PeerID       string    `json:"peer_id"`
	Nickname     string    `json:"nickname"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Status       string    `json:"status"`
	SharedSecret string    `json:"shared_secret,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func CreateNexusContact(c *NexusContact) (int64, error) {
	res, err := DB.Exec(`INSERT INTO nexus_contacts (peer_id, nickname, host, port, status, shared_secret)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.PeerID, c.Nickname, c.Host, c.Port, c.Status, c.SharedSecret)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListNexusContacts() ([]NexusContact, error) {
	rows, err := DB.Query(`SELECT id, peer_id, nickname, host, port, status, shared_secret, created_at
		FROM nexus_contacts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NexusContact
	for rows.Next() {
		var c NexusContact
		if err := rows.Scan(&c.ID, &c.PeerID, &c.Nickname, &c.Host, &c.Port, &c.Status, &c.SharedSecret, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func GetNexusContact(id int64) (*NexusContact, error) {
	var c NexusContact
	err := DB.QueryRow(`SELECT id, peer_id, nickname, host, port, status, shared_secret, created_at
		FROM nexus_contacts WHERE id=?`, id).
		Scan(&c.ID, &c.PeerID, &c.Nickname, &c.Host, &c.Port, &c.Status, &c.SharedSecret, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func GetNexusContactByPeerID(peerID string) (*NexusContact, error) {
	var c NexusContact
	err := DB.QueryRow(`SELECT id, peer_id, nickname, host, port, status, shared_secret, created_at
		FROM nexus_contacts WHERE peer_id=?`, peerID).
		Scan(&c.ID, &c.PeerID, &c.Nickname, &c.Host, &c.Port, &c.Status, &c.SharedSecret, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateNexusContactStatus(id int64, status, sharedSecret string) error {
	_, err := DB.Exec(`UPDATE nexus_contacts SET status=?, shared_secret=? WHERE id=?`, status, sharedSecret, id)
	return err
}

func UpdateNexusContactNickname(id int64, nickname string) error {
	_, err := DB.Exec(`UPDATE nexus_contacts SET nickname=? WHERE id=?`, nickname, id)
	return err
}

func DeleteNexusContact(id int64) error {
	_, err := DB.Exec(`DELETE FROM nexus_contacts WHERE id=?`, id)
	return err
}

func NexusContactExistsBySecret(secret string) bool {
	var count int
	err := DB.QueryRow(`SELECT COUNT(*) FROM nexus_contacts WHERE shared_secret=? AND status IN ('connected','pending','pending_incoming')`, secret).Scan(&count)
	return err == nil && count > 0
}

// ─── Agent Nexus Config ─────────────────────────────────────────

type AgentNexusConfig struct {
	AgentID            int64  `json:"agent_id"`
	Public             bool   `json:"public"`
	PublicName         string `json:"public_name"`
	CapabilityTags     string `json:"capability_tags"`
	AuthLevel          string `json:"auth_level"`
	ForbiddenInfo      string `json:"forbidden_info"`
	PublicKnowledgeIDs string `json:"public_knowledge_ids"`
}

func GetAgentNexusConfig(agentID int64) (*AgentNexusConfig, error) {
	var c AgentNexusConfig
	var pub int
	err := DB.QueryRow(`SELECT agent_id, public, public_name, capability_tags, auth_level, forbidden_info, public_knowledge_ids
		FROM agent_nexus_config WHERE agent_id=?`, agentID).
		Scan(&c.AgentID, &pub, &c.PublicName, &c.CapabilityTags, &c.AuthLevel, &c.ForbiddenInfo, &c.PublicKnowledgeIDs)
	if err == sql.ErrNoRows {
		return &AgentNexusConfig{AgentID: agentID, CapabilityTags: "[]", AuthLevel: "readonly", PublicKnowledgeIDs: "[]"}, nil
	}
	if err != nil {
		return nil, err
	}
	c.Public = pub == 1
	return &c, nil
}

func UpsertAgentNexusConfig(c *AgentNexusConfig) error {
	pub := 0
	if c.Public {
		pub = 1
	}
	_, err := DB.Exec(`
		INSERT INTO agent_nexus_config (agent_id, public, public_name, capability_tags, auth_level, forbidden_info, public_knowledge_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			public=excluded.public,
			public_name=excluded.public_name,
			capability_tags=excluded.capability_tags,
			auth_level=excluded.auth_level,
			forbidden_info=excluded.forbidden_info,
			public_knowledge_ids=excluded.public_knowledge_ids
	`, c.AgentID, pub, c.PublicName, c.CapabilityTags, c.AuthLevel, c.ForbiddenInfo, c.PublicKnowledgeIDs)
	return err
}

func ListPublicAgentConfigs() ([]AgentNexusConfig, error) {
	rows, err := DB.Query(`SELECT agent_id, public, public_name, capability_tags, auth_level, forbidden_info, public_knowledge_ids
		FROM agent_nexus_config WHERE public=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentNexusConfig
	for rows.Next() {
		var c AgentNexusConfig
		var pub int
		if err := rows.Scan(&c.AgentID, &pub, &c.PublicName, &c.CapabilityTags, &c.AuthLevel, &c.ForbiddenInfo, &c.PublicKnowledgeIDs); err != nil {
			continue
		}
		c.Public = pub == 1
		out = append(out, c)
	}
	return out, nil
}

// ─── A2A Conversations ──────────────────────────────────────────

type A2AConversation struct {
	ID                 int64      `json:"id"`
	LocalAgentID       int64      `json:"local_agent_id"`
	RemoteAgentName    string     `json:"remote_agent_name"`
	RemotePeerID       string     `json:"remote_peer_id"`
	RemotePeerNickname string     `json:"remote_peer_nickname"`
	Topic              string     `json:"topic"`
	Goal               string     `json:"goal"`
	InitialPrompt      string     `json:"initial_prompt"`
	MaxRounds          int        `json:"max_rounds"`
	CurrentRound       int        `json:"current_round"`
	Status             string     `json:"status"`
	RequireApproval    bool       `json:"require_approval"`
	Summary            string     `json:"summary"`
	DecisionsJSON      string     `json:"decisions_json"`
	InitiatedBy        string     `json:"initiated_by"`
	RemoteConvID       int64      `json:"remote_conv_id"`
	LocalSessionID     int64      `json:"local_session_id"`
	Deadline           *time.Time `json:"deadline"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func CreateA2AConversation(c *A2AConversation) (int64, error) {
	reqAppr := 0
	if c.RequireApproval {
		reqAppr = 1
	}
	res, err := DB.Exec(`
		INSERT INTO a2a_conversations
			(local_agent_id, remote_agent_name, remote_peer_id, remote_peer_nickname,
			 topic, goal, initial_prompt, max_rounds, status, require_approval, initiated_by, deadline, remote_conv_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.LocalAgentID, c.RemoteAgentName, c.RemotePeerID, c.RemotePeerNickname,
		c.Topic, c.Goal, c.InitialPrompt, c.MaxRounds, c.Status, reqAppr, c.InitiatedBy, c.Deadline, c.RemoteConvID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetA2AConversationByRemoteID(remoteConvID int64) (*A2AConversation, error) {
	var c A2AConversation
	var reqAppr int
	err := DB.QueryRow(`
		SELECT id, local_agent_id, remote_agent_name, remote_peer_id, remote_peer_nickname,
		       topic, goal, initial_prompt, max_rounds, current_round, status, require_approval,
		       summary, decisions_json, initiated_by, remote_conv_id, local_session_id, deadline, created_at, updated_at
		FROM a2a_conversations WHERE remote_conv_id=? AND remote_conv_id!=0 ORDER BY id DESC LIMIT 1`, remoteConvID).
		Scan(&c.ID, &c.LocalAgentID, &c.RemoteAgentName, &c.RemotePeerID, &c.RemotePeerNickname,
			&c.Topic, &c.Goal, &c.InitialPrompt, &c.MaxRounds, &c.CurrentRound, &c.Status, &reqAppr,
			&c.Summary, &c.DecisionsJSON, &c.InitiatedBy, &c.RemoteConvID, &c.LocalSessionID, &c.Deadline, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.RequireApproval = reqAppr == 1
	return &c, nil
}

func GetA2AConversation(id int64) (*A2AConversation, error) {
	var c A2AConversation
	var reqAppr int
	err := DB.QueryRow(`
		SELECT id, local_agent_id, remote_agent_name, remote_peer_id, remote_peer_nickname,
		       topic, goal, initial_prompt, max_rounds, current_round, status, require_approval,
		       summary, decisions_json, initiated_by, remote_conv_id, local_session_id, deadline, created_at, updated_at
		FROM a2a_conversations WHERE id=?`, id).
		Scan(&c.ID, &c.LocalAgentID, &c.RemoteAgentName, &c.RemotePeerID, &c.RemotePeerNickname,
			&c.Topic, &c.Goal, &c.InitialPrompt, &c.MaxRounds, &c.CurrentRound, &c.Status, &reqAppr,
			&c.Summary, &c.DecisionsJSON, &c.InitiatedBy, &c.RemoteConvID, &c.LocalSessionID, &c.Deadline, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.RequireApproval = reqAppr == 1
	return &c, nil
}

func ListA2AConversations() ([]A2AConversation, error) {
	rows, err := DB.Query(`
		SELECT id, local_agent_id, remote_agent_name, remote_peer_id, remote_peer_nickname,
		       topic, goal, initial_prompt, max_rounds, current_round, status, require_approval,
		       summary, decisions_json, initiated_by, remote_conv_id, local_session_id, deadline, created_at, updated_at
		FROM a2a_conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []A2AConversation
	for rows.Next() {
		var c A2AConversation
		var reqAppr int
		if err := rows.Scan(&c.ID, &c.LocalAgentID, &c.RemoteAgentName, &c.RemotePeerID, &c.RemotePeerNickname,
			&c.Topic, &c.Goal, &c.InitialPrompt, &c.MaxRounds, &c.CurrentRound, &c.Status, &reqAppr,
			&c.Summary, &c.DecisionsJSON, &c.InitiatedBy, &c.RemoteConvID, &c.LocalSessionID, &c.Deadline, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.RequireApproval = reqAppr == 1
		out = append(out, c)
	}
	return out, nil
}

func UpdateA2AConversationStatus(id int64, status string) error {
	_, err := DB.Exec(`UPDATE a2a_conversations SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func UpdateA2AConversationRound(id int64, round int) error {
	_, err := DB.Exec(`UPDATE a2a_conversations SET current_round=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, round, id)
	return err
}

func UpdateA2AConversationSessionID(id, sessionID int64) error {
	_, err := DB.Exec(`UPDATE a2a_conversations SET local_session_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, sessionID, id)
	return err
}

func UpdateA2AConversationSummary(id int64, summary, decisionsJSON string) error {
	_, err := DB.Exec(`UPDATE a2a_conversations SET summary=?, decisions_json=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		summary, decisionsJSON, id)
	return err
}

// ─── A2A Messages ───────────────────────────────────────────────

type A2AMessage struct {
	ID              int64     `json:"id"`
	ConversationID  int64     `json:"conversation_id"`
	Sender          string    `json:"sender"`
	SenderAgentName string    `json:"sender_agent_name"`
	MsgType         string    `json:"msg_type"`
	Content         string    `json:"content"`
	StructuredData  string    `json:"structured_data"`
	CreatedAt       time.Time `json:"created_at"`
}

func CreateA2AMessage(m *A2AMessage) (int64, error) {
	res, err := DB.Exec(`
		INSERT INTO a2a_messages (conversation_id, sender, sender_agent_name, msg_type, content, structured_data)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.ConversationID, m.Sender, m.SenderAgentName, m.MsgType, m.Content, m.StructuredData)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListA2AMessages(conversationID int64) ([]A2AMessage, error) {
	rows, err := DB.Query(`
		SELECT id, conversation_id, sender, sender_agent_name, msg_type, content, structured_data, created_at
		FROM a2a_messages WHERE conversation_id=? ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []A2AMessage
	for rows.Next() {
		var m A2AMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Sender, &m.SenderAgentName, &m.MsgType, &m.Content, &m.StructuredData, &m.CreatedAt); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}
