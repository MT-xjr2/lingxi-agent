// 灵犀本地协议路由层 — 用 supermemoryai/llm-bridge 把 Anthropic 协议
// 透明翻译成任意 OpenAI 兼容厂商，再把 OpenAI SSE 流翻译回 Anthropic SSE。
//
// 进程模型：由 Go 后端 (router 包) spawn，监听 BRIDGE_PORT（默认 0=随机）。
// 端点：
//   POST /v1/messages   —— 接 claude-code CLI 的请求（Anthropic 协议）
//   POST /__config      —— Go 后端推 active profile（base_url / model / token）
//   GET  /__health      —— 健康检查（含当前 active profile 摘要）
//   GET  /__status      —— 调试信息

import http from 'node:http'
import { EventEmitter } from 'node:events'
import {
  toUniversal,
  fromUniversal,
  handleUniversalStreamRequest,
} from 'llm-bridge'

const HOST = process.env.BRIDGE_HOST || '127.0.0.1'
const PORT = parseInt(process.env.BRIDGE_PORT || '0', 10)
const LOG_PREFIX = '[bridge]'

// 单一 active 配置（Go 后端会在 profile 切换时 POST /__config 覆盖）
let active = null // { profileId, name, baseUrl, model, token }
let stats = { requests: 0, errors: 0, lastErr: '', startedAt: new Date().toISOString() }

// ─── 工具纪律约束（核心约束置顶利用 primacy bias，末尾放 few-shot）────

const TOOL_DISCIPLINE_PREFIX = `
[ABSOLUTE RULE — 语言]
你的所有输出必须使用中文（简体中文），包括思考过程（thinking）也必须用中文。
禁止在思考或回复中使用英文（代码和命令除外）。

[ABSOLUTE RULE — 你是一个 Agent]
你是一个拥有完整工具集的 AI Agent。你必须通过调用工具（function call）来完成用户的请求。
你可以读文件、执行命令、搜索网络、操作系统——你不是普通聊天机器人。

**铁律：当用户的请求涉及任何操作（查数据、执行命令、打开应用、读文件、搜索……），你必须调用工具，绝对禁止仅用文本回复。**

[操作指南 — 如果用户说X，你必须做Y]
- "打开终端" / "打开Terminal" → 调用 Bash 工具执行: open -a Terminal
- "执行命令xxx" / "运行xxx" → 调用 Bash 工具执行该命令
- "查看文件xxx" / "读取xxx" → 调用 Read 工具读取文件
- "搜索xxx" / "查找xxx" → 调用 Bash 工具执行 grep/find，或调用 WebSearch
- "帮我查日志" / "查订单" / "查数据" → 先用 Read 读取相关 SKILL.md，再用 Bash 执行
- "打开xxx应用" → 调用 Bash 工具执行: open -a "应用名"
- "看看我的系统信息" → 调用 Bash 工具执行: uname -a && sw_vers 等命令
- 用户发送了图片 → 直接描述你在图片中看到的内容（你有视觉能力）

[禁止行为]
- ❌ 绝对不要说"我无法访问您的系统"、"我没法打开终端"、"需要专业人员"
- ❌ 绝对不要只列建议而不执行——你就是执行者
- ❌ 绝对不要调用 AskUserQuestion/AskFollowupQuestion 等工具（用户看不到输出）
`

const TOOL_DISCIPLINE_SUFFIX = `
[Skill 技能使用]
技能在 skills 目录下。涉及技能时：Read 读 SKILL.md → 按指引用 Bash 执行 → 中文汇报结果。
SKILL.md 指定的解释器、工作目录等细节禁止自行替换。

[Tool Calling 纪律]
1. 必须使用 function_call / tool_calls 格式调用工具，不要在文本中伪造 JSON。
2. Bash 工具给完整命令，一次成功，不要盲目试探。失败看错误信息，不超过 3 次重试。

[向用户提问的唯一方式]
在文本回复中用 \`\`\`json 代码块写交互式 JSON（前端自动渲染）：
选择题: {"type":"choice","id":"xxx","title":"问题","multi":false,"options":[{"id":"a","label":"选项A"},{"id":"b","label":"选项B"}]}
填写框: {"type":"input","id":"xxx","title":"请提供信息","fields":[{"id":"f1","label":"字段名","placeholder":"提示","required":true}]}

[工具调用示例]

示例1 — 用户说"打开终端"：
你应该调用 Bash 工具，command 参数为: open -a Terminal
然后回复："终端已经帮你打开了！"

示例2 — 用户说"帮我查看系统信息"：
你应该调用 Bash 工具，command 参数为: uname -a && sw_vers && sysctl -n machdep.cpu.brand_string && system_profiler SPHardwareDataType 2>/dev/null | grep -E "Memory|Chip"
然后用中文整理输出给用户。

示例3 — 用户说"帮我搜索本地文件xxx"：
你应该调用 Bash 工具，command 参数为: find ~ -name "*xxx*" -maxdepth 5 2>/dev/null | head -20
然后汇报搜索结果。
`

