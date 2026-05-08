#!/usr/bin/env python3
"""
灵犀 LiteLLM Bridge
接受 Anthropic /v1/messages 请求，经 LiteLLM 转发到任意 OpenAI 兼容供应商，
并把流式响应实时转回 Anthropic SSE 格式返回给 Claude Code。

进程模型：由 Go 后端 router.EnsureRunning() spawn。
接口：
  POST /v1/messages   接收 Claude Code 的 Anthropic 请求
  POST /__config      Go 后端推送激活档案（仅写内存）
  GET  /__health      健康检查
  GET  /__status      同健康检查
"""

import json
import os
import sys
import uuid
import socket
import traceback
import socketserver
from http.server import HTTPServer, BaseHTTPRequestHandler

# ─── 导入 LiteLLM ────────────────────────────────────────────────
try:
    import litellm

    litellm.suppress_debug_info = True
    litellm.set_verbose = False

    import logging
    logging.getLogger("LiteLLM").setLevel(logging.WARNING)
    logging.getLogger("LiteLLM Router").setLevel(logging.WARNING)
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("httpcore").setLevel(logging.WARNING)
except ImportError:
    print(
        "[litellm-bridge] ERROR: litellm not installed.\n"
        "  Run: pip install litellm   (or see electron/resources/litellm-bridge/requirements.txt)",
        file=sys.stderr,
    )
    sys.exit(1)

HOST = os.environ.get("BRIDGE_HOST", "127.0.0.1")
PORT = int(os.environ.get("BRIDGE_PORT", "0"))
LOG_PREFIX = "[litellm-bridge]"

# 给经过本 bridge 的 OpenAI 兼容模型追加的强约束。
# Claude 等强模型走 Anthropic 直连不会经过这里，不受影响。

TOOL_DISCIPLINE_PREFIX = """
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
- 绝对不要说"我无法访问您的系统"、"我没法打开终端"、"需要专业人员"
- 绝对不要只列建议而不执行——你就是执行者
- 绝对不要调用 AskUserQuestion/AskFollowupQuestion 等工具（用户看不到输出）
"""

TOOL_DISCIPLINE_SUFFIX = """
[Skill 技能使用]
技能在 skills 目录下。涉及技能时：Read 读 SKILL.md → 按指引用 Bash 执行 → 中文汇报结果。
SKILL.md 指定的解释器、工作目录等细节禁止自行替换。

[Tool Calling 纪律]
1. 必须使用 function_call / tool_calls 格式调用工具，不要在文本中伪造 JSON。
2. Bash 工具给完整命令，一次成功，不要盲目试探。失败看错误信息，不超过 3 次重试。

[向用户提问的唯一方式]
在文本回复中用 ```json 代码块写交互式 JSON（前端自动渲染）：
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
"""

ACTION_KEYWORDS = [
    "打开", "执行", "运行", "查看", "搜索", "查找", "查询", "查日志", "查数据",
    "查订单", "帮我", "操作", "安装", "下载", "启动", "停止", "重启", "创建",
    "删除", "修改", "编辑", "读取", "写入", "复制", "移动", "看看", "检查",
    "open", "run", "execute", "search", "find", "check", "show", "list",
    "终端", "terminal", "命令", "command", "文件", "file",
]


