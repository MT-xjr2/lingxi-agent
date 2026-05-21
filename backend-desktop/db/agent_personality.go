package db

import (
	"database/sql"
	"log/slog"
	"time"
)

// AgentPersonality 群聊人格配置（每个 Agent 一行，可选）
// 控制 Agent 在群聊场景下的发言行为：发言概率、延迟、兴趣、冷场救场等
type AgentPersonality struct {
	AgentID           int64     `json:"agent_id"`
	Tags              string    `json:"tags"`               // JSON 数组：["佛系","捧场王",...]
	Interests         string    `json:"interests"`          // JSON 数组：["前端","咖啡",...]
	SpeakProbability  int       `json:"speak_probability"`  // 0-100，基础发言概率
	MinDelayMs        int       `json:"min_delay_ms"`       // 最小思考延迟（毫秒）
	MaxDelayMs        int       `json:"max_delay_ms"`       // 最大思考延迟（毫秒）
	EmojiFreq         string    `json:"emoji_freq"`         // low/medium/high
	QuietStart        string    `json:"quiet_start"`        // HH:MM 安静时段开始
	QuietEnd          string    `json:"quiet_end"`          // HH:MM 安静时段结束
	TypoRate          int       `json:"typo_rate"`          // 0-100 打错别字概率（%）
	EchoRate          int       `json:"echo_rate"`          // 0-100 复读率（%）
	GhostMinutes      int       `json:"ghost_minutes"`      // 被怼后消失分钟数
	ColdStartEligible bool      `json:"cold_start_eligible"`// 是否冷场救场
	StyleHint         string    `json:"style_hint"`         // 额外 system prompt 片段
	UpdatedAt         time.Time `json:"updated_at"`
}

// MigrateAgentPersonality 建表
func MigrateAgentPersonality() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agent_personalities (
			agent_id            INTEGER PRIMARY KEY,
			tags                TEXT    NOT NULL DEFAULT '[]',
			interests           TEXT    NOT NULL DEFAULT '[]',
			speak_probability   INTEGER NOT NULL DEFAULT 35,
			min_delay_ms        INTEGER NOT NULL DEFAULT 1500,
			max_delay_ms        INTEGER NOT NULL DEFAULT 5000,
			emoji_freq          TEXT    NOT NULL DEFAULT 'medium',
			quiet_start         TEXT    NOT NULL DEFAULT '',
			quiet_end           TEXT    NOT NULL DEFAULT '',
			typo_rate           INTEGER NOT NULL DEFAULT 1,
			echo_rate           INTEGER NOT NULL DEFAULT 2,
			ghost_minutes       INTEGER NOT NULL DEFAULT 0,
			cold_start_eligible INTEGER NOT NULL DEFAULT 1,
			style_hint          TEXT    NOT NULL DEFAULT '',
			updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			slog.Warn("agent_personalities migrate", "err", err)
		}
	}
}

const personalityCols = `agent_id, tags, interests, speak_probability, min_delay_ms, max_delay_ms,
	emoji_freq, quiet_start, quiet_end, typo_rate, echo_rate, ghost_minutes,
	cold_start_eligible, style_hint, updated_at`

