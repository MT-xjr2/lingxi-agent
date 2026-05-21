package handler

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"time"

	"lingxi-agent/config"
	"lingxi-agent/db"
)

var validDistillFamilies = map[string]bool{
	"colleague":    true,
	"relationship": true,
	"celebrity":    true,
}

// GetDistillStatus GET /api/agents/distill/status
func GetDistillStatus(c *gin.Context) {
	syncBundledDotSkill()
	c.JSON(http.StatusOK, gin.H{
		"dot_skill_installed": dotSkillInstalled(),
		"python_ready":        dotSkillPythonReady(),
		"dot_skill_path":      dotSkillDir(),
	})
}

// InstallDotSkillHandler POST /api/skills/install-github
func InstallDotSkillHandler(c *gin.Context) {
	if err := InstallDotSkill(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":                  true,
		"dot_skill_installed": true,
		"path":                dotSkillDir(),
	})
}

// DistillAgentStream POST /api/agents/distill/stream (multipart SSE)
func DistillAgentStream(c *gin.Context) {
	family := strings.TrimSpace(c.PostForm("family"))
	if !validDistillFamilies[family] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "family 须为 colleague / relationship / celebrity"})
		return
	}
	alias := strings.TrimSpace(c.PostForm("alias"))
	if alias == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写花名/代号"})
		return
	}
	profile := strings.TrimSpace(c.PostForm("profile"))
	personality := strings.TrimSpace(c.PostForm("personality"))
	researchProfile := strings.TrimSpace(c.PostForm("research_profile"))
	if researchProfile == "" {
		researchProfile = "budget-friendly"
	}
	recordID, _ := strconv.ParseInt(strings.TrimSpace(c.PostForm("record_id")), 10, 64)

	syncBundledDotSkill()
	if !dotSkillInstalled() {
		if err := InstallDotSkill(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dot-skill 未安装: " + err.Error()})
			return
		}
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart 解析失败"})
		return
	}
	files := form.File["files"]

	distillID := uuid.New().String()[:8]
	rawDir := filepath.Join(dotSkillDir(), "tmp", "distill-"+distillID, "raw")
	os.MkdirAll(rawDir, 0755)
	defer os.RemoveAll(filepath.Join(dotSkillDir(), "tmp", "distill-"+distillID))

	manifest := []map[string]string{}
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(f)
		f.Close()
		if len(data) == 0 {
			continue
		}
		safeName := filepath.Base(fh.Filename)
		dest := filepath.Join(rawDir, safeName)
		os.WriteFile(dest, data, 0644)
		entry := map[string]string{"file": safeName, "path": dest}
		if summary, err := distillExtractText(safeName, data); err == nil && summary != "" {
			summary = annotateDistillChatMaterial(summary, alias)
			summaryPath := dest + ".extracted.txt"
			os.WriteFile(summaryPath, []byte(summary), 0644)
			entry["extracted"] = summaryPath
			if len(summary) > 8000 {
				summary = summary[:8000] + "\n...(truncated)"
			}
			entry["summary"] = summary
		}
		manifest = append(manifest, entry)
	}
	expectedSlug := slugifyAlias(alias)
	manifestBytes, _ := json.MarshalIndent(map[string]interface{}{
		"family":           family,
		"alias":            alias,
		"slug":             expectedSlug,
		"profile":          profile,
		"personality":      personality,
		"research_profile": researchProfile,
		"files":            manifest,
	}, "", "  ")
	os.WriteFile(filepath.Join(rawDir, "manifest.json"), manifestBytes, 0644)

	task := buildDistillTaskPrompt(family, alias, profile, personality, researchProfile, string(manifestBytes), rawDir)
	distillStarted := time.Now()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--allowedTools", "Bash,Read,Write,Edit"}
	claudeBin := config.Get().Claude.Bin
	execCmd := exec.Command(claudeBin, args...)
	execCmd.Stdin = strings.NewReader(task)
	execCmd.Dir = dotSkillDir()
	execCmd.Env = buildClaudeEnv(config.Get())

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		writeSSE(c, "error", jsonStr("启动蒸馏失败"))
		writeSSE(c, "done", "[DONE]")
		return
	}
	stderrPipe, _ := execCmd.StderrPipe()
	if err := execCmd.Start(); err != nil {
		writeSSE(c, "error", jsonStr("启动蒸馏失败: "+err.Error()))
		writeSSE(c, "done", "[DONE]")
		return
	}

	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			slog.Info("[distill stderr]", "line", s.Text())
		}
	}()

	var transcript strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev claudeEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == "stream_event" {
			var inner innerEvent
			if err := json.Unmarshal(ev.Event, &inner); err != nil {
				continue
			}
			switch inner.Type {
			case "content_block_delta":
				d := inner.Delta
				if d.Type == "text_delta" && d.Text != "" {
					transcript.WriteString(d.Text)
					writeSSE(c, "text", jsonStr(d.Text))
				} else if d.Type == "thinking_delta" && d.Thinking != "" {
					writeSSE(c, "thinking", jsonStr(d.Thinking))
				} else if d.Type == "input_json_delta" && d.PartialJSON != "" {
					writeSSE(c, "tool_input", jsonStr(d.PartialJSON))
				}
			case "content_block_start":
				if inner.ContentBlock.Type == "tool_use" {
					payload, _ := json.Marshal(map[string]string{"name": inner.ContentBlock.Name})
					writeSSE(c, "tool_start", string(payload))
				}
			case "content_block_stop":
				writeSSE(c, "tool_end", "{}")
			}
		}
	}
	if waitErr := execCmd.Wait(); waitErr != nil {
		slog.Warn("distill claude exit", "err", waitErr)
	}

	slug, err := resolveDistillSlug(family, alias, transcript.String(), distillStarted)
	if err != nil {
		writeSSE(c, "error", jsonStr(err.Error()))
		writeSSE(c, "done", "[DONE]")
		return
	}

	result, err := parseDistillSkillOutput(family, slug, alias, profile, personality)
	if err != nil {
		writeSSE(c, "error", jsonStr(err.Error()))
		writeSSE(c, "done", "[DONE]")
		return
	}
	result["family"] = family
	result["slug"] = slug

	savedID, saveErr := persistDistillRecord(persistDistillInput{
		RecordID:        recordID,
		Family:          family,
		Alias:           alias,
		Slug:            slug,
		Profile:         profile,
		PersonalityHint: personality,
		Result:          result,
		RawDir:          rawDir,
	})
	if saveErr != nil {
		slog.Warn("persist distill record", "err", saveErr)
	} else {
		result["record_id"] = savedID
	}

	payload, _ := json.Marshal(result)
	writeSSE(c, "preview", string(payload))
	writeSSE(c, "done", "[DONE]")
	c.Writer.Flush()
}