// 操作性关键词：当最后一条 user 消息包含这些词时，追加更强的 tool-use 提示
const ACTION_KEYWORDS = [
  '打开', '执行', '运行', '查看', '搜索', '查找', '查询', '查日志', '查数据',
  '查订单', '帮我', '操作', '安装', '下载', '启动', '停止', '重启', '创建',
  '删除', '修改', '编辑', '读取', '写入', '复制', '移动', '看看', '检查',
  'open', 'run', 'execute', 'search', 'find', 'check', 'show', 'list',
  '终端', 'terminal', '命令', 'command', '文件', 'file',
]

// 把工具纪律约束注入到 OpenAI messages 数组的 system 消息里。
function injectToolDiscipline(openaiBody) {
  if (!openaiBody || !Array.isArray(openaiBody.messages)) return openaiBody

  // 动态列出本次请求中可用的工具名
  let toolList = ''
  if (openaiBody.tools && openaiBody.tools.length > 0) {
    const names = openaiBody.tools
      .map(t => t.function?.name || t.name)
      .filter(Boolean)
    if (names.length > 0) {
      toolList = `\n\n[当前可用工具列表]\n你在本次对话中可以调用以下 ${names.length} 个工具：\n${names.map(n => `- ${n}`).join('\n')}\n\n当用户的请求需要操作时，你必须从上面的列表中选择工具调用。`
    }
  }

  // 核心约束放在 system prompt 最前面（primacy bias），few-shot 放最后
  const prefix = TOOL_DISCIPLINE_PREFIX + toolList
  const suffix = TOOL_DISCIPLINE_SUFFIX

  const sysIdx = openaiBody.messages.findIndex((m) => m.role === 'system')
  if (sysIdx >= 0) {
    const existing = openaiBody.messages[sysIdx]
    const baseText = typeof existing.content === 'string'
      ? existing.content
      : Array.isArray(existing.content)
        ? existing.content.map((c) => (typeof c === 'string' ? c : c?.text || '')).join('')
        : ''
    // 核心约束 + 原始 system prompt + few-shot 示例
    openaiBody.messages[sysIdx] = { ...existing, content: prefix + '\n\n' + baseText + '\n\n' + suffix }
  } else {
    openaiBody.messages.unshift({ role: 'system', content: (prefix + '\n\n' + suffix).trim() })
  }

  // 在最后一条 user 消息后追加 tool-use 提醒
  if (openaiBody.tools?.length > 0) {
    const msgs = openaiBody.messages
    const toolMsgCount = msgs.filter(m => m.role === 'tool').length
    const assistantToolCallCount = msgs.filter(m => m.role === 'assistant' && m.tool_calls?.length > 0).length

    // 找到最后一条 user 消息
    let lastUserIdx = -1
    for (let i = msgs.length - 1; i >= 0; i--) {
      if (msgs[i].role === 'user') { lastUserIdx = i; break }
    }

    if (lastUserIdx >= 0) {
      const lastUserContent = typeof msgs[lastUserIdx].content === 'string'
        ? msgs[lastUserIdx].content
        : Array.isArray(msgs[lastUserIdx].content)
          ? msgs[lastUserIdx].content.map(c => typeof c === 'string' ? c : c?.text || '').join('')
          : ''

      // 检测是否包含操作性关键词
      const hasActionKeyword = ACTION_KEYWORDS.some(kw => lastUserContent.includes(kw))

      if (!lastUserContent.includes('[提醒：')) {
        let reminder = ''
        if (toolMsgCount === 0 && assistantToolCallCount === 0) {
          // 首轮：通用工具调用提醒
          reminder = '\n\n[提醒：你必须用 function call 调用工具来完成此任务，不要仅用文本回复。所有回复用中文。]'
        } else if (hasActionKeyword) {
          // 多轮但包含操作关键词：继续强调工具调用
          reminder = '\n\n[提醒：这是一个需要你调用工具执行的操作请求。请用 function call 调用工具，不要说"我无法做到"。]'
        }

        if (reminder && typeof msgs[lastUserIdx].content === 'string') {
          msgs[lastUserIdx] = { ...msgs[lastUserIdx], content: lastUserContent + reminder }
        }
      }
    }
  }

  return openaiBody
}