def inject_tool_discipline(messages: list, tools: list | None) -> list:
    """将工具纪律约束注入到 system 消息中，并在 user 消息后追加动态提醒。"""
    if not messages:
        return messages

    # 动态列出可用工具名
    tool_list = ""
    if tools:
        names = [t.get("function", {}).get("name", "") for t in tools if t.get("function")]
        names = [n for n in names if n]
        if names:
            tool_list = (
                f"\n\n[当前可用工具列表]\n"
                f"你在本次对话中可以调用以下 {len(names)} 个工具：\n"
                + "\n".join(f"- {n}" for n in names)
                + "\n\n当用户的请求需要操作时，你必须从上面的列表中选择工具调用。"
            )

    prefix = TOOL_DISCIPLINE_PREFIX + tool_list
    suffix = TOOL_DISCIPLINE_SUFFIX

    # 注入到 system 消息
    sys_idx = next((i for i, m in enumerate(messages) if m.get("role") == "system"), -1)
    if sys_idx >= 0:
        base_text = messages[sys_idx].get("content", "")
        if isinstance(base_text, list):
            base_text = "".join(
                (p.get("text", "") if isinstance(p, dict) else str(p)) for p in base_text
            )
        messages[sys_idx]["content"] = prefix + "\n\n" + base_text + "\n\n" + suffix
    else:
        messages.insert(0, {"role": "system", "content": (prefix + "\n\n" + suffix).strip()})

    # 在最后一条 user 消息后追加工具调用提醒
    if tools:
        tool_msg_count = sum(1 for m in messages if m.get("role") == "tool")
        assistant_tc_count = sum(
            1 for m in messages
            if m.get("role") == "assistant" and m.get("tool_calls")
        )

        last_user_idx = -1
        for i in range(len(messages) - 1, -1, -1):
            if messages[i].get("role") == "user":
                last_user_idx = i
                break

        if last_user_idx >= 0:
            content = messages[last_user_idx].get("content", "")
            if isinstance(content, str):
                user_text = content
            elif isinstance(content, list):
                user_text = "".join(
                    (p.get("text", "") if isinstance(p, dict) else str(p)) for p in content
                )
            else:
                user_text = str(content)

            has_action = any(kw in user_text for kw in ACTION_KEYWORDS)

            if "[提醒：" not in user_text:
                reminder = ""
                if tool_msg_count == 0 and assistant_tc_count == 0:
                    reminder = "\n\n[提醒：你必须用 function call 调用工具来完成此任务，不要仅用文本回复。所有回复用中文。]"
                elif has_action:
                    reminder = '\n\n[提醒：这是一个需要你调用工具执行的操作请求。请用 function call 调用工具，不要说"我无法做到"。]'

                if reminder and isinstance(messages[last_user_idx].get("content"), str):
                    messages[last_user_idx]["content"] = user_text + reminder

    return messages

# 激活档案（仅内存，不落盘）
active = None  # { profileId, name, baseUrl, model, token }
stats = {"requests": 0, "errors": 0, "last_err": ""}


def log(*args):
    print(LOG_PREFIX, *args, flush=True)


# ─── SSE 工具 ─────────────────────────────────────────────────────

def sse_event(event_type: str, data: dict) -> bytes:
    return f"event: {event_type}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n".encode("utf-8")


# ─── URL 规范化 ───────────────────────────────────────────────────

def normalize_api_base(url: str) -> str:
    """
    去掉 /chat/completions 后缀，让 LiteLLM 自行追加。
    DeepSeek / DashScope / 大多数供应商都在 UI 里配了完整路径。
    """
    url = url.strip()
    for suffix in ("/chat/completions", "/chat/completion"):
        bare = url.rstrip("/")
        if bare.endswith(suffix):
            url = bare[: -len(suffix)]
            break
    return url.rstrip("/")


# ─── Anthropic → OpenAI 消息转换 ─────────────────────────────────

def _content_to_text(content) -> str:
    """从 Anthropic content（字符串或 block 列表）提取纯文本。"""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(
            b.get("text", "") for b in content
            if isinstance(b, dict) and b.get("type") == "text"
        )
    return str(content) if content is not None else ""


# ─── 图片多模态转发 ───────────────────────────────────────────────
# Claude CLI 用 Read 工具读取图片文件后，以 {type:"image", source:{type:"base64",...}}
# 放在 tool_result.content 数组中。需要提取出来转成 OpenAI image_url 格式。

TOOL_RESULT_TRUNCATE_LIMIT = 30000

