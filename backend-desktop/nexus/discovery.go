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

const mdnsScanTimeout = 5 * time.Second

const (
	serviceType  = "_lingxi._tcp"
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
	mu         sync.Mutex
	server     *mdns.Server
	running    bool
	stopCh     chan struct{}
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

	db.CleanStalePeers(time.Now())

	settings, _ := db.GetNexusSettings()
	if settings.Visible {
		d.startServer(settings)
	}

	go d.scanLoop()

	// 如果启用了广域网，连接信令服务器
	if settings.WANEnabled && settings.SignalingURL != "" {
		GetSignalingClient().Start(settings.SignalingURL)
	}

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
	GetSignalingClient().Stop()
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
	d.doScan()

	peers, _ := db.ListNexusPeers()
	if len(peers) == 0 {
		time.Sleep(500 * time.Millisecond)
		d.doScan()
	}

	db.CleanStalePeers(time.Now().Add(-peerTimeout))
}

func (d *Discovery) doScan() {
	entriesCh := make(chan *mdns.ServiceEntry, 32)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
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

			host := entry.AddrV4.String()
			if host == "" || host == "<nil>" {
				if entry.AddrV6 != nil {
					host = entry.AddrV6.String()
				} else {
					host = entry.Host
				}
			}

			agentsJSON := fetchRemoteAgents(host, entry.Port)
			if agentsJSON == "" {
				continue
			}

			db.UpsertNexusPeer(&db.NexusPeer{
				ID:         peerID,
				Nickname:   nickname,
				Host:       host,
				Port:       entry.Port,
				AgentsJSON: agentsJSON,
			})
		}
	}()

	var queryWg sync.WaitGroup
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
				continue
			}
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			queryWg.Add(1)
			go func(iface net.Interface) {
				defer queryWg.Done()
				params := mdns.DefaultParams(serviceType)
				params.Entries = entriesCh
				params.Timeout = mdnsScanTimeout
				params.DisableIPv6 = true
				params.Interface = &iface
				mdns.Query(params)
			}(iface)
		}
	} else {
		queryWg.Add(1)
		go func() {
			defer queryWg.Done()
			params := mdns.DefaultParams(serviceType)
			params.Entries = entriesCh
			params.Timeout = mdnsScanTimeout
			params.DisableIPv6 = true
			mdns.Query(params)
		}()
	}

	queryWg.Wait()
	close(entriesCh)
	wg.Wait()
}

// fetchRemoteAgents 调用对方的 /api/nexus/info 获取公开 Agent 列表
func fetchRemoteAgents(host string, port int) string {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	url := fmt.Sprintf("http://%s/api/nexus/info", addr)

	client := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := client.Dial("tcp", addr)
	if err != nil {
		return ""
	}
	conn.Close()

	resp, err := httpGet(url)
	if err != nil {
		return ""
	}
	var info struct {
		InstanceID string          `json:"instance_id"`
		Agents     json.RawMessage `json:"agents"`
	}
	if json.Unmarshal(resp, &info) != nil || info.InstanceID == "" {
		return ""
	}
	if info.Agents != nil {
		return string(info.Agents)
	}
	return "[]"
}

func getOrCreateInstanceID() string {
	id := db.GetNexusInstanceID()
	if id != "" {
		return id
	}
	hostname, _ := os.Hostname()
	id = fmt.Sprintf("lingxi-%s-%d", hostname, time.Now().UnixNano())
	db.SetNexusInstanceID(id)
	return id
}
