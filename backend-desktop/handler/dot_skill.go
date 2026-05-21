package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	dotSkillName   = "dot-skill"
	dotSkillRepo   = "https://github.com/titanwings/colleague-skill.git"
	dotSkillBranch = "dot-skill"
)

func dotSkillDir() string {
	return filepath.Join(claudeSkillsDir(), dotSkillName)
}

func dotSkillInstalled() bool {
	_, err := os.Stat(filepath.Join(dotSkillDir(), "SKILL.md"))
	return err == nil
}

func dotSkillPythonReady() bool {
	req := filepath.Join(dotSkillDir(), "requirements.txt")
	if _, err := os.Stat(req); err != nil {
		return false
	}
	cmd := exec.Command("python3", "-c", "import yaml")
	cmd.Dir = dotSkillDir()
	return cmd.Run() == nil
}

// InstallDotSkill 克隆 dot-skill 到 claude skills 目录并尝试安装 Python 依赖
func InstallDotSkill() error {
	if dotSkillInstalled() {
		return ensureDotSkillPythonDeps()
	}
	dest := dotSkillDir()
	parent := filepath.Dir(dest)
	os.MkdirAll(parent, 0755)
	os.RemoveAll(dest)

	cmd := exec.Command("git", "clone", "--depth", "1", "-b", dotSkillBranch, dotSkillRepo, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(string(out)))
	}
	slog.Info("dot-skill installed", "path", dest)
	return ensureDotSkillPythonDeps()
}

func ensureDotSkillPythonDeps() error {
	req := filepath.Join(dotSkillDir(), "requirements.txt")
	if _, err := os.Stat(req); err != nil {
		return nil
	}
	cmd := exec.Command("python3", "-m", "pip", "install", "-r", "requirements.txt", "-q")
	cmd.Dir = dotSkillDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("dot-skill pip install", "err", err, "out", strings.TrimSpace(string(out)))
		return fmt.Errorf("Python 依赖安装失败，请在本机执行: cd %s && pip install -r requirements.txt", dotSkillDir())
	}
	return nil
}

func copyDirAll(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// syncBundledDotSkill 从 ai-config/skills/dot-skill 同步到 claude skills（开发/打包内置）
func syncBundledDotSkill() {
	if dotSkillInstalled() {
		return
	}
	candidates := []string{
		filepath.Join("ai-config", "skills", dotSkillName),
		filepath.Join("..", "ai-config", "skills", dotSkillName),
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "ai-config", "skills", dotSkillName),
			filepath.Join(cwd, "..", "ai-config", "skills", dotSkillName),
		)
	}
	for _, src := range candidates {
		if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
			continue
		}
		dest := dotSkillDir()
		os.MkdirAll(filepath.Dir(dest), 0755)
		if err := copyDirAll(src, dest); err == nil {
			slog.Info("synced bundled dot-skill", "from", src)
			_ = ensureDotSkillPythonDeps()
			return
		}
	}
}

func findLatestDistillSlug(family string) (string, error) {
	base := filepath.Join(dotSkillDir(), "skills", family)
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("未找到生成目录 skills/%s（蒸馏可能未完成）", family)
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			if _, err2 := os.Stat(filepath.Join(dir, "persona.md")); err2 != nil {
				continue
			}
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			bestTime = info.ModTime()
			best = e.Name()
		}
	}
	if best == "" {
		return "", fmt.Errorf("skills/%s 下没有有效的蒸馏产物", family)
	}
	return best, nil
}

// slugifyAlias 为每位蒸馏对象生成独立目录名（中文花名用 hash，避免都落到 character）
func slugifyAlias(alias string) string {
	trimmed := strings.TrimSpace(alias)
	s := strings.ToLower(trimmed)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		sum := md5.Sum([]byte(trimmed))
		out = "p-" + hex.EncodeToString(sum[:])[:12]
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func distillSkillDir(family, slug string) string {
	return filepath.Join(dotSkillDir(), "skills", family, slug)
}

func distillArtifactPaths(family, slug string) []string {
	dirs := []string{
		distillSkillDir(family, slug),
	}
	if family == "colleague" {
		dirs = append(dirs, filepath.Join(dotSkillDir(), "colleagues", slug))
	}
	return dirs
}

func hasDistillArtifacts(family, slug string) bool {
	for _, dir := range distillArtifactPaths(family, slug) {
		if st, err := os.Stat(filepath.Join(dir, "persona.md")); err == nil && st.Size() > 0 {
			return true
		}
		if st, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil && st.Size() > 0 {
			return true
		}
	}
	return false
}

func resolveDistillSlug(family, alias, transcript string, since time.Time) (string, error) {
	expected := slugifyAlias(alias)
	if m := distillSlugFromTranscript(transcript); m != "" {
		expected = m
	}
	if hasDistillArtifacts(family, expected) {
		return expected, nil
	}
	// 仅在本轮蒸馏时间窗内、且 slug 与预期一致时采纳（避免并行蒸馏串人）
	if alt := findDistillSlugSince(family, expected, since); alt != "" {
		return alt, nil
	}
	if err := materializeDistillOutput(family, alias, expected, transcript); err != nil {
		return "", fmt.Errorf(
			"未找到蒸馏产物 skills/%s/%s（Claude 未写入目录；自动落盘失败: %v）",
			family, expected, err,
		)
	}
	if hasDistillArtifacts(family, expected) {
		return expected, nil
	}
	return "", fmt.Errorf("未找到蒸馏产物 skills/%s/%s（请确认 Claude 已写入该目录）", family, expected)
}

func findDistillSlugSince(family, expected string, since time.Time) string {
	base := filepath.Join(dotSkillDir(), "skills", family)
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	var matched []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() != expected {
			continue
		}
		dir := filepath.Join(base, e.Name())
		info, err := os.Stat(dir)
		if err != nil || info.ModTime().Before(since.Add(-2*time.Minute)) {
			continue
		}
		if hasDistillArtifacts(family, e.Name()) {
			matched = append(matched, e.Name())
		}
	}
	if len(matched) == 1 {
		return matched[0]
	}
	return ""
}

