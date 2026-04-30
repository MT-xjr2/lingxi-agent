package config

import (
	"os"
	"sync"
)

type Config struct {
	Server struct {
		Port         string
		FrontendDist string
	}
	DB struct {
		Path string
	}
	Claude struct {
		Model   string
		Bin     string // claude CLI 可执行文件路径（内置或系统）
		AuthToken  string // ANTHROPIC_AUTH_TOKEN
		BaseURL    string // ANTHROPIC_BASE_URL
		ModelEnv   string // ANTHROPIC_MODEL（覆盖 settings.json）
	}
}

var (
	cfg  Config
	once sync.Once
)

func Get() *Config {
	once.Do(func() {
		cfg.Server.Port = "3001"
		cfg.Server.FrontendDist = "../frontend-desktop/dist"
		cfg.DB.Path = "smart-agent.db"
		cfg.Claude.Model = "claude-opus-4-5"
		cfg.Claude.Bin = "claude"
		cfg.Claude.AuthToken = ""
		cfg.Claude.BaseURL = ""
		cfg.Claude.ModelEnv = ""

		override(&cfg.Server.Port, "PORT")
		override(&cfg.Server.FrontendDist, "FRONTEND_DIST")
		override(&cfg.DB.Path, "DB_PATH")
		override(&cfg.Claude.Model, "CLAUDE_MODEL")
		override(&cfg.Claude.Bin, "CLAUDE_BIN")
		override(&cfg.Claude.AuthToken, "ANTHROPIC_AUTH_TOKEN")
		override(&cfg.Claude.BaseURL, "ANTHROPIC_BASE_URL")
		override(&cfg.Claude.ModelEnv, "ANTHROPIC_MODEL")
	})
	return &cfg
}

func override(field *string, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		*field = v
	}
}