def _extract_images_from_messages(anthropic_messages: list) -> list:
    """
    预处理：从最后一轮 tool_result 中提取图片，注入到最后一条 user 消息。
    只处理最后一条 user 消息及之后的消息，避免历史图片污染。
    返回修改后的 messages（原地修改）。
    """
    if not anthropic_messages:
        return anthropic_messages

    last_user_idx = -1
    for i in range(len(anthropic_messages) - 1, -1, -1):
        if anthropic_messages[i].get("role") == "user":
            last_user_idx = i
            break
    if last_user_idx < 0:
        return anthropic_messages

    pending_images = []

    for i in range(last_user_idx, len(anthropic_messages)):
        msg = anthropic_messages[i]
        content = msg.get("content")
        if not isinstance(content, list):
            continue

        for block in content:
            if block.get("type") == "tool_result" and isinstance(block.get("content"), list):
                image_blocks = [
                    c for c in block["content"]
                    if isinstance(c, dict) and c.get("type") == "image" and c.get("source")
                ]
                non_image_blocks = [
                    c for c in block["content"]
                    if not (isinstance(c, dict) and c.get("type") == "image")
                ]
                if image_blocks:
                    pending_images.extend(image_blocks)
                    if non_image_blocks:
                        block["content"] = non_image_blocks
                    else:
                        block["content"] = [{"type": "text", "text": "[图片内容已提取]"}]
                    log(f"[image] extracted {len(image_blocks)} image(s) from tool_result (msg {i})")

    if pending_images:
        user_msg = anthropic_messages[last_user_idx]
        content = user_msg.get("content", "")
        if isinstance(content, str):
            user_msg["content"] = [{"type": "text", "text": content}] + pending_images
        elif isinstance(content, list):
            user_msg["content"] = content + pending_images
        else:
            user_msg["content"] = pending_images
        log(f"[image] injected {len(pending_images)} image(s) into last user message (idx {last_user_idx})")

    return anthropic_messages


def _anthropic_image_to_openai(block: dict) -> dict | None:
    """将 Anthropic image block 转成 OpenAI image_url 格式。"""
    source = block.get("source", {})
    if source.get("type") == "base64":
        media_type = source.get("media_type", "image/jpeg")
        data = source.get("data", "")
        return {
            "type": "image_url",
            "image_url": {
                "url": f"data:{media_type};base64,{data}",
                "detail": "auto",
            },
        }
    elif source.get("type") == "url":
        return {
            "type": "image_url",
            "image_url": {"url": source.get("url", ""), "detail": "auto"},
        }
    return None


def _convert_content_blocks(content_list: list) -> list:
    """将 Anthropic content block 列表转成 OpenAI multimodal content 列表。"""
    parts = []
    for b in content_list:
        if not isinstance(b, dict):
            continue
        if b.get("type") == "text":
            text = b.get("text", "")
            if text:
                parts.append({"type": "text", "text": text})
        elif b.get("type") == "image" and b.get("source"):
            oai = _anthropic_image_to_openai(b)
            if oai:
                parts.append(oai)
    return parts


