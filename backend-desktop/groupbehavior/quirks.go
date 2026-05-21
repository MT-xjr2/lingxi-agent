package groupbehavior

import (
	"math/rand"
	"strings"

	"lingxi-agent/db"
)

// MaybeAddTypo 以一定概率（百分比 0-100）在文本中加入轻微的"打错别字"效果
// 简单实现：随机交换相邻两个字符
func MaybeAddTypo(text string, rate int) string {
	if rate <= 0 || text == "" {
		return text
	}
	if rand.Intn(100) >= rate {
		return text
	}
	runes := []rune(text)
	if len(runes) < 3 {
		return text
	}
	// 在中间区域随机选一处
	i := 1 + rand.Intn(len(runes)-2)
	if isSeparator(runes[i]) || isSeparator(runes[i+1]) {
		return text
	}
	runes[i], runes[i+1] = runes[i+1], runes[i]
	return string(runes)
}

// MaybeEcho 以一定概率复读最近某条消息的一小段（"+1"或截取片段）
// 返回 echoed 串（追加在最末），如果没触发则原样
func MaybeEcho(text string, recent []db.GroupMessage, rate int) string {
	if rate <= 0 || len(recent) == 0 {
		return text
	}
	if rand.Intn(100) >= rate {
		return text
	}
	// 从近 5 条中随机挑一条非系统消息
	candidates := make([]string, 0, 5)
	upper := len(recent)
	lower := upper - 5
	if lower < 0 {
		lower = 0
	}
	for i := lower; i < upper; i++ {
		c := strings.TrimSpace(recent[i].Content)
		if c == "" || recent[i].MsgType == "system" {
			continue
		}
		if len([]rune(c)) <= 12 {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return text
	}
	pick := candidates[rand.Intn(len(candidates))]
	if strings.Contains(text, pick) {
		return text
	}
	return text + " " + pick + "+1"
}

// MaybeEmpty 以一定概率（按千分比 rate；0.5% 传 5）让 Agent 输出 [SKIP]，模拟"懒得回"
// 返回值为 true 表示要跳过；调用方应丢弃 LLM 回复
func MaybeEmpty(perMille int) bool {
	if perMille <= 0 {
		return false
	}
	return rand.Intn(1000) < perMille
}

// EmojiSuffix 根据频率随机追加一个简单情绪表情
func EmojiSuffix(freq string) string {
	if freq == "" || freq == "low" {
		if rand.Intn(100) < 5 {
			return pickEmoji()
		}
		return ""
	}
	if freq == "high" {
		if rand.Intn(100) < 60 {
			return pickEmoji()
		}
		return ""
	}
	// medium
	if rand.Intn(100) < 25 {
		return pickEmoji()
	}
	return ""
}

var commonEmojis = []string{"😂", "🤣", "😅", "😏", "🤔", "👍", "🙌", "🥲", "🫠", "🥹", "🙏", "👀"}

func pickEmoji() string {
	return " " + commonEmojis[rand.Intn(len(commonEmojis))]
}

func isSeparator(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', ',', '，', '。', '.', '!', '！', '?', '？', ':', '：':
		return true
	}
	return false
}
