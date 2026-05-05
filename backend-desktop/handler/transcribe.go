package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// TranscribeAudio POST /api/transcribe
// 优先使用本地 whisper.cpp 离线识别，若不可用则回退到远端 OpenAI Whisper API
func TranscribeAudio(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未收到音频文件"})
		return
	}
	defer file.Close()

	audioData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取音频失败"})
		return
	}
	if len(audioData) < 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "音频太短"})
		return
	}

	filename := header.Filename
	if filename == "" {
		filename = "audio.wav"
	}

	// 优先：本地 whisper.cpp
	whisperBin := os.Getenv("WHISPER_BIN")
	whisperModel := os.Getenv("WHISPER_MODEL")
	if whisperBin != "" && whisperModel != "" {
		text, err := transcribeLocal(whisperBin, whisperModel, audioData, filename)
		if err != nil {
			log.Printf("[transcribe] local whisper error: %v, falling back to remote", err)
		} else {
			c.JSON(http.StatusOK, gin.H{"text": text})
			return
		}
	}

	// 回退：远端 Whisper API
	_, _, _, baseURL, token, protocol, _ := activeProfileSnapshot()
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "本地语音引擎不可用，且未配置模型接入点"})
		return
	}

	whisperURL := buildWhisperURL(baseURL, protocol)
	if whisperURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "本地语音引擎不可用，且当前接入点不支持语音识别"})
		return
	}

	text, err := callWhisperAPI(whisperURL, token, audioData, filename)
	if err != nil {
		log.Printf("[transcribe] remote error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"text": text})
}

// transcribeLocal 使用本地 whisper.cpp CLI 进行离线语音识别
func transcribeLocal(whisperBin, modelPath string, audioData []byte, filename string) (string, error) {
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, "lingxi-asr-input.wav")

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".wav"
	}

	// 如果输入是 webm/ogg，需要先转换为 wav（16kHz mono）
	if ext == ".webm" || ext == ".ogg" {
		inputWebm := filepath.Join(tmpDir, "lingxi-asr-input"+ext)
		if err := os.WriteFile(inputWebm, audioData, 0644); err != nil {
			return "", fmt.Errorf("写入临时音频失败: %w", err)
		}
		defer os.Remove(inputWebm)

		// 尝试用 ffmpeg 转换
		ffmpegBin, _ := exec.LookPath("ffmpeg")
		if ffmpegBin == "" {
			// 也检查常用路径
			for _, p := range []string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"} {
				if _, err := os.Stat(p); err == nil {
					ffmpegBin = p
					break
				}
			}
		}
		if ffmpegBin == "" {
			return "", fmt.Errorf("需要 ffmpeg 将 %s 转换为 wav 格式", ext)
		}

		cmd := exec.Command(ffmpegBin, "-y", "-i", inputWebm, "-ar", "16000", "-ac", "1", "-f", "wav", inputPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("ffmpeg 转换失败: %s", string(out))
		}
		defer os.Remove(inputPath)
	} else {
		// wav 或其他格式直接写入
		if err := os.WriteFile(inputPath, audioData, 0644); err != nil {
			return "", fmt.Errorf("写入临时音频失败: %w", err)
		}
		defer os.Remove(inputPath)
	}

	// 调用 whisper.cpp CLI
	cmd := exec.Command(whisperBin,
		"-m", modelPath,
		"-f", inputPath,
		"-l", "zh",
		"--no-timestamps",
		"-nt",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[transcribe] running whisper: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper 执行失败: %v, stderr: %s", err, stderr.String())
	}

	text := strings.TrimSpace(stdout.String())
	// whisper 有时会输出 [BLANK_AUDIO] 之类的标记
	text = strings.ReplaceAll(text, "[BLANK_AUDIO]", "")
	text = strings.TrimSpace(text)

	if text == "" {
		return "", fmt.Errorf("whisper 未识别到有效文本")
	}

	log.Printf("[transcribe] local result: %s", truncateStr(text, 100))
	return text, nil
}

// buildWhisperURL 根据 baseURL 和协议构建 Whisper API 地址
func buildWhisperURL(baseURL, protocol string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return ""
	}

	if protocol == "openai" || protocol == "" {
		if idx := strings.Index(baseURL, "/v1"); idx >= 0 {
			return baseURL[:idx] + "/v1/audio/transcriptions"
		}
		return baseURL + "/v1/audio/transcriptions"
	}

	if idx := strings.Index(baseURL, "/v1"); idx >= 0 {
		return baseURL[:idx] + "/v1/audio/transcriptions"
	}

	return ""
}

// callWhisperAPI 调用 OpenAI 兼容的 /v1/audio/transcriptions
func callWhisperAPI(url, token string, audioData []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("创建表单失败: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("写入音频数据失败: %w", err)
	}

	_ = writer.WriteField("model", "whisper-1")
	_ = writer.WriteField("language", "zh")
	_ = writer.WriteField("response_format", "json")

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API 错误 %d: %s", resp.StatusCode, truncateStr(string(body), 300))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return strings.TrimSpace(string(body)), nil
	}

	return strings.TrimSpace(result.Text), nil
}
