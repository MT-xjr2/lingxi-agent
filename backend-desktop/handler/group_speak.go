package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"lingxi-agent/config"
	"lingxi-agent/db"
	"lingxi-agent/groupbehavior"
	"lingxi-agent/grouploop"
	"lingxi-agent/nexus"
)

var groupSpeakLocks sync.Map // roomID -> *sync.Map[string]bool

// GroupTurnResult 群聊一轮 LLM 执行结果
type GroupTurnResult struct {
	BlocksJSON string
	PlainText  string
	Skipped    bool
}

// SpeakInGroup 由 grouploop 注入：让某 Agent 在群内发言
func SpeakInGroup(req grouploop.SpeakRequest) {
	room, err := db.GetGroupChat(req.RoomID)
	if err != nil || room == nil || room.Status != "active" {
		return
	}

	members, _ := db.ListGroupMembers(req.RoomID)
	var speaker *db.GroupMember
	for i := range members {
		if members[i].AgentID == req.AgentID && members[i].AgentName == req.AgentName && members[i].IsLocal {
			speaker = &members[i]
			break
		}
	}
	if speaker == nil {
		return
	}

	locks := getGroupSpeakLocks(req.RoomID)
	if _, loaded := locks.LoadOrStore(req.AgentName, true); loaded {
		return
	}
	defer locks.Delete(req.AgentName)

	personality, _ := db.GetPersonality(req.AgentID)
	if !req.Forced && groupbehavior.MaybeEmpty(3) {
		return
	}

	nexus.BroadcastGroupTypingPublic(room, *speaker, 800)

	recent, _ := db.GetRecentGroupMessages(req.RoomID, 50)
	systemPrompt := buildGroupFullSystemPrompt(*speaker, personality, req.AgentID)
	userPrompt := nexus.BuildGroupUserPrompt(room, *speaker, recent)
	forwarder := nexus.BuildGroupStreamForwarder(req.RoomID, *speaker)

	result, err := RunGroupAgentTurn(systemPrompt, userPrompt, req.AgentID, forwarder)
	if err != nil {
		slog.Warn("group turn error", "room", req.RoomID, "agent", req.AgentName, "err", err)
		return
	}
	if result.Skipped || strings.TrimSpace(result.PlainText) == "" {
		return
	}

	plain := strings.TrimSpace(result.PlainText)
	replyToID := int64(0)
	if m := nexus.ParseReplyTag(plain); m > 0 {
		replyToID = m
		plain = strings.TrimSpace(nexus.StripReplyTag(plain))
	}
	if strings.HasPrefix(plain, "[SKIP]") || plain == "" {
		return
	}

	content := result.BlocksJSON
	if content == "" {
		content = plain
	}

	plain = stripBracketTextEmojis(plain)
	if personality != nil {
		plain = groupbehavior.MaybeAddTypo(plain, personality.TypoRate)
		plain = groupbehavior.MaybeEcho(plain, recent, personality.EchoRate)
		if suffix := groupbehavior.EmojiSuffix(personality.EmojiFreq); suffix != "" {
			plain += suffix
		}
		content = patchGroupBlocksText(result.BlocksJSON, plain)
	}

	nexus.PublishGroupAgentMessage(req.RoomID, *speaker, content, plain, replyToID)
}

func getGroupSpeakLocks(roomID int64) *sync.Map {
	v, _ := groupSpeakLocks.LoadOrStore(roomID, &sync.Map{})
	return v.(*sync.Map)
}

func buildGroupFullSystemPrompt(speaker db.GroupMember, p *db.AgentPersonality, agentID int64) string {
	var b strings.Builder
	if agentID > 0 {
		if a, err := db.GetAgent(agentID); err == nil && !a.Builtin && strings.TrimSpace(a.SystemPrompt) != "" {
			b.WriteString(strings.TrimSpace(a.SystemPrompt))
			b.WriteString("\n\n---\n\n")
		}
	}
	b.WriteString(nexus.BuildGroupSystemPrompt(speaker, p))
	return b.String()
}