function log(...args) {
  console.log(LOG_PREFIX, ...args)
}

function readJSON(req) {
  return new Promise((resolve, reject) => {
    const chunks = []
    req.on('data', (c) => chunks.push(c))
    req.on('end', () => {
      try {
        const buf = Buffer.concat(chunks)
        if (!buf.length) return resolve({})
        resolve(JSON.parse(buf.toString('utf8')))
      } catch (e) {
        reject(e)
      }
    })
    req.on('error', reject)
  })
}

function sendJSON(res, code, obj) {
  res.statusCode = code
  res.setHeader('content-type', 'application/json; charset=utf-8')
  res.end(JSON.stringify(obj))
}

function sendError(res, code, msg, extra = {}) {
  log('error', code, msg, extra)
  sendJSON(res, code, { type: 'error', error: { type: 'bridge_error', message: msg, ...extra } })
}

// ─── 图片多模态转发 ─────────────────────────────────────────────
// Anthropic 协议中图片出现在两个位置：
// 1. user message content: [{ type: "image", source: { type: "base64", data, media_type } }]
// 2. tool_result content: tool_result.content 数组中可能包含 { type: "image", source: {...} }
//    （Claude CLI 用 Read 工具读取图片文件后，以这种格式放在 tool_result 里）
//
// llm-bridge 可以处理第 1 种，但 tool_result 的 content 被当作 result 字符串，图片数据丢失。
// 解决：在传入 llm-bridge 之前预处理，从 tool_result 提取图片并注入最近的 user 消息。

function extractImagesFromAnthropicRequest(inbound) {
  if (!inbound?.messages || !Array.isArray(inbound.messages)) return inbound

  // 只处理最后一轮对话中的 tool_result 图片（避免历史图片污染）
  // 找到最后一条 user 消息的位置，只扫描它之后的 tool_result
  let lastUserIdx = -1
  for (let i = inbound.messages.length - 1; i >= 0; i--) {
    if (inbound.messages[i].role === 'user') {
      lastUserIdx = i
      break
    }
  }
  if (lastUserIdx < 0) return inbound

  const pendingImages = []

  // 只扫描最后一条 user 消息之后的消息中的 tool_result
  for (let i = lastUserIdx; i < inbound.messages.length; i++) {
    const msg = inbound.messages[i]
    if (!Array.isArray(msg.content)) continue

    for (const block of msg.content) {
      if (block.type === 'tool_result' && Array.isArray(block.content)) {
        const imageBlocks = block.content.filter(c => c.type === 'image' && c.source)
        const nonImageBlocks = block.content.filter(c => c.type !== 'image')

        if (imageBlocks.length > 0) {
          pendingImages.push(...imageBlocks)
          if (nonImageBlocks.length > 0) {
            block.content = nonImageBlocks
          } else {
            block.content = [{ type: 'text', text: '[图片内容已提取]' }]
          }
          log(`[image] extracted ${imageBlocks.length} image(s) from tool_result (msg ${i})`)
        }
      }
    }
  }

  // 将提取的图片注入到最后一条 user 消息中
  if (pendingImages.length > 0) {
    const userMsg = inbound.messages[lastUserIdx]
    if (typeof userMsg.content === 'string') {
      userMsg.content = [{ type: 'text', text: userMsg.content }, ...pendingImages]
    } else if (Array.isArray(userMsg.content)) {
      userMsg.content = [...userMsg.content, ...pendingImages]
    } else {
      userMsg.content = [...pendingImages]
    }
    log(`[image] injected ${pendingImages.length} image(s) into last user message (idx ${lastUserIdx})`)
  }

  return inbound
}