def anthropic_to_openai_messages(anthropic_messages: list, system=None) -> list:
    """
    将 Anthropic messages 转换成 OpenAI Chat Completions messages。
    关键处理：
      1. 图片：从 tool_result 提取并注入 user 消息，转成 OpenAI image_url 格式
      2. tool_result → role=tool 消息
      3. tool_use → tool_calls
    """
    # 预处理：提取 tool_result 中的图片
    anthropic_messages = _extract_images_from_messages(anthropic_messages)

    messages = []

    # system prompt（纪律约束在后面 inject_tool_discipline 中统一注入）
    if system:
        system_text = _content_to_text(system)
        messages.append({"role": "system", "content": system_text or ""})
    else:
        messages.append({"role": "system", "content": ""})

    for msg in anthropic_messages:
        role = msg.get("role", "user")
        content = msg.get("content", "")

        # 纯字符串内容
        if isinstance(content, str):
            messages.append({"role": role, "content": content})
            continue

        if not isinstance(content, list):
            messages.append({"role": role, "content": str(content)})
            continue

        tool_results = [b for b in content if isinstance(b, dict) and b.get("type") == "tool_result"]
        tool_uses    = [b for b in content if isinstance(b, dict) and b.get("type") == "tool_use"]
        text_blocks  = [b for b in content if isinstance(b, dict) and b.get("type") == "text"]
        image_blocks = [b for b in content if isinstance(b, dict) and b.get("type") == "image"]

        # user 消息含 tool_result → 拆成 role=tool 消息
        if role == "user" and tool_results:
            # 先输出非工具内容（含图片）
            non_tool = [b for b in content if isinstance(b, dict) and b.get("type") not in ("tool_result",)]
            if non_tool:
                parts = _convert_content_blocks(non_tool)
                if parts:
                    messages.append({"role": "user", "content": parts})

            for tr in tool_results:
                raw_result = tr.get("content", "")
                if isinstance(raw_result, list):
                    raw_result = "".join(
                        b.get("text", "") for b in raw_result
                        if isinstance(b, dict) and b.get("type") == "text"
                    )
                elif raw_result is None:
                    raw_result = ""
                result_str = str(raw_result)
                # 超长 tool_result 截断保护
                if len(result_str) > TOOL_RESULT_TRUNCATE_LIMIT:
                    result_str = result_str[:TOOL_RESULT_TRUNCATE_LIMIT] + "...[内容过长，已截断]"
                messages.append({
                    "role": "tool",
                    "tool_call_id": tr.get("tool_use_id", ""),
                    "content": result_str,
                })
            continue

        # assistant 消息含 tool_use → tool_calls
        if role == "assistant" and tool_uses:
            plain_text = "".join(b.get("text", "") for b in text_blocks)
            tool_calls = [
                {
                    "id": tu.get("id") or f"call_{uuid.uuid4().hex[:8]}",
                    "type": "function",
                    "function": {
                        "name": tu.get("name", ""),
                        "arguments": json.dumps(tu.get("input", {})),
                    },
                }
                for tu in tool_uses
            ]
            messages.append({
                "role": "assistant",
                "content": plain_text or None,
                "tool_calls": tool_calls,
            })
            continue

        # 普通消息（可能含图片）
        if image_blocks:
            parts = _convert_content_blocks(content)
            if parts:
                messages.append({"role": role, "content": parts})
        else:
            text = "".join(b.get("text", "") for b in text_blocks)
            if text:
                messages.append({"role": role, "content": text})
            elif content:
                messages.append({"role": role, "content": json.dumps(content)})

    return messages


def anthropic_to_openai_tools(tools: list) -> list | None:
    """将 Anthropic tools 转换成 OpenAI function 格式。"""
    if not tools:
        return None
    return [
        {
            "type": "function",
            "function": {
                "name": t.get("name", ""),
                "description": t.get("description", ""),
                "parameters": t.get("input_schema", {"type": "object", "properties": {}}),
            },
        }
        for t in tools
    ]


# ─── LiteLLM 流 → Anthropic SSE ──────────────────────────────────

