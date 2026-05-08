# Lingxi Signaling Server

灵犀 AI Agent 广域网信令服务器，用于跨互联网的 Agent 间发现与通信。

## 功能

- WebSocket 实时信令（注册/发现/消息中继）
- HMAC 签名验证 + 时间戳防重放
- TLS/WSS 支持
- 轻量无状态，单二进制部署

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 监听端口 | `9090` |
| `SIGNALING_SECRET` | HMAC 签名密钥（客户端需配置相同值） | 空（不验证） |
| `TLS_CERT` | TLS 证书路径 | 空（使用 WS） |
| `TLS_KEY` | TLS 私钥路径 | 空（使用 WS） |
| `ALLOWED_ORIGINS` | 允许的 Origin（逗号分隔） | 空（允许所有） |

## 本地运行

```bash
go run .
# 或指定端口和密钥
PORT=9090 SIGNALING_SECRET=mysecret go run .
```

## 部署到 Render

1. Fork 本仓库或推送到你的 GitHub
2. 登录 [render.com](https://render.com) → New → Web Service → 连接仓库
3. Render 会自动识别 `render.yaml` 配置
4. 在 Environment 中设置 `SIGNALING_SECRET`
5. 部署后地址格式：`wss://your-app.onrender.com/ws`

## 部署到 Fly.io

```bash
fly launch
fly secrets set SIGNALING_SECRET=你的密钥
fly deploy
```

## API

| 端点 | 说明 |
|------|------|
| `GET /ws` | WebSocket 信令连接 |
| `GET /api/peers` | 在线节点列表 |
| `GET /health` | 健康检查 |

## 在灵犀中配置

设置 → 网络与协作 → 广域网设置：
- **信令服务器地址**: `wss://your-domain.com/ws`
- **签名密钥**: 与 `SIGNALING_SECRET` 相同的值