func scanPersonality(scanner interface{ Scan(...interface{}) error }) (*AgentPersonality, error) {
	var p AgentPersonality
	var coldStart int
	err := scanner.Scan(&p.AgentID, &p.Tags, &p.Interests, &p.SpeakProbability,
		&p.MinDelayMs, &p.MaxDelayMs, &p.EmojiFreq, &p.QuietStart, &p.QuietEnd,
		&p.TypoRate, &p.EchoRate, &p.GhostMinutes, &coldStart, &p.StyleHint, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.ColdStartEligible = coldStart == 1
	return &p, nil
}

// GetPersonality 查询单个 Agent 的人格设置；不存在则返回带默认值的对象
func GetPersonality(agentID int64) (*AgentPersonality, error) {
	row := DB.QueryRow(`SELECT `+personalityCols+` FROM agent_personalities WHERE agent_id=?`, agentID)
	p, err := scanPersonality(row)
	if err == sql.ErrNoRows {
		return defaultPersonality(agentID), nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// defaultPersonality 没有显式配置时使用的默认人格
func defaultPersonality(agentID int64) *AgentPersonality {
	return &AgentPersonality{
		AgentID:           agentID,
		Tags:              "[]",
		Interests:         "[]",
		SpeakProbability:  35,
		MinDelayMs:        1500,
		MaxDelayMs:        5000,
		EmojiFreq:         "medium",
		QuietStart:        "",
		QuietEnd:          "",
		TypoRate:          1,
		EchoRate:          2,
		GhostMinutes:      0,
		ColdStartEligible: true,
		StyleHint:         "",
	}
}

// UpsertPersonality 插入或更新（按 agent_id 主键）
func UpsertPersonality(p *AgentPersonality) error {
	if p.Tags == "" {
		p.Tags = "[]"
	}
	if p.Interests == "" {
		p.Interests = "[]"
	}
	if p.EmojiFreq == "" {
		p.EmojiFreq = "medium"
	}
	if p.SpeakProbability < 0 {
		p.SpeakProbability = 0
	}
	if p.SpeakProbability > 100 {
		p.SpeakProbability = 100
	}
	if p.MinDelayMs < 0 {
		p.MinDelayMs = 0
	}
	if p.MaxDelayMs < p.MinDelayMs {
		p.MaxDelayMs = p.MinDelayMs + 1000
	}
	coldStart := 0
	if p.ColdStartEligible {
		coldStart = 1
	}
	_, err := DB.Exec(`
		INSERT INTO agent_personalities
			(agent_id, tags, interests, speak_probability, min_delay_ms, max_delay_ms,
			 emoji_freq, quiet_start, quiet_end, typo_rate, echo_rate, ghost_minutes,
			 cold_start_eligible, style_hint, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(agent_id) DO UPDATE SET
			tags=excluded.tags,
			interests=excluded.interests,
			speak_probability=excluded.speak_probability,
			min_delay_ms=excluded.min_delay_ms,
			max_delay_ms=excluded.max_delay_ms,
			emoji_freq=excluded.emoji_freq,
			quiet_start=excluded.quiet_start,
			quiet_end=excluded.quiet_end,
			typo_rate=excluded.typo_rate,
			echo_rate=excluded.echo_rate,
			ghost_minutes=excluded.ghost_minutes,
			cold_start_eligible=excluded.cold_start_eligible,
			style_hint=excluded.style_hint,
			updated_at=CURRENT_TIMESTAMP`,
		p.AgentID, p.Tags, p.Interests, p.SpeakProbability, p.MinDelayMs, p.MaxDelayMs,
		p.EmojiFreq, p.QuietStart, p.QuietEnd, p.TypoRate, p.EchoRate, p.GhostMinutes,
		coldStart, p.StyleHint)
	return err
}

// ListPersonalities 批量查询多个 Agent 的人格（用于群聊调度时一次性加载）
// 未配置的会返回默认值
func ListPersonalities(agentIDs []int64) (map[int64]*AgentPersonality, error) {
	out := make(map[int64]*AgentPersonality, len(agentIDs))
	for _, id := range agentIDs {
		out[id] = defaultPersonality(id)
	}
	if len(agentIDs) == 0 {
		return out, nil
	}

	// 构造 IN 占位符
	placeholders := ""
	args := make([]interface{}, 0, len(agentIDs))
	for i, id := range agentIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	rows, err := DB.Query(`SELECT `+personalityCols+` FROM agent_personalities WHERE agent_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPersonality(rows)
		if err != nil {
			continue
		}
		out[p.AgentID] = p
	}
	return out, nil
}