// 后处理：确保 OpenAI 请求体中的图片都是 image_url 格式
function ensureOpenAIImageFormat(openaiBody) {
  if (!openaiBody?.messages) return openaiBody

  for (const msg of openaiBody.messages) {
    if (!Array.isArray(msg.content)) continue

    msg.content = msg.content.map(part => {
      // llm-bridge 可能输出 universal image 格式
      if (part.type === 'image' && part.media) {
        const url = part.media.url
          || (part.media.data && part.media.mimeType
            ? `data:${part.media.mimeType};base64,${part.media.data}`
            : part.media.data || '')
        return {
          type: 'image_url',
          image_url: { url, detail: part.media.detail || 'auto' }
        }
      }
      // Anthropic 原始格式残留
      if (part.type === 'image' && part.source) {
        if (part.source.type === 'base64') {
          return {
            type: 'image_url',
            image_url: {
              url: `data:${part.source.media_type || 'image/jpeg'};base64,${part.source.data}`,
              detail: 'auto'
            }
          }
        } else if (part.source.type === 'url') {
          return {
            type: 'image_url',
            image_url: { url: part.source.url, detail: 'auto' }
          }
        }
      }
      return part
    })
  }

  return openaiBody
}

// ─── 工具结果格式修正 ───────────────────────────────────────────
// Anthropic 工具结果在 Claude Code 请求里通常是：
//   role=user, content=[{ type:'tool_result', tool_use_id:'...' }]
// OpenAI Chat Completions 标准要求工具结果必须是：
//   role=tool, tool_call_id='...', content='...'
// llm-bridge 的通用转换会保留原始 role=user，导致部分 OpenAI 兼容模型把工具结果当普通用户消息，
// 从而破坏 “tool_use -> 执行工具 -> tool_result -> 继续推理” 的 Agent 闭环。
function normalizeToolResultsForOpenAI(universal) {
  if (!universal?.messages || !Array.isArray(universal.messages)) return universal

  const normalized = []
  for (const msg of universal.messages) {
    const toolResults = (msg.content || []).filter((c) => c.type === 'tool_result' && c.tool_result)
    if (msg.role !== 'user' || toolResults.length === 0) {
      normalized.push(msg)
      continue
    }

    const nonToolContent = (msg.content || []).filter((c) => c.type !== 'tool_result')
    if (nonToolContent.length > 0) {
      normalized.push({ ...msg, content: nonToolContent })
    }

    toolResults.forEach((content, idx) => {
      const tr = content.tool_result || {}

      // 截断超长工具结果（部分 OpenAI 兼容模型对长 tool_result 处理差）
      let resultContent = [content]
      if (tr.result && typeof tr.result === 'string' && tr.result.length > 30000) {
        const truncated = tr.result.slice(0, 30000) + '\n\n...[内容过长，已截断]'
        resultContent = [{
          ...content,
          tool_result: { ...tr, result: truncated }
        }]
        log(`[tool_result] truncated from ${tr.result.length} to 30000 chars`)
      }

      normalized.push({
        ...msg,
        id: `${msg.id || 'tool_result'}_${idx}`,
        role: 'tool',
        content: resultContent,
        metadata: {
          ...(msg.metadata || {}),
          tool_call_id: tr.tool_call_id || tr.metadata?.tool_use_id || '',
          name: tr.name || '',
          normalized_from_anthropic_tool_result: true,
        },
      })
    })
  }

  universal.messages = normalized
  return universal
}

// ─── 处理器 ───────────────────────────────────────────────────────

async function handleConfig(req, res) {
  let body
  try {
    body = await readJSON(req)
  } catch (e) {
    return sendError(res, 400, 'invalid json: ' + e.message)
  }
  if (!body.base_url || !body.model || !body.token) {
    return sendError(res, 400, 'missing base_url / model / token')
  }
  active = {
    profileId: body.profile_id || 0,
    name: body.name || '',
    baseUrl: body.base_url,
    model: body.model,
    token: body.token,
  }
  log('config updated:', { profileId: active.profileId, name: active.name, model: active.model })
  sendJSON(res, 200, { ok: true })
}

async function handleHealth(_req, res) {
  sendJSON(res, 200, {
    ok: true,
    active: active
      ? { profile_id: active.profileId, name: active.name, model: active.model }
      : null,
    stats,
  })
}