// ApplyDistillResult POST /api/agents/distill/apply
func ApplyDistillResult(c *gin.Context) {
	var body struct {
		RecordID     int64  `json:"record_id"`
		Family       string `json:"family"`
		Slug         string `json:"slug"`
		Alias        string `json:"alias"`
		Profile      string `json:"profile"`
		Personality  string `json:"personality"`
		InstallSkill bool   `json:"install_skill"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求"})
		return
	}
	if body.RecordID > 0 {
		rec, err := db.GetDistillRecord(body.RecordID)
		if err != nil || rec == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "蒸馏记录不存在"})
			return
		}
		result := distillRecordToApplyMap(rec)
		if body.InstallSkill {
			skillName := rec.Slug
			if rec.Family != "colleague" {
				skillName = rec.Family + "-" + rec.Slug
			}
			outDir := filepath.Join(rec.StorageDir, "output")
			if err := installDistillSkillDir(skillName, outDir); err != nil {
				result["skill_install_error"] = err.Error()
			} else {
				result["skill_name"] = skillName
				result["skill_installed"] = true
			}
		}
		c.JSON(http.StatusOK, result)
		return
	}
	if body.Family == "" || body.Slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要 family 与 slug，或 record_id"})
		return
	}
	if !validDistillFamilies[body.Family] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 family"})
		return
	}
	if !dotSkillInstalled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dot-skill 未安装"})
		return
	}

	result, err := parseDistillSkillOutput(body.Family, body.Slug, body.Alias, body.Profile, body.Personality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if body.InstallSkill {
		skillName := body.Slug
		if body.Family != "colleague" {
			skillName = body.Family + "-" + body.Slug
		}
		srcDir := filepath.Join(dotSkillDir(), "skills", body.Family, body.Slug)
		if err := installDistillSkillDir(skillName, srcDir); err != nil {
			result["skill_install_error"] = err.Error()
		} else {
			result["skill_name"] = skillName
			result["skill_installed"] = true
		}
	}

	c.JSON(http.StatusOK, result)
}

func buildDistillAttributionRules(alias string) string {
	target := alias
	if target == "" {
		target = "蒸馏对象"
	}
	return fmt.Sprintf(`【关键：说话人归因 — 必须遵守】
本次蒸馏对象（仅此人的性格、背景、经历）: %s
聊天原材料中：
- 标注为【我】或未标注但明显是「上传者本人」在自我介绍的内容（如本人籍贯、孩子、通勤、住址），禁止写入 %s 的 persona/work，只能作对话语境。
- 仅将【%s】或对方昵称发送的消息，以及明确描述 %s 的第三人称信息，提炼为 TA 的属性。
- 群聊中其他人的话不要当成 %s 说的；不要把他人对 %s 的评价当成 %s 自述。
- 若无法判断是谁说的，该条事实不得写入 persona，须在输出中标注「说话人不明」。
分析前请先 Read prompts/persona_analyzer.md，并遵守其中「聊天记录归因」章节。

`, target, target, target, target, target, target, target)
}

// annotateDistillChatMaterial 为聊天记录类原材料添加蒸馏归因说明
func annotateDistillChatMaterial(text, targetAlias string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	lower := strings.ToLower(text)
	looksChat := strings.Contains(text, "# Chat:") ||
		strings.Contains(text, "【我】") ||
		strings.Contains(text, "]: ") ||
		strings.Contains(lower, "wxid_") ||
		strings.Contains(text, "@chatroom")
	if !looksChat {
		return text
	}
	target := strings.TrimSpace(targetAlias)
	if target == "" {
		target = "对方"
	}
	header := fmt.Sprintf(`# 蒸馏归因说明（系统注入，必须遵守）
# 蒸馏对象: %s（仅提炼此人的性格与事实）
# 【我】= 上传材料的用户本人，其自述的籍贯/家庭/通勤/子女等不得写入 %s
# 无【我】/【对方】前缀的旧版导出：结合上下文判断，宁可漏提不可错归因

`, target, target)
	if strings.Contains(text, "蒸馏归因说明") {
		return text
	}
	return header + text
}

func buildDistillTaskPrompt(family, alias, profile, personality, researchProfile, manifestJSON, rawDir string) string {
	slugHint := slugifyAlias(alias)
	var b strings.Builder
	b.WriteString("你是 dot-skill 蒸馏执行器。必须在当前目录（dot-skill 根目录，含 SKILL.md）执行。\n")
	b.WriteString("严格遵循 SKILL.md 的「主流程：创建新 Skill」，不要跳过步骤。\n\n")
	b.WriteString(fmt.Sprintf("- character family: %s\n", family))
	b.WriteString(fmt.Sprintf("- 花名/代号: %s\n", alias))
	b.WriteString(fmt.Sprintf("- 建议 slug: %s\n", slugHint))
	if profile != "" {
		b.WriteString(fmt.Sprintf("- 基本信息: %s\n", profile))
	}
	if personality != "" {
		b.WriteString(fmt.Sprintf("- 性格画像: %s\n", personality))
	}
	if family == "celebrity" {
		b.WriteString(fmt.Sprintf("- research_profile: %s\n", researchProfile))
	}
	b.WriteString(fmt.Sprintf("\n用户已上传原材料目录: %s\n", rawDir))
	b.WriteString("manifest.json 已生成，请先 Read manifest.json 与各 raw 文件（及 .extracted.txt 若有）。\n")
	b.WriteString("对 JSON/邮件等可用 python3 tools/ 下对应解析脚本。\n\n")
	b.WriteString(buildDistillAttributionRules(alias))
	b.WriteString("\n原材料清单:\n")
	b.WriteString(manifestJSON)
	b.WriteString("\n\n完成后必须在 ./skills/")
	b.WriteString(family)
	b.WriteString("/")
	b.WriteString(slugHint)
	b.WriteString("/ 下生成完整 Skill（含 SKILL.md、persona.md")
	if family == "colleague" {
		b.WriteString("、work.md")
	}
	b.WriteString("）。\n")
	b.WriteString(fmt.Sprintf("【重要】本次必须且只能写入目录 ./skills/%s/%s/ ，禁止覆盖其他人物目录（勿用 character 等通用名）。\n", family, slugHint))
	b.WriteString("\n【落盘 — 必须执行，否则视为失败】\n")
	b.WriteString("分析完成后，用 Write 生成 persona.md / work.md，再执行：\n")
	b.WriteString(fmt.Sprintf(
		"python3 tools/skill_writer.py --action create --slug %s --name %q --character %s --base-dir ./skills/%s --persona <persona路径> --work <work路径> --no-install-claude-skill\n",
		slugHint, alias, family, family,
	))
	b.WriteString("确认 skills/")
	b.WriteString(family)
	b.WriteString("/")
	b.WriteString(slugHint)
	b.WriteString("/persona.md 已存在后，最后一行回复: DISTILL_DONE slug=")
	b.WriteString(slugHint)
	return b.String()
}

func distillExtractText(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".txt", ".csv", ".json":
		return string(data), nil
	case ".pdf":
		return extractTextFromPDF(data)
	case ".docx":
		return extractTextFromDocx(data)
	case ".db", ".sqlite", ".sqlite3":
		return extractWeChatSQLite(data)
	case ".eml":
		return string(data), nil
	default:
		if len(data) < 512*1024 {
			return string(data), nil
		}
		return "", fmt.Errorf("unsupported")
	}
}

func extractWeChatSQLite(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "wx-*.db")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		return "", err
	}
	tmp.Close()

	db, err := sql.Open("sqlite", tmp.Name())
	if err != nil {
		return "", err
	}
	defer db.Close()

	queries := []string{
		`SELECT content FROM message WHERE content IS NOT NULL AND content != '' ORDER BY createTime DESC LIMIT 3000`,
		`SELECT StrContent FROM MSG WHERE StrContent IS NOT NULL ORDER BY localId DESC LIMIT 3000`,
		`SELECT content FROM Message WHERE content IS NOT NULL LIMIT 3000`,
	}
	for _, q := range queries {
		rows, err := db.Query(q)
		if err != nil {
			continue
		}
		var lines []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				continue
			}
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		rows.Close()
		if len(lines) > 0 {
			if len(lines) > 5000 {
				lines = lines[:5000]
			}
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", fmt.Errorf("无法解析微信数据库表结构")
}

func parseDistillSkillOutput(family, slug, alias, profile, personality string) (map[string]interface{}, error) {
	if !hasDistillArtifacts(family, slug) {
		return nil, fmt.Errorf("未找到蒸馏产物: skills/%s/%s", family, slug)
	}
	dir := distillSkillDir(family, slug)
	personaPath := filepath.Join(dir, "persona.md")
	personaBytes, err := os.ReadFile(personaPath)
	if err != nil {
		for _, d := range distillArtifactPaths(family, slug) {
			personaPath = filepath.Join(d, "persona.md")
			personaBytes, err = os.ReadFile(personaPath)
			if err == nil {
				dir = d
				break
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("未找到 persona.md: %w", err)
	}
	persona := string(personaBytes)

	systemPrompt := persona
	workPath := filepath.Join(dir, "work.md")
	if workBytes, err := os.ReadFile(workPath); err == nil && len(workBytes) > 0 {
		work := string(workBytes)
		if len(work) > 12000 {
			work = work[:12000] + "\n...(truncated)"
		}
		systemPrompt += "\n\n## 工作能力\n\n" + work
	}

	name := alias
	skillMD, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if desc := parseSkillMDDescription(string(skillMD)); desc != "" && profile == "" {
		profile = desc
	}
	if name == "" {
		name = slug
	}

	pers := map[string]interface{}{
		"tags":         []string{},
		"interests":    []string{},
		"style_hint":   extractStyleHint(persona, personality),
		"speak_probability": 70,
		"min_delay_ms": 800,
		"max_delay_ms": 4000,
	}
	if personality != "" {
		for _, t := range strings.FieldsFunc(personality, func(r rune) bool {
			return r == ' ' || r == ',' || r == '，' || r == '、'
		}) {
			t = strings.TrimSpace(t)
			if t != "" {
				pers["tags"] = append(pers["tags"].([]string), t)
			}
		}
	}

	return map[string]interface{}{
		"name":          name,
		"description":   profile,
		"system_prompt": systemPrompt,
		"avatar":        "✦",
		"personality":   pers,
		"skill_dir":     dir,
	}, nil
}

func extractStyleHint(persona, personality string) string {
	if personality != "" {
		return personality
	}
	lines := strings.Split(persona, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 20 && len(line) < 200 {
			return line
		}
	}
	if len(persona) > 300 {
		return persona[:300]
	}
	return persona
}

func installDistillSkillDir(skillName, srcDir string) error {
	dest := filepath.Join(claudeSkillsDir(), skillName)
	os.RemoveAll(dest)
	if err := copyDirAll(srcDir, dest); err != nil {
		return err
	}
	slug := filepath.Base(srcDir)
	zipData, err := zipDir(filepath.Dir(srcDir), slug)
	if err != nil {
		return err
	}
	storageDir := skillsStorageDir()
	filePath := filepath.Join(storageDir, skillName+".zip")
	if err := os.WriteFile(filePath, zipData, 0644); err != nil {
		return err
	}
	skillMD, _ := os.ReadFile(filepath.Join(srcDir, "SKILL.md"))
	desc := parseSkillMDDescription(string(skillMD))
	_, err = db.DB.Exec(`
		INSERT INTO skills (name, description, file_path, installed)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(name) DO UPDATE SET
			description=excluded.description,
			file_path=excluded.file_path,
			installed=1,
			updated_at=CURRENT_TIMESTAMP
	`, skillName, desc, filePath)
	invalidateSkillsCache()
	return err
}