def stream_to_anthropic_sse(response_iter, model: str, message_id: str):
    """
    消费 LiteLLM 流式响应（OpenAI 格式），实时 yield Anthropic SSE bytes。
    支持文本块和工具调用块，正确维护 block index。
    """
    yield sse_event("message_start", {
        "type": "message_start",
        "message": {
            "id": message_id,
            "type": "message",
            "role": "assistant",
            "content": [],
            "model": model,
            "stop_reason": None,
            "stop_sequence": None,
            "usage": {"input_tokens": 0, "output_tokens": 0},
        },
    })
    yield sse_event("ping", {"type": "ping"})

    next_block_idx = 0          # 下一个 Anthropic block 的 index
    text_block_idx = None       # 文本 block 使用的 index（None=尚未开始）
    thinking_block_idx = None   # 思考链 block 使用的 index
    # oai_tool_call_index -> {"block_idx": int, "id": str, "name": str}
    tool_blocks: dict = {}
    input_tokens = 0
    output_tokens = 0
    stop_reason = "end_turn"

    try:
        for chunk in response_iter:
            # 收集 usage（某些供应商在末尾单独发）
            if hasattr(chunk, "usage") and chunk.usage:
                u = chunk.usage
                pt = getattr(u, "prompt_tokens", None)
                ct = getattr(u, "completion_tokens", None)
                if pt is not None:
                    input_tokens = pt
                if ct is not None:
                    output_tokens = ct

            if not (hasattr(chunk, "choices") and chunk.choices):
                continue

            choice = chunk.choices[0]
            delta = getattr(choice, "delta", None)
            finish = getattr(choice, "finish_reason", None)

            if delta:
                # ── 思考链 delta（OpenAI 兼容供应商透传 reasoning_content / reasoning）──
                # 优先尝试属性，再尝试 dict 形式（部分供应商把 reasoning 放在 model_extra 里）
                reasoning_text = (
                    getattr(delta, "reasoning_content", None)
                    or getattr(delta, "reasoning", None)
                )
                if not reasoning_text:
                    # 尝试从原始 dict 取（litellm 有时通过 model_extra 暴露）
                    try:
                        raw = delta.model_dump() if hasattr(delta, "model_dump") else None
                    except Exception:
                        raw = None
                    if isinstance(raw, dict):
                        reasoning_text = raw.get("reasoning_content") or raw.get("reasoning")
                if reasoning_text:
                    if thinking_block_idx is None:
                        thinking_block_idx = next_block_idx
                        next_block_idx += 1
                        yield sse_event("content_block_start", {
                            "type": "content_block_start",
                            "index": thinking_block_idx,
                            "content_block": {"type": "thinking", "thinking": ""},
                        })
                    yield sse_event("content_block_delta", {
                        "type": "content_block_delta",
                        "index": thinking_block_idx,
                        "delta": {"type": "thinking_delta", "thinking": reasoning_text},
                    })

                # ── 文本 delta ──
                text = getattr(delta, "content", None)
                if text:
                    # 思考链结束（开始进入正式文本输出）→ 关闭 thinking block
                    if thinking_block_idx is not None:
                        yield sse_event("content_block_stop", {
                            "type": "content_block_stop",
                            "index": thinking_block_idx,
                        })
                        thinking_block_idx = None
                    if text_block_idx is None:
                        text_block_idx = next_block_idx
                        next_block_idx += 1
                        yield sse_event("content_block_start", {
                            "type": "content_block_start",
                            "index": text_block_idx,
                            "content_block": {"type": "text", "text": ""},
                        })
                    yield sse_event("content_block_delta", {
                        "type": "content_block_delta",
                        "index": text_block_idx,
                        "delta": {"type": "text_delta", "text": text},
                    })

                # ── 工具调用 delta ──
                tool_calls_delta = getattr(delta, "tool_calls", None)
                if tool_calls_delta:
                    for tc in tool_calls_delta:
                        oai_idx = getattr(tc, "index", 0)

                        # 新工具：开始一个 tool_use block
                        if oai_idx not in tool_blocks:
                            block_idx = next_block_idx
                            next_block_idx += 1
                            tool_id = getattr(tc, "id", None) or f"toolu_{uuid.uuid4().hex[:8]}"
                            tool_name = ""
                            if hasattr(tc, "function") and tc.function:
                                tool_name = getattr(tc.function, "name", None) or ""
                            tool_blocks[oai_idx] = {
                                "block_idx": block_idx,
                                "id": tool_id,
                                "name": tool_name,
                            }
                            yield sse_event("content_block_start", {
                                "type": "content_block_start",
                                "index": block_idx,
                                "content_block": {
                                    "type": "tool_use",
                                    "id": tool_id,
                                    "name": tool_name,
                                    "input": {},
                                },
                            })

                        tb = tool_blocks[oai_idx]

                        if hasattr(tc, "function") and tc.function:
                            # 补全 name（某些供应商 name 在后续 chunk 才到）
                            late_name = getattr(tc.function, "name", None)
                            if late_name and not tb["name"]:
                                tb["name"] = late_name

                            args_delta = getattr(tc.function, "arguments", None) or ""
                            if args_delta:
                                yield sse_event("content_block_delta", {
                                    "type": "content_block_delta",
                                    "index": tb["block_idx"],
                                    "delta": {
                                        "type": "input_json_delta",
                                        "partial_json": args_delta,
                                    },
                                })

            if finish:
                if finish == "tool_calls":
                    stop_reason = "tool_use"
                elif finish == "length":
                    stop_reason = "max_tokens"
                elif finish in ("stop", "eos", "end"):
                    stop_reason = "end_turn"

    except Exception as exc:
        log("stream error:", type(exc).__name__, str(exc))
        traceback.print_exc(file=sys.stderr)

    # ── 关闭所有 block ──────────────────────────────────────────
    if thinking_block_idx is not None:
        yield sse_event("content_block_stop", {
            "type": "content_block_stop",
            "index": thinking_block_idx,
        })
    if text_block_idx is not None:
        yield sse_event("content_block_stop", {
            "type": "content_block_stop",
            "index": text_block_idx,
        })


    for tb in sorted(tool_blocks.values(), key=lambda x: x["block_idx"]):
        yield sse_event("content_block_stop", {
            "type": "content_block_stop",
            "index": tb["block_idx"],
        })

    yield sse_event("message_delta", {
        "type": "message_delta",
        "delta": {"stop_reason": stop_reason, "stop_sequence": None},
        "usage": {"output_tokens": output_tokens},
    })
    yield sse_event("message_stop", {"type": "message_stop"})


