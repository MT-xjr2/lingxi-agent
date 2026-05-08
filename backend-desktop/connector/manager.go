package connector

import (
	"context"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
)

// Manager 管理所有 IM 连接器的生命周期
type Manager struct {
	mu         sync.Mutex
	running    map[string]context.CancelFunc // platform -> cancel
	ginRouter  gin.IRouter
}

var GlobalManager *Manager

func InitManager(router gin.IRouter) {
	GlobalManager = &Manager{
		running:   make(map[string]context.CancelFunc),
		ginRouter: router,
	}
}

// LoadFromDB 从数据库读取所有已启用的连接器并启动
func (m *Manager) LoadFromDB() {
	connectors, err := db.ListIMConnectors()
	if err != nil {
		log.Printf("[connector manager] LoadFromDB error: %v", err)
		return
	}
	for _, c := range connectors {
		if c.Enabled {
			if err := m.Start(c.Platform, c.Config); err != nil {
				log.Printf("[connector manager] start %s error: %v", c.Platform, err)
			}
		}
	}
}

// Start 启动指定平台的连接器（已在运行则先停止再重启）
func (m *Manager) Start(platform, configJSON string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先停止已有的
	if cancel, ok := m.running[platform]; ok {
		cancel()
		delete(m.running, platform)
	}

	conn, err := m.buildConnector(platform, configJSON)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.running[platform] = cancel

	go func() {
		log.Printf("[connector manager] starting %s", platform)
		if err := conn.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[connector manager] %s exited with error: %v", platform, err)
		}
		m.mu.Lock()
		delete(m.running, platform)
		m.mu.Unlock()
	}()

	return nil
}

// Stop 停止指定平台的连接器
func (m *Manager) Stop(platform string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.running[platform]; ok {
		cancel()
		delete(m.running, platform)
		log.Printf("[connector manager] stopped %s", platform)
	}
}

// IsRunning 返回指定平台是否正在运行
func (m *Manager) IsRunning(platform string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[platform]
	return ok
}

// buildConnector 根据平台名和配置 JSON 构建对应连接器
func (m *Manager) buildConnector(platform, configJSON string) (Connector, error) {
	switch platform {
	case "dingtalk":
		return NewDingtalkConnector(configJSON)
	case "feishu":
		return NewFeishuConnector(configJSON)
	case "wecom":
		return NewWecomConnector(configJSON, m.ginRouter)
	default:
		return nil, nil
	}
}