// RunGroupAgentTurn 群聊专用 LLM 执行：主会话工具链 + 群友 system prompt
func RunGroupAgentTurn(systemPrompt, userMessage string, agentID int64, forwarder nexus.StreamForwarder) (GroupTurnResult, error) {
	cfg := config.Get()

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--dangerously-skip-permissions",
		"--system-prompt", systemPrompt,
	}

	claudeBin := cfg.Claude.Bin
	cmd := exec.Command(claudeBin, args...)
	cmd.Stdin = strings.NewReader(userMessage)
	cmd.Env = buildClaudeEnv(cfg)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return GroupTurnResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return GroupTurnResult{}, fmt.Errorf("cmd start: %w", err)
	}
	slog.Info("group claude started", "agent", agentID, "pid", cmd.Process.Pid)

	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			slog.Info("[group claude stderr]", "text", s.Text())
		}
	}()

	if forwarder != nil {
		forwarder("stream_start", "")
	}

	var (
		textBuf strings.Builder
		blocks  []msgBlock
	)

	appendBlock := func(typ, name, chunk string) {
		if len(blocks) > 0 && typ != "tool" {
			last := &blocks[len(blocks)-1]
			if last.Type == typ {
				last.Text += chunk
				return
			}
		}
		blocks = append(blocks, msgBlock{Type: typ, Name: name, Text: chunk})
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev claudeEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}

		switch ev.Type {
		case "stream_event":
			var inner innerEvent
			if json.Unmarshal(ev.Event, &inner) != nil {
				continue
			}
			switch inner.Type {
			case "content_block_start":
				if inner.ContentBlock.Type == "tool_use" {
					toolName := inner.ContentBlock.Name
					if !isAskUserTool(toolName) {
						payload, _ := json.Marshal(map[string]any{
							"id":    inner.ContentBlock.ID,
							"name":  toolName,
							"label": toolDisplayLabel(toolName),
						})
						if forwarder != nil {
							forwarder("tool_start", string(payload))
						}
					}
					blocks = append(blocks, msgBlock{
						Type:  "tool",
						Name:  toolName,
						Label: toolDisplayLabel(toolName),
						Ms:    time.Now().UnixMilli(),
					})
				} else if inner.ContentBlock.Type == "thinking" {
					// 群聊不展示思考过程
				}
			case "content_block_delta":
				d := inner.Delta
				switch d.Type {
				case "thinking_delta":
					// 丢弃
				case "text_delta":
					if d.Text != "" {
						safeText := redactSensitive(d.Text)
						appendBlock("text", "", safeText)
						textBuf.WriteString(safeText)
						if forwarder != nil {
							forwarder("text", safeText)
						}
					}
				case "input_json_delta":
					if d.PartialJSON != "" && len(blocks) > 0 {
						last := &blocks[len(blocks)-1]
						if last.Type == "tool" {
							last.Input += d.PartialJSON
						}
					}
				default:
					// reasoning 同样不进入群聊
				}
			case "content_block_stop":
				if len(blocks) > 0 {
					last := &blocks[len(blocks)-1]
					if last.Type == "tool" {
						if isAskUserTool(last.Name) {
							blocks = blocks[:len(blocks)-1]
						} else {
							last.Done = true
							elapsed := time.Now().UnixMilli() - last.Ms
							if elapsed < 0 {
								elapsed = 0
							}
							summary := safeSummarizeToolInput(last.Name, last.Input)
							last.Input = summary
							last.Ms = elapsed
							last.Status = "ok"
							endPayload, _ := json.Marshal(map[string]any{
								"done":   true,
								"name":   last.Name,
								"label":  last.Label,
								"input":  summary,
								"ms":     elapsed,
								"status": "ok",
							})
							if forwarder != nil {
								forwarder("tool_end", string(endPayload))
							}
						}
					}
				}
			}
		}
	}

	cmd.Wait()

	if forwarder != nil {
		forwarder("stream_done", "")
	}

	plain := strings.TrimSpace(textBuf.String())
	if plain == "" && len(blocks) == 0 {
		return GroupTurnResult{Skipped: true}, nil
	}

	filtered := make([]msgBlock, 0, len(blocks))
	for i := range blocks {
		if blocks[i].Type == "thinking" {
			continue
		}
		if blocks[i].Type == "tool" {
			blocks[i].Done = true
			blocks[i].Text = ""
		} else {
			blocks[i].Text = redactSensitive(blocks[i].Text)
		}
		filtered = append(filtered, blocks[i])
	}
	blocks = filtered

	blocksJSON := ""
	if bj, err := json.Marshal(blocks); err == nil {
		blocksJSON = string(bj)
	}

	skipped := strings.HasPrefix(strings.TrimSpace(plain), "[SKIP]")
	return GroupTurnResult{
		BlocksJSON: blocksJSON,
		PlainText:  plain,
		Skipped:    skipped,
	}, nil
}

func patchGroupBlocksText(blocksJSON, plain string) string {
	if blocksJSON == "" {
		b, _ := json.Marshal([]msgBlock{{Type: "text", Text: plain}})
		return string(b)
	}
	var blocks []msgBlock
	if json.Unmarshal([]byte(blocksJSON), &blocks) != nil {
		b, _ := json.Marshal([]msgBlock{{Type: "text", Text: plain}})
		return string(b)
	}
	updated := false
	for i := range blocks {
		if blocks[i].Type == "text" {
			blocks[i].Text = plain
			updated = true
			break
		}
	}
	if !updated {
		blocks = append(blocks, msgBlock{Type: "text", Text: plain})
	}
	b, _ := json.Marshal(blocks)
	return string(b)
}

var bracketEmojiRe = regexp.MustCompile(`\[[\p{Han}A-Za-z0-9_+-]{1,12}\]`)

func stripBracketTextEmojis(s string) string {
	return bracketEmojiRe.ReplaceAllString(s, "")
}

// ExtractGroupPlainText 从群消息 content 提取纯文本（支持 blocks JSON）
func ExtractGroupPlainText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if content[0] == '[' {
		var blocks []msgBlock
		if json.Unmarshal([]byte(content), &blocks) == nil {
			var parts []string
			for _, b := range blocks {
				if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
					parts = append(parts, b.Text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		}
	}
	return content
}
