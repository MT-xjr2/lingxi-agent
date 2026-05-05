package nexus

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"lingxi-agent/db"
)

const (
	serviceType = "_lingxi._tcp"
	scanInterval = 10 * time.Second
	peerTimeout  = 60 * time.Second
)

// PublicAgent 用于 mDNS TXT 记录 和 /api/nexus/info 返回
type PublicAgent struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	CapabilityTags []string `json:"capability_tags"`
	AuthLevel      string   `json:"auth_level"`
}

type Discovery struct {
	mu       sync.Mutex
	server   *mdns.Server
	running  bool
	stopCh   chan struct{}
	instanceID string
}

var Global = &Discovery{}

func (d *Discovery) InstanceID() string {
	return d.instanceID
}

// Start 启动 mDNS 服务注册和定时扫描
func (d *Discovery) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return
	}

	d.instanceID = getOrCreateInstanceID()
	d.stopCh = make(chan struct{})
	d.running = true

	settings, _ := db.GetNexusSettings()
	if settings.Visible {
		d.startServer(settings)
	}

	go d.scanLoop()
	log.Printf("[nexus] discovery started, instanceID=%s", d.instanceID)
}

// Stop 停止 mDNS 服务
func (d *Discovery) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	close(d.stopCh)
	d.running = false
	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
	}
	log.Println("[nexus] discovery stopped")
}

// Restart 根据新设置重新注册 mDNS
func (d *Discovery) Restart() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
	}

	settings, _ := db.GetNexusSettings()
	if settings.Visible && d.running {
		d.startServer(settings)
	}
}

func (d *Discovery) startServer(settings *db.NexusSettings) {
	nickname := settings.Nickname
	if nickname == "" {
		hostname, _ := os.Hostname()
		nickname = hostname
	}

	info := []string{
		"id=" + d.instanceID,
		"nick=" + nickname,
	}

	service, err := mdns.NewMDNSService(
		d.instanceID,
		serviceType,
		"",
		"",
		settings.ListenPort,
		nil,
		info,
	)
	if err != nil {
		log.Printf("[nexus] mdns service create error: %v", err)
		return
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		log.Printf("[nexus] mdns server start error: %v", err)
		return
	}
	d.server = server
	log.Printf("[nexus] mdns server broadcasting on port %d", settings.ListenPort)
}

func (d *Discovery) scanLoop() {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	d.scan()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.scan()
		}
	}
}

func (d *Discovery) scan() {
	entriesCh := make(chan *mdns.ServiceEntry, 16)
	found := make(map[string]bool)

	go func() {
		for entry := range entriesCh {
			peerID := ""
			nickname := ""
			for _, field := range entry.InfoFields {
				if len(field) > 3 && field[:3] == "id=" {
					peerID = field[3:]
				}
				if len(field) > 5 && field[:5] == "nick=" {
					nickname = field[5:]
				}
			}
			if peerID == "" || peerID == d.instanceID {
				continue
			}
			found[peerID] = true

			host := entry.AddrV4.String()
			if host == "" || host == "<nil>" {
				if entry.AddrV6 != nil {
					host = entry.AddrV6.String()
				} else {
					host = entry.Host
				}
			}

			agentsJSON := fetchRemoteAgents(host, entry.Port)

			db.UpsertNexusPeer(&db.NexusPeer{
				ID:         peerID,
				Nickname:   nickname,
				Host:       host,
				Port:       entry.Port,
				AgentsJSON: agentsJSON,
			})
		}
	}()

	params := mdns.DefaultParams(serviceType)
	params.Entries = entriesCh
	params.Timeout = 3 * time.Second
	params.DisableIPv6 = true

	mdns.Query(params)
	close(entriesCh)

	db.CleanStalePeers(time.Now().Add(-peerTimeout))
}

// fetchRemoteAgents 调用对方的 /api/nexus/info 获取公开 Agent 列表
func fetchRemoteAgents(host string, port int) string {
	url := fmt.Sprintf("http://%s/api/nexus/info", net.JoinHostPort(host, fmt.Sprintf("%d", port)))

	client := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := client.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return "[]"
	}
	conn.Close()

	resp, err := httpGet(url)
	if err != nil {
		return "[]"
	}
	var info struct {
		Agents json.RawMessage `json:"agents"`
	}
	if json.Unmarshal(resp, &info) == nil && info.Agents != nil {
		return string(info.Agents)
	}
	return "[]"
}

func getOrCreateInstanceID() string {
	cfg, _ := db.GetNexusSettings()
	_ = cfg

	hostname, _ := os.Hostname()
	pid := os.Getpid()
	return fmt.Sprintf("lingxi-%s-%d", hostname, pid)
}