// 从 OpenAI SSE 流中分离 reasoning_content / reasoning 字段。
// 返回一个过滤后的 ReadableStream（reasoning 已移除）和一个 EventEmitter（发出 reasoning 事件）。
// 这样 llm-bridge 只处理 text/tool 内容，reasoning 由 bridge 直接生成 Anthropic thinking 事件。
function splitReasoningFromStream(upstreamBody) {
  const emitter = new EventEmitter()
  let hasReasoning = false
  let buffer = ''

  const filtered = new ReadableStream({
    async start(controller) {
      const reader = upstreamBody.getReader()
      const decoder = new TextDecoder()

      try {
        while (true) {
          const { value, done } = await reader.read()
          if (done) {
            // 处理缓冲区中剩余的数据
            if (buffer.trim()) {
              processBuffer(buffer, controller, emitter, () => hasReasoning)
            }
            if (hasReasoning) emitter.emit('reasoning_end')
            controller.close()
            break
          }

          buffer += decoder.decode(value, { stream: true })

          // 按完整的 SSE 事件分割处理
          const lines = buffer.split('\n')
          buffer = lines.pop() || '' // 保留最后一个不完整的行

          let currentEventLines = []
          for (const line of lines) {
            currentEventLines.push(line)
            if (line === '') {
              // 空行表示一个完整的 SSE 事件结束
              const eventText = currentEventLines.join('\n') + '\n'
              const dataLine = currentEventLines.find(l => l.startsWith('data: '))
              currentEventLines = []

              if (!dataLine) {
                // 非 data 行原样转发
                controller.enqueue(new TextEncoder().encode(eventText))
                continue
              }

              const jsonStr = dataLine.slice(6)
              if (jsonStr === '[DONE]') {
                if (hasReasoning) emitter.emit('reasoning_end')
                controller.enqueue(new TextEncoder().encode(eventText))
                continue
              }

              try {
                const parsed = JSON.parse(jsonStr)
                const delta = parsed?.choices?.[0]?.delta
                if (!delta) {
                  controller.enqueue(new TextEncoder().encode(eventText))
                  continue
                }

                const reasoning = delta.reasoning_content || delta.reasoning || ''
                if (reasoning) {
                  hasReasoning = true
                  emitter.emit('reasoning', reasoning)

                  // 如果这个 chunk 同时有 content，保留 content 部分
                  if (delta.content) {
                    const cleaned = { ...parsed }
                    cleaned.choices = parsed.choices.map(c => ({
                      ...c,
                      delta: { ...c.delta, reasoning_content: undefined, reasoning: undefined }
                    }))
                    const newLine = `data: ${JSON.stringify(cleaned)}\n\n`
                    controller.enqueue(new TextEncoder().encode(newLine))
                  }
                  // 纯 reasoning chunk 不转发给 llm-bridge
                } else {
                  // 非 reasoning chunk 原样转发
                  if (hasReasoning && delta.content && !delta.reasoning_content && !delta.reasoning) {
                    // reasoning 阶段结束，content 阶段开始
                    emitter.emit('reasoning_end')
                    hasReasoning = false
                  }
                  controller.enqueue(new TextEncoder().encode(eventText))
                }
              } catch {
                // JSON 解析失败原样转发
                controller.enqueue(new TextEncoder().encode(eventText))
              }
            }
          }
        }
      } catch (e) {
        if (hasReasoning) emitter.emit('reasoning_end')
        controller.error(e)
      }
    },
  })

  return { filteredStream: filtered, reasoningEmitter: emitter }
}

function processBuffer(buf, controller, emitter, hasReasoningFn) {
  const encoder = new TextEncoder()
  if (buf.startsWith('data: ')) {
    controller.enqueue(encoder.encode(buf + '\n\n'))
  }
}

