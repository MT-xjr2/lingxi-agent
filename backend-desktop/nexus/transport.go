package nexus

// Transport 定义了 Nexus 节点间通信的传输层接口
type Transport interface {
	// Send 发送 JSON 消息到指定端点（path 为相对路径如 "/conversation/message"）
	Send(path string, payload interface{}) ([]byte, error)
	// Get 执行 GET 请求
	Get(path string) ([]byte, error)
	// Type 返回传输类型标识
	Type() string
}
