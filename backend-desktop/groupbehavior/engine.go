// Package groupbehavior 群聊行为引擎：根据 Agent 人格与上下文，决定谁说话、何时说、说什么。
// 相对原来的"主持人选下一个发言者"模式，本引擎采用并发评估：
// 对每个加入的本地 Agent 独立计算发言概率，rolled 通过的 Agent 各自延迟后发言，更接近真实群聊。
package groupbehavior

import (
	"encoding/json"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"lingxi-agent/db"
)

// SpeakDecision 单个 Agent 的发言决策
type SpeakDecision struct {
	AgentID     int64    `json:"agent_id"`
	AgentName   string   `json:"agent_name"`
	Probability int      `json:"probability"` // 最终 %（用于日志）
	DelayMs     int      `json:"delay_ms"`    // 等待该毫秒后再发言
	Forced      bool     `json:"forced"`      // 被强制（如 @mention）
	Reasons     []string `json:"reasons"`     // 评分原因（用于调试）
}

// Context 评估上下文
type Context struct {
	Room       *db.GroupChat
	NewMsg     *db.GroupMessage // 触发本次评估的消息（用户/远端/Agent 都可）；冷场触发时为 nil
	Recent     []db.GroupMessage
	Members    []db.GroupMember
	IsColdStart bool // 冷场救场触发
	Now        time.Time
}

var mentionRe = regexp.MustCompile(`@([\p{Han}\w_-]{1,40})`)