// 把 Anthropic /v1/messages 请求 → OpenAI Chat Completions 上游，再把 OpenAI SSE 流翻成 Anthropic SSE 流回客户端
async function handleMessages(req, res) {
  if (!active) {
    return sendError(res, 503, 'bridge not configured: no active profile yet')
  }

  let inbound
  try {
    inbound = await readJSON(req)
  } catch (e) {
    return sendError(res, 400, 'invalid json: ' + e.message)
  }

  const wantsStream = inbound.stream !== false // 默认为流
  stats.requests += 1

  // ── 0. 图片预处理：从 tool_result 中提取图片到 user 消息 ─────
  extractImagesFromAnthropicRequest(inbound)

  // ── 1. Anthropic → OpenAI 请求体 ─────────────────────────────
  let openaiBody
  try {
    const universal = normalizeToolResultsForOpenAI(toUniversal('anthropic', inbound))
    openaiBody = fromUniversal('openai', universal)
  } catch (e) {
    stats.errors += 1
    stats.lastErr = 'translate_request: ' + e.message
    return sendError(res, 500, 'translate request failed: ' + e.message)
  }

  // ── 1.5 图片后处理：确保所有图片都是 OpenAI image_url 格式 ──
  ensureOpenAIImageFormat(openaiBody)

  // 上游模型名以 active.model 为准（前端可能发的是 anthropic 模型名）
  openaiBody.model = active.model
  openaiBody.stream = true
  // 让 OpenAI 兼容上游在 stream 中也带 usage（DashScope/DeepSeek/GLM 等都支持），
  // 否则 llm-bridge 的 emitter 会回 output_tokens=0
  openaiBody.stream_options = { ...(openaiBody.stream_options || {}), include_usage: true }

  // 注入 OpenAI 小模型工具纪律约束
  injectToolDiscipline(openaiBody)

  // 确保 tool_choice 至少为 "auto"（部分 OpenAI 兼容提供商需要显式设置）
  if (openaiBody.tools?.length > 0 && !openaiBody.tool_choice) {
    openaiBody.tool_choice = 'auto'
  }

  // 诊断日志
  const toolCount = openaiBody.tools?.length || 0
  const roleDist = (openaiBody.messages || []).reduce((acc, m) => {
    acc[m.role] = (acc[m.role] || 0) + 1
    return acc
  }, {})
  const imageCount = (openaiBody.messages || []).reduce((acc, m) => {
    if (!Array.isArray(m.content)) return acc
    return acc + m.content.filter(c => c.type === 'image_url').length
  }, 0)
  log(`[diag] model=${openaiBody.model} tools=${toolCount} imgs=${imageCount} msgs=${JSON.stringify(roleDist)} tool_choice=${JSON.stringify(openaiBody.tool_choice)}`)
  if (toolCount > 0) {
    log(`[diag] tool_names: ${openaiBody.tools.map(t => t.function?.name || t.name).join(', ')}`)
  }

  // max_tokens 动态调整：根据模型名判断上限
  if (typeof openaiBody.max_tokens === 'number') {
    const model = (openaiBody.model || '').toLowerCase()
    let maxLimit = 16384 // 默认上限
    // 部分模型有更高的输出限制
    if (model.includes('deepseek') || model.includes('qwen-long') || model.includes('gpt-4')) {
      maxLimit = 32768
    } else if (model.includes('qwen-max') || model.includes('qwen-plus') || model.includes('glm')) {
      maxLimit = 16384
    } else if (model.includes('qwen-turbo') || model.includes('mini')) {
      maxLimit = 8192
    }
    if (openaiBody.max_tokens > maxLimit) {
      openaiBody.max_tokens = maxLimit
    }
  }

  // ── 2. 调上游 ───────────────────────────────────────────────
  let upstreamResp
  try {
    upstreamResp = await fetch(active.baseUrl, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'authorization': active.token.startsWith('Bearer ') ? active.token : `Bearer ${active.token}`,
      },
      body: JSON.stringify(openaiBody),
    })
  } catch (e) {
    stats.errors += 1
    stats.lastErr = 'upstream_fetch: ' + e.message
    return sendError(res, 502, 'upstream fetch failed: ' + e.message)
  }

  if (!upstreamResp.ok) {
    const text = await upstreamResp.text().catch(() => '')
    stats.errors += 1
    stats.lastErr = `upstream ${upstreamResp.status}: ${text.slice(0, 200)}`
    return sendError(res, upstreamResp.status, `upstream returned ${upstreamResp.status}`, { upstream_body: text.slice(0, 1000) })
  }

  // ── 3. SSE 流式翻译（含 reasoning token 透传）────────────────
  res.statusCode = 200
  res.setHeader('content-type', 'text/event-stream; charset=utf-8')
  res.setHeader('cache-control', 'no-cache')
  res.setHeader('connection', 'keep-alive')
  res.flushHeaders?.()

  try {
    // 拦截上游 SSE 流：提取 reasoning_content / reasoning 字段，
    // 生成 Anthropic thinking 事件，剩余内容交给 llm-bridge 翻译。
    const { filteredStream, reasoningEmitter } = splitReasoningFromStream(upstreamResp.body)

    let thinkingBlockStarted = false
    let thinkingBlockIdx = 0

    // 并行：reasoning 事件直接写入 res
    reasoningEmitter.on('reasoning', (text) => {
      if (!thinkingBlockStarted) {
        thinkingBlockStarted = true
        const startEvt = {
          type: 'content_block_start',
          index: thinkingBlockIdx,
          content_block: { type: 'thinking', thinking: '' },
        }
        res.write(`event: content_block_start\ndata: ${JSON.stringify(startEvt)}\n\n`)
      }
      const deltaEvt = {
        type: 'content_block_delta',
        index: thinkingBlockIdx,
        delta: { type: 'thinking_delta', thinking: text },
      }
      res.write(`event: content_block_delta\ndata: ${JSON.stringify(deltaEvt)}\n\n`)
    })
    reasoningEmitter.on('reasoning_end', () => {
      if (thinkingBlockStarted) {
        const stopEvt = {
          type: 'content_block_stop',
          index: thinkingBlockIdx,
        }
        res.write(`event: content_block_stop\ndata: ${JSON.stringify(stopEvt)}\n\n`)
        thinkingBlockIdx++
      }
    })

    // llm-bridge 翻译剩余的 text/tool 内容
    const anthropicStream = handleUniversalStreamRequest(
      filteredStream,
      'openai',
      'anthropic'
    )

    const reader = anthropicStream.getReader()
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      if (value) res.write(value)
    }
    res.end()
  } catch (e) {
    stats.errors += 1
    stats.lastErr = 'stream_translate: ' + e.message
    log('stream translate error:', e)
    try {
      res.write(`event: error\ndata: ${JSON.stringify({ type: 'error', error: { type: 'bridge_error', message: e.message } })}\n\n`)
    } catch {}
    try { res.end() } catch {}
  }

  if (wantsStream === false) {
    // 暂未支持非流；客户端基本都会要 stream
    log('warning: client requested non-stream, served as stream anyway')
  }
}