func materializeDistillOutput(family, alias, slug, transcript string) error {
	if strings.TrimSpace(transcript) == "" {
		return fmt.Errorf("蒸馏输出为空")
	}
	persona, work := extractDistillMarkdown(transcript, alias)
	if strings.TrimSpace(persona) == "" {
		persona = buildFallbackDistillPersona(alias, transcript)
	}
	if strings.TrimSpace(work) == "" {
		work = buildFallbackDistillWork(family)
	}
	tmpDir := filepath.Join(dotSkillDir(), "tmp", "materialize-"+slug)
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	personaPath := filepath.Join(tmpDir, "persona.md")
	workPath := filepath.Join(tmpDir, "work.md")
	if err := os.WriteFile(personaPath, []byte(persona), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(workPath, []byte(work), 0644); err != nil {
		return err
	}
	baseDir := filepath.Join(dotSkillDir(), "skills", family)
	cmd := exec.Command(
		"python3", "tools/skill_writer.py",
		"--action", "create",
		"--slug", slug,
		"--name", alias,
		"--character", family,
		"--base-dir", baseDir,
		"--persona", personaPath,
		"--work", workPath,
		"--no-install-claude-skill",
	)
	cmd.Dir = dotSkillDir()
	cmd.Env = append(os.Environ(), "DOT_SKILL_AUTO_INSTALL_CLAUDE=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	slog.Info("distill materialized via skill_writer", "slug", slug, "family", family)
	return nil
}

func extractDistillMarkdown(transcript, alias string) (persona, work string) {
	blocks := extractFencedMarkdownBlocks(transcript)
	bestPersona, bestWork := "", ""
	bestPScore, bestWScore := 0, 0
	for _, block := range blocks {
		lower := strings.ToLower(block)
		ps := distillBlockScore(lower, true)
		ws := distillBlockScore(lower, false)
		if ps > bestPScore {
			bestPScore = ps
			bestPersona = block
		}
		if ws > bestWScore {
			bestWScore = ws
			bestWork = block
		}
	}
	if bestPersona != "" && bestWork != "" && bestPersona == bestWork && bestWScore > 0 {
		// 单块同时含 persona+work 时尝试拆分
		if i := strings.Index(bestPersona, "## PART A"); i > 0 {
			work = strings.TrimSpace(bestPersona[i:])
			bestPersona = strings.TrimSpace(bestPersona[:i])
		}
	}
	if work == "" {
		work = bestWork
	}
	return bestPersona, work
}

func distillBlockScore(lower string, persona bool) int {
	score := 0
	if persona {
		for _, kw := range []string{"layer 0", "part b", "人物性格", "persona", "性格", "口吻", "禁忌"} {
			if strings.Contains(lower, kw) {
				score += 5
			}
		}
	} else {
		for _, kw := range []string{"part a", "工作能力", "work.md", "技术栈", "工作方法"} {
			if strings.Contains(lower, kw) {
				score += 5
			}
		}
	}
	if len(lower) > 200 {
		score += 2
	}
	return score
}

func extractFencedMarkdownBlocks(text string) []string {
	var blocks []string
	lines := strings.Split(text, "\n")
	inFence := false
	var cur strings.Builder
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			if inFence {
				b := strings.TrimSpace(cur.String())
				if len(b) > 80 {
					blocks = append(blocks, b)
				}
				cur.Reset()
				inFence = false
			} else {
				inFence = true
			}
			continue
		}
		if inFence {
			cur.WriteString(line)
			cur.WriteByte('\n')
		}
	}
	return blocks
}

func buildFallbackDistillPersona(alias, transcript string) string {
	body := strings.TrimSpace(transcript)
	if len(body) > 12000 {
		body = body[:12000] + "\n\n...(truncated)"
	}
	return fmt.Sprintf(`# %s — 人物性格（自动整理）

> 说明：Claude 未完成 skill_writer 落盘，以下内容由蒸馏对话自动提取，建议核对后重新蒸馏。

%s
`, alias, body)
}

func buildFallbackDistillWork(family string) string {
	if family != "colleague" {
		return "# 背景与公开形象\n\n（自动整理，待补充）\n"
	}
	return `# 工作画像

> 说明：本次由对话自动整理；若缺失具体技能栈，请补充材料后重新蒸馏。

## 概述

待根据聊天记录补充工作职责、技术栈与协作风格。
`
}

func distillSlugFromTranscript(transcript string) string {
	for _, line := range strings.Split(transcript, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "DISTILL_DONE") {
			if i := strings.Index(line, "slug="); i >= 0 {
				s := strings.TrimSpace(line[i+5:])
				s = strings.Fields(s)[0]
				s = strings.Trim(s, `"'.,;`)
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}
