package nexus

import (
	"fmt"
	"net"
)

// LANTransport 基于局域网 HTTP 直连的传输层
type LANTransport struct {
	Host string
	Port int
}

func NewLANTransport(host string, port int) *LANTransport {
	return &LANTransport{Host: host, Port: port}
}

func (t *LANTransport) baseURL() string {
	return fmt.Sprintf("http://%s/api/nexus", net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port)))
}

func (t *LANTransport) Send(path string, payload interface{}) ([]byte, error) {
	url := t.baseURL() + path
	return httpPost(url, payload, "")
}

func (t *LANTransport) Get(path string) ([]byte, error) {
	url := t.baseURL() + path
	return httpGet(url)
}

func (t *LANTransport) Type() string {
	return "lan"
}