// ─── HTTP server ────────────────────────────────────────────────

const server = http.createServer(async (req, res) => {
  // CORS（只对本地）
  res.setHeader('access-control-allow-origin', '*')
  res.setHeader('access-control-allow-headers', '*')
  if (req.method === 'OPTIONS') {
    res.statusCode = 204
    return res.end()
  }

  const url = req.url || '/'
  if (url.startsWith('/__config') && req.method === 'POST') return handleConfig(req, res)
  if (url.startsWith('/__health') && req.method === 'GET') return handleHealth(req, res)
  if (url.startsWith('/__status') && req.method === 'GET') return handleHealth(req, res)
  if (url.startsWith('/v1/messages') && req.method === 'POST') return handleMessages(req, res)

  // Anthropic SDK 偶尔会发 HEAD/GET 到 /v1/messages 或 / 探活，给 200 静默返回
  if ((req.method === 'HEAD' || req.method === 'GET') && (url === '/' || url.startsWith('/v1'))) {
    res.statusCode = 200
    return res.end()
  }

  sendError(res, 404, `not found: ${req.method} ${url}`)
})

server.listen(PORT, HOST, () => {
  const port = server.address().port
  log(`listening on ${HOST}:${port}`)
  // 把 port 打到 stdout 第一行，让 Go 父进程能解析
  console.log(`BRIDGE_READY port=${port}`)
})

// 优雅退出
function shutdown(sig) {
  log('shutting down on', sig)
  server.close(() => process.exit(0))
  setTimeout(() => process.exit(1), 2000).unref()
}
process.on('SIGINT', () => shutdown('SIGINT'))
process.on('SIGTERM', () => shutdown('SIGTERM'))
process.on('uncaughtException', (e) => {
  log('uncaughtException:', e)
  stats.errors += 1
  stats.lastErr = 'uncaught: ' + e.message
})