# ─── HTTP 处理器 ───────────────────────────────────────────────────

class BridgeHandler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass  # 禁用默认访问日志

    # ── CORS 头 ──
    def _cors(self):
        self.send_header("access-control-allow-origin", "*")
        self.send_header("access-control-allow-headers", "*")

    def do_OPTIONS(self):
        self.send_response(204)
        self._cors()
        self.end_headers()

    def do_HEAD(self):
        self.send_response(200)
        self.end_headers()

    def do_GET(self):
        path = self.path.split("?")[0]
        if path in ("/__health", "/__status"):
            self._send_json(200, {
                "ok": True,
                "active": {
                    "profile_id": active["profileId"],
                    "name": active["name"],
                    "model": active["model"],
                } if active else None,
                "stats": stats,
            })
            return
        if path == "/" or path.startswith("/v1"):
            self.send_response(200)
            self.end_headers()
            return
        self._send_error(404, f"not found: GET {path}")

    def do_POST(self):
        length = int(self.headers.get("content-length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        try:
            body = json.loads(raw) if raw else {}
        except Exception as exc:
            return self._send_error(400, f"invalid json: {exc}")

        path = self.path.split("?")[0]
        if path.startswith("/__config"):
            return self._handle_config(body)
        if path.startswith("/v1/messages"):
            return self._handle_messages(body)
        self._send_error(404, f"not found: POST {path}")

    # ── 工具方法 ──

    def _send_json(self, code: int, obj: dict):
        body = json.dumps(obj, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("content-type", "application/json; charset=utf-8")
        self.send_header("content-length", str(len(body)))
        self._cors()
        self.end_headers()
        self.wfile.write(body)

    def _send_error(self, code: int, msg: str, extra: dict | None = None):
        log("error", code, msg)
        obj: dict = {"type": "error", "error": {"type": "bridge_error", "message": msg}}
        if extra:
            obj["error"].update(extra)
        self._send_json(code, obj)

    # ── 接口处理 ──

    def _handle_config(self, body: dict):
        global active
        if not body.get("base_url") or not body.get("model") or not body.get("token"):
            return self._send_error(400, "missing base_url / model / token")
        active = {
            "profileId": body.get("profile_id", 0),
            "name": body.get("name", ""),
            "baseUrl": body["base_url"],
            "model": body["model"],
            "token": body["token"],
        }
        log(f"config updated: profileId={active['profileId']} model={active['model']}")
        self._send_json(200, {"ok": True})

    def _handle_messages(self, inbound: dict):
        global stats
        if not active:
            return self._send_error(503, "bridge not configured: no active profile yet")

        stats["requests"] += 1
        message_id = f"msg_{uuid.uuid4().hex[:16]}"

        # ── 请求体转换 ──────────────────────────────────────────
        try:
            messages = anthropic_to_openai_messages(
                inbound.get("messages", []),
                inbound.get("system"),
            )
            tools = anthropic_to_openai_tools(inbound.get("tools"))
            # 注入工具纪律约束
            messages = inject_tool_discipline(messages, tools)
        except Exception as exc:
            stats["errors"] += 1
            stats["last_err"] = str(exc)
            return self._send_error(500, f"message conversion failed: {exc}")

        # max_tokens 动态调整：按模型名判断上限
        max_tokens = int(inbound.get("max_tokens") or 4096)
        model_lower = (active.get("model") or "").lower()
        if "deepseek" in model_lower or "qwen-long" in model_lower or "gpt-4" in model_lower:
            max_limit = 32768
        elif "qwen-max" in model_lower or "qwen-plus" in model_lower or "glm" in model_lower:
            max_limit = 16384
        elif "qwen-turbo" in model_lower or "mini" in model_lower:
            max_limit = 8192
        else:
            max_limit = 16384
        if max_tokens > max_limit:
            max_tokens = max_limit

        # 诊断日志
        image_count = sum(
            1 for m in messages if isinstance(m.get("content"), list)
            for p in (m.get("content") if isinstance(m.get("content"), list) else [])
            if isinstance(p, dict) and p.get("type") == "image_url"
        )
        log(f"[diag] model={active['model']} tools={len(tools) if tools else 0} imgs={image_count} max_tokens={max_tokens}")

        api_base = normalize_api_base(active["baseUrl"])
        # LiteLLM OpenAI-compatible 路由：model 前缀 openai/
        model_str = f"openai/{active['model']}"

        call_kwargs: dict = {
            "model": model_str,
            "messages": messages,
            "api_base": api_base,
            "api_key": active["token"],
            "stream": True,
            "max_tokens": max_tokens,
            "stream_options": {"include_usage": True},
        }
        if tools:
            call_kwargs["tools"] = tools
        temperature = inbound.get("temperature")
        if temperature is not None:
            call_kwargs["temperature"] = float(temperature)

        # ── 发送 SSE 头 ─────────────────────────────────────────
        # 每个 /v1/messages 是一次完整交换，SSE 结束后主动关闭连接，
        # 避免 keep-alive 导致客户端 read() 无限阻塞。
        self.close_connection = True
        self.send_response(200)
        self.send_header("content-type", "text/event-stream; charset=utf-8")
        self.send_header("cache-control", "no-cache")
        self.send_header("connection", "close")
        self._cors()
        self.end_headers()

        # ── 流式输出 ─────────────────────────────────────────────
        try:
            response = litellm.completion(**call_kwargs)
            for chunk_bytes in stream_to_anthropic_sse(response, active["model"], message_id):
                self.wfile.write(chunk_bytes)
                self.wfile.flush()
        except Exception as exc:
            stats["errors"] += 1
            stats["last_err"] = str(exc)
            log("completion error:", type(exc).__name__, str(exc))
            traceback.print_exc(file=sys.stderr)
            err_json = json.dumps({
                "type": "error",
                "error": {"type": "bridge_error", "message": str(exc)},
            })
            try:
                self.wfile.write(f"event: error\ndata: {err_json}\n\n".encode("utf-8"))
                self.wfile.flush()
            except Exception:
                pass


# ─── ThreadingHTTPServer（每请求一线程，支持并发探活）─────────────

class _ThreadingHTTPServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True


# ─── 入口 ─────────────────────────────────────────────────────────

def _pick_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("", 0))
        return s.getsockname()[1]


def main():
    port = PORT if PORT != 0 else _pick_free_port()
    server = _ThreadingHTTPServer((HOST, port), BridgeHandler)
    log(f"listening on {HOST}:{port}")
    # Go 父进程通过 stdout 检测此行确认 bridge 已就绪
    sys.stdout.write(f"BRIDGE_READY port={port}\n")
    sys.stdout.flush()
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log("shutting down (SIGINT)")
        server.shutdown()


if __name__ == "__main__":
    main()