// PickSpeakers 对每个 joined 本地 Agent 进行独立评估
// 返回所有"摇到"发言的 Agent 决策，按延迟升序
func PickSpeakers(ctx Context) []SpeakDecision {
	if ctx.Now.IsZero() {
		ctx.Now = time.Now()
	}
	localAgents := localJoinedAgents(ctx.Members)
	if len(localAgents) == 0 {
		return nil
	}
	personalities, _ := db.ListPersonalities(agentIDs(localAgents))

	// 提及解析
	mentionedNames := parseMentionNames(msgContent(ctx.NewMsg))
	mentionedSet := map[string]bool{}
	for _, n := range mentionedNames {
		mentionedSet[n] = true
	}

	// 最近发言者集合（用于反刷屏）
	recentSpeakers := map[string]int{} // name -> 距离当前 = index from back（0=最近）
	for i := len(ctx.Recent) - 1; i >= 0; i-- {
		name := ctx.Recent[i].SenderAgentName
		if _, ok := recentSpeakers[name]; !ok {
			recentSpeakers[name] = (len(ctx.Recent) - 1) - i
		}
	}

	// 计算连续 agent 消息数（没有 user_post 中断）—— 用于链式衰减防止刷屏
	chainDepth := 0
	for i := len(ctx.Recent) - 1; i >= 0; i-- {
		if ctx.Recent[i].MsgType == "user_post" {
			break
		}
		chainDepth++
	}

	// 顶层消息的 reply_to：判断"被怼"
	repliedTargetName := ""
	if ctx.NewMsg != nil && ctx.NewMsg.ReplyToID > 0 {
		if orig, _ := db.GetGroupMessage(ctx.NewMsg.ReplyToID); orig != nil {
			repliedTargetName = orig.SenderAgentName
		}
	}

	out := make([]SpeakDecision, 0, len(localAgents))

	// 是否为自己刚发完一条消息（避免 Agent 自己刷自己）
	lastSenderName := ""
	if ctx.NewMsg != nil {
		lastSenderName = ctx.NewMsg.SenderAgentName
	}

	for _, m := range localAgents {
		p := personalities[m.AgentID]
		if p == nil {
			p = &db.AgentPersonality{
				AgentID:          m.AgentID,
				SpeakProbability: 40,
				MinDelayMs:       2000,
				MaxDelayMs:       5000,
				EmojiFreq:        "medium",
				ColdStartEligible: true,
			}
		}

		// 自己不接自己最新一条消息
		if m.AgentName == lastSenderName {
			continue
		}

		var reasons []string
		score := float64(p.SpeakProbability)
		forced := false

		// 1) @me 强制（无视安静时段）
		if mentionedSet[m.AgentName] {
			forced = true
			reasons = append(reasons, "@mention")
		}

		// 2) 距上次自己发言的间隔 -> 反刷屏
		if dist, ok := recentSpeakers[m.AgentName]; ok {
			if dist == 0 {
				// 上一条就是自己 → 大幅降权（不要连说两条）
				score *= 0.1
				reasons = append(reasons, "just-spoke×0.1")
			} else if dist <= 2 {
				// 最近 2-3 条里说过 → 适度降权
				score *= 0.4
				reasons = append(reasons, "recent-spoke×0.4")
			}
		}

		// 3) 兴趣命中 +30
		if hitInterests(msgContent(ctx.NewMsg), p.Interests) {
			score += 30
			reasons = append(reasons, "interest-hit+30")
		}

		// 4) 冷场救场
		if ctx.IsColdStart {
			if p.ColdStartEligible {
				score += 40
				reasons = append(reasons, "cold-start+40")
			} else {
				score *= 0.3
				reasons = append(reasons, "cold-start-ineligible×0.3")
			}
		}

		// 5) 安静时段（除非 @ 强制）
		if !forced && inQuietHours(ctx.Now, p.QuietStart, p.QuietEnd) {
			score *= 0.1
			reasons = append(reasons, "quiet-hours×0.1")
		}

		// 6) "被怼" - 上一条引用的是自己
		if repliedTargetName != "" && repliedTargetName == m.AgentName {
			score += 50
			reasons = append(reasons, "replied-to-me+50")
		}

		// 7) 用户消息（user_post）总体提升关注度
		if ctx.NewMsg != nil && ctx.NewMsg.MsgType == "user_post" {
			score += 25
			reasons = append(reasons, "user-post+25")
		}

		// 8) 开局破冰：消息较少时大幅提升概率，确保 Agent 活跃
		if len(ctx.Recent) < 3 {
			score += 35
			reasons = append(reasons, "icebreaker+35")
		}

		// 9) Agent-to-Agent 接话加成：另一个 Agent 说完话后，其他 Agent 有概率接上
		if ctx.NewMsg != nil && ctx.NewMsg.MsgType == "message" && ctx.NewMsg.SenderAgentName != "" {
			score += 15
			reasons = append(reasons, "agent-reply+15")
		}

		// 10) 链式衰减：连续 Agent 发言（无用户中断）越多，概率递减，防止无限刷屏
		// depth 0-2: 无衰减; 3-5: ×0.6; 6-8: ×0.3; 9+: ×0.1
		if !forced && chainDepth > 2 {
			switch {
			case chainDepth <= 5:
				score *= 0.6
				reasons = append(reasons, "chain-decay×0.6")
			case chainDepth <= 8:
				score *= 0.3
				reasons = append(reasons, "chain-decay×0.3")
			default:
				score *= 0.1
				reasons = append(reasons, "chain-decay×0.1")
			}
		}

		// 上限 100
		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}

		// 随机摇号
		var rolled bool
		if forced {
			rolled = true
		} else {
			roll := rand.Intn(100)
			rolled = roll < int(score)
			reasons = append(reasons, "roll="+itoa(roll)+"/score="+itoa(int(score)))
		}
		if !rolled {
			continue
		}

		// 计算延迟
		delay := computeDelay(p, forced, score)
		// 兴趣命中再加 200ms 思考
		if hitInterests(msgContent(ctx.NewMsg), p.Interests) {
			delay += 200
		}

		out = append(out, SpeakDecision{
			AgentID:     m.AgentID,
			AgentName:   m.AgentName,
			Probability: int(score),
			DelayMs:     delay,
			Forced:      forced,
			Reasons:     reasons,
		})
	}

	// 按延迟升序
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DelayMs < out[i].DelayMs {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// computeDelay 计算单个 Agent 的发言延迟
// 被 @ 时给一个更小的窗口（500-1500ms），更"秒回"
func computeDelay(p *db.AgentPersonality, forced bool, score float64) int {
	if forced {
		// 500 - 1500ms
		return 500 + rand.Intn(1000)
	}
	min := p.MinDelayMs
	max := p.MaxDelayMs
	if max <= min {
		max = min + 1500
	}
	d := min + rand.Intn(max-min+1)
	// 高分时稍快回（最多减 30%）
	if score >= 70 {
		d = int(float64(d) * 0.7)
	}
	return d
}

// hitInterests 检查消息是否命中 Agent 的兴趣关键词
func hitInterests(text, interestsJSON string) bool {
	if text == "" || interestsJSON == "" || interestsJSON == "[]" {
		return false
	}
	var arr []string
	if err := json.Unmarshal([]byte(interestsJSON), &arr); err != nil {
		return false
	}
	lc := strings.ToLower(text)
	for _, k := range arr {
		k = strings.TrimSpace(strings.ToLower(k))
		if k == "" {
			continue
		}
		if strings.Contains(lc, k) {
			return true
		}
	}
	return false
}

// inQuietHours 判断当前时间是否在 quiet 时段
// 格式 HH:MM；支持跨午夜（例如 23:00 - 07:00）
func inQuietHours(now time.Time, startStr, endStr string) bool {
	if startStr == "" || endStr == "" {
		return false
	}
	start, ok1 := parseHM(startStr)
	end, ok2 := parseHM(endStr)
	if !ok1 || !ok2 {
		return false
	}
	cur := now.Hour()*60 + now.Minute()
	if start == end {
		return false
	}
	if start < end {
		return cur >= start && cur < end
	}
	// 跨午夜
	return cur >= start || cur < end
}

func parseHM(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h := atoi(parts[0])
	m := atoi(parts[1])
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func localJoinedAgents(members []db.GroupMember) []db.GroupMember {
	out := make([]db.GroupMember, 0, len(members))
	for _, m := range members {
		if m.IsLocal && m.Status == "joined" && m.AgentID > 0 {
			out = append(out, m)
		}
	}
	return out
}

func agentIDs(members []db.GroupMember) []int64 {
	out := make([]int64, 0, len(members))
	for _, m := range members {
		out = append(out, m.AgentID)
	}
	return out
}

func msgContent(m *db.GroupMessage) string {
	if m == nil {
		return ""
	}
	return m.Content
}

// parseMentionNames 从文本中抽取 @xxx 名称列表
func parseMentionNames(text string) []string {
	matches := mentionRe.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, mm := range matches {
		if len(mm) > 1 {
			out = append(out, mm[1])
		}
	}
	return out
}
