import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  MessageSquare, Pause, Hand, XCircle, CheckCircle, Clock, Target,
  ArrowLeft, Sparkles, FileText, Shield, BookOpen,
} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { api, wsClient } from '../api/client';
import { useStore } from '../state/useStore';
import { Button, Card, Badge, Textarea } from '../ui/primitives';
import { cn } from '../ui/cn';
import { BlocksRenderer } from '../chat/blocks';
import { parseAssistantContent } from '../chat/blockUtils';
import A2AMessageBubble, { A2AAgentBubble, A2AStreamingIndicator } from './A2AMessageBubble';

export default function A2AConversationView({ convId, onBack }) {
  const [conv, setConv] = useState(null);
  const [a2aMessages, setA2aMessages] = useState([]);
  const [takeoverText, setTakeoverText] = useState('');

  const a2aLiveBlocks = useStore((s) => s.a2aLiveBlocks);
  const a2aIsStreaming = useStore((s) => s.a2aIsStreaming);
  const a2aRemoteLiveBlocks = useStore((s) => s.a2aRemoteLiveBlocks);
  const a2aRemoteIsStreaming = useStore((s) => s.a2aRemoteIsStreaming);
  const storeA2aMessages = useStore((s) => s.a2aMessages);
  const setActiveA2ASession = useStore((s) => s.setActiveA2ASession);

  const scrollRef = useRef(null);
  const stickToBottomRef = useRef(true);
  const userScrolledRef = useRef(false);

  const loadConversation = useCallback(async () => {
    if (!convId) return;
    try {
      const data = await api.getA2AConversation(convId);
      setConv(data.conversation);
      setA2aMessages(data.messages || []);
      if (data.conversation?.local_session_id) {
        setActiveA2ASession(data.conversation.local_session_id);
      }
    } catch {}
  }, [convId, setActiveA2ASession]);

  useEffect(() => {
    loadConversation();
    return () => setActiveA2ASession(null);
  }, [loadConversation, setActiveA2ASession]);

  // 当对话处于 active 状态时，定时轮询刷新消息列表
  // 解决离开页面后重新进入时丢失中间消息的问题
  useEffect(() => {
    if (!conv || !conv.local_session_id) return;
    const isActive = conv.status === 'active' || conv.status === 'paused';
    if (!isActive) return;

    const interval = setInterval(() => {
      useStore.getState().refreshA2AMessages?.();
    }, 3000);

    return () => clearInterval(interval);
  }, [conv?.status, conv?.local_session_id]);

  // 重新进入时也刷新 A2A 对话元数据（轮次/状态等）
  useEffect(() => {
    if (!convId || !conv) return;
    const isActive = conv.status === 'active' || conv.status === 'paused';
    if (!isActive) return;

    const interval = setInterval(async () => {
      try {
        const data = await api.getA2AConversation(convId);
        if (data.conversation) {
          setConv(data.conversation);
        }
      } catch {}
    }, 5000);

    return () => clearInterval(interval);
  }, [convId, conv?.status]);

  useEffect(() => {
    const unsub = wsClient.on((msg) => {
      if (msg.event === 'a2a_message' && convId) {
        try {
          const m = JSON.parse(msg.data);
          if (m.conversation_id === convId) {
            setA2aMessages((prev) => {
              if (prev.find(p => p.id === m.id)) return prev;
              return [...prev, m];
            });
            if (m.sender === 'remote' && conv?.local_session_id) {
              setTimeout(() => {
                api.listMessages(conv.local_session_id).then(msgs => {
                  useStore.getState().refreshA2AMessages?.();
                }).catch(() => {});
              }, 500);
            }
          }
        } catch {}
      }
      if (msg.event === 'a2a_status_change') {
        try {
          const d = JSON.parse(msg.data);
          if (d.id === convId) {
            setConv((c) => c ? { ...c, status: d.status } : c);
            if (d.session_id && !conv?.local_session_id) {
              setConv((c) => c ? { ...c, local_session_id: d.session_id } : c);
              setActiveA2ASession(d.session_id);
            }
          }
        } catch {}
      }
      if (msg.event === 'a2a_turn_start') {
        try {
          const d = JSON.parse(msg.data);
          if (d.id === convId && d.session_id) {
            setActiveA2ASession(d.session_id);
          }
        } catch {}
      }
    });
    return unsub;
  }, [convId, conv?.local_session_id, setActiveA2ASession]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    stickToBottomRef.current = atBottom;
    userScrolledRef.current = !atBottom;
  }, []);

  const scrollToBottom = useCallback(() => {
    const el = scrollRef.current;
    if (!el || userScrolledRef.current) return;
    requestAnimationFrame(() => {
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
    });
  }, []);

  const localAgentName = useMemo(() => {
    if (!conv) return '己方 Agent';
    const agents = useStore.getState().agents || [];
    const agent = agents.find(a => a.id === conv.local_agent_id);
    return agent?.name || '己方 Agent';
  }, [conv]);

  const remoteAgentName = conv?.remote_agent_name || '对方 Agent';

  // 将 session 消息（user/assistant）映射为 A2A 语义：
  // user 消息 = 远端 Agent 的输入（对方的回复被注入为 user）
  // assistant 消息 = 己方 Agent 的回复
  // 发起方（initiated_by=local）的第一条 user 消息是系统初始指令，需跳过
  // 接收方（initiated_by=remote）的第一条 user 消息是对方的真实消息，需展示
  const isInitiator = conv?.initiated_by === 'local';
  const displayItems = useMemo(() => {
    const items = [];

    if (conv?.local_session_id && storeA2aMessages.length > 0) {
      storeA2aMessages.forEach((m, idx) => {
        if (m.role === 'user') {
          if (idx === 0 && isInitiator) return;
          items.push({ type: 'remote_agent', message: m, agentName: remoteAgentName });
        } else {
          items.push({ type: 'local_agent', message: m, agentName: localAgentName });
        }
      });
    } else if (a2aMessages.length > 0) {
      a2aMessages.forEach(m => {
        items.push({ type: 'a2a_message', message: m });
      });
    }

    // 对方 Agent 正在流式输出（跨实例转发）— 优先展示，严格一来一回
    if (a2aRemoteIsStreaming && a2aRemoteLiveBlocks?.length > 0) {
      items.push({ type: 'remote_live', liveBlocks: a2aRemoteLiveBlocks });
    } else if (a2aRemoteIsStreaming) {
      items.push({ type: 'remote_connecting' });
    }

    // 己方 Agent 正在流式输出（仅在对方不再流式输出时才展示）
    const showLocalStreaming = a2aIsStreaming && !a2aRemoteIsStreaming;
    if (showLocalStreaming && a2aLiveBlocks.length > 0) {
      items.push({ type: 'local_live', liveBlocks: a2aLiveBlocks });
    } else if (showLocalStreaming && a2aLiveBlocks.length === 0) {
      items.push({ type: 'local_connecting' });
    }

    return items;
  }, [storeA2aMessages, a2aMessages, a2aLiveBlocks, a2aIsStreaming,
      a2aRemoteLiveBlocks, a2aRemoteIsStreaming,
      conv?.local_session_id, isInitiator, localAgentName, remoteAgentName]);

  useEffect(() => {
    if (stickToBottomRef.current) scrollToBottom();
  }, [displayItems, scrollToBottom]);

  const handlePause = async () => {
    if (!convId) return;
    await api.pauseA2AConversation(convId);
  };

  const handleTerminate = async () => {
    if (!convId) return;
    await api.terminateA2AConversation(convId);
  };

  const handleTakeover = async () => {
    if (!convId || !takeoverText.trim()) return;
    await api.takeoverA2AConversation(convId, takeoverText.trim());
    setTakeoverText('');
  };

  const handleApprove = async (approved) => {
    if (!convId) return;
    await api.approveA2AConversation(convId, approved);
  };

  const anyStreaming = a2aIsStreaming || a2aRemoteIsStreaming;

  return (
    <div className="flex-1 flex min-h-0">
      <div className="flex-1 flex flex-col min-h-0">
        {/* 顶部栏 */}
        <div className="h-12 px-4 flex items-center gap-3 border-b border-[color:var(--line)] shrink-0">
          {onBack && (
            <Button variant="ghost" size="sm" onClick={onBack}>
              <ArrowLeft size={14} />
            </Button>
          )}
          <span className="text-sm font-medium text-[color:var(--text)]">{conv?.topic || 'Agent 对话'}</span>
          <StatusBadge status={conv?.status} />
          {conv?.status === 'active' && anyStreaming && (
            <span className="text-[10px] text-[color:var(--accent)] animate-pulse ml-1">
              {a2aIsStreaming ? `${localAgentName} 正在思考...` : `${remoteAgentName} 正在思考...`}
            </span>
          )}
          <div className="flex-1" />
          <div className="flex items-center gap-3 text-[10px] text-[color:var(--text-faint)]">
            <span className="flex items-center gap-1">
              <span className="w-2 h-2 rounded-full bg-[color:var(--accent)]" /> {localAgentName}
            </span>
            <span className="flex items-center gap-1">
              <span className="w-2 h-2 rounded-full bg-violet-500" /> {remoteAgentName}
            </span>
            <span className="ml-2">轮次 {conv?.current_round || 0}/{conv?.max_rounds || 10}</span>
          </div>
        </div>

        {/* 消息列表 */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto scrollable px-6 pb-2" onScroll={handleScroll}>
          <div className="max-w-3xl mx-auto py-6">
            {displayItems.length === 0 && (
              <div className="flex flex-col items-center justify-center py-16 text-center">
                <MessageSquare size={32} className="text-[color:var(--text-faint)] mb-3" />
                <p className="text-sm text-[color:var(--text-soft)]">
                  {conv?.status === 'pending_remote' ? '等待对方接受...' :
                   conv?.status === 'pending_incoming' ? '等待选择 Agent 接受...' :
                   '对话即将开始...'}
                </p>
              </div>
            )}
            {displayItems.map((item, i) => {
              if (item.type === 'local_live') {
                return (
                  <div key="local-live" className="enter-up">
                    <A2AAgentBubble live liveBlocks={item.liveBlocks} isLocal agentName={localAgentName} />
                  </div>
                );
              }
              if (item.type === 'local_connecting') {
                return <A2AStreamingIndicator key="local-conn" agentName={localAgentName} isLocal />;
              }
              if (item.type === 'remote_live') {
                return (
                  <div key="remote-live" className="enter-up">
                    <A2AAgentBubble live liveBlocks={item.liveBlocks} isLocal={false} agentName={remoteAgentName} />
                  </div>
                );
              }
              if (item.type === 'remote_connecting') {
                return <A2AStreamingIndicator key="remote-conn" agentName={remoteAgentName} isLocal={false} />;
              }
              if (item.type === 'local_agent') {
                return (
                  <div key={item.message.id || `la-${i}`} className="enter-up">
                    <A2AAgentBubble message={item.message} isLocal agentName={item.agentName} />
                  </div>
                );
              }
              if (item.type === 'remote_agent') {
                return (
                  <div key={item.message.id || `ra-${i}`} className="enter-up">
                    <A2AAgentBubble message={item.message} isLocal={false} agentName={item.agentName} />
                  </div>
                );
              }
              // a2a_message fallback（旧格式 + 跨实例消息）
              return (
                <div key={item.message.id || `am-${i}`} className="enter-up">
                  <A2AMessageBubble message={item.message} />
                </div>
              );
            })}
          </div>
        </div>

        {/* 底部操作栏 */}
        <div className="shrink-0 border-t border-[color:var(--line)] p-3 space-y-2">
          {conv?.status === 'active' && (
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={handlePause}>
                <Pause size={14} /> 暂停
              </Button>
              <Button variant="ghost" size="sm" onClick={handleTerminate}>
                <XCircle size={14} className="text-red-400" /> 终止
              </Button>
            </div>
          )}
          {(conv?.status === 'paused' || conv?.status === 'active') && (
            <div className="flex gap-2">
              <Textarea
                className="flex-1 text-sm"
                rows={1}
                placeholder="手动接管发送消息..."
                value={takeoverText}
                onChange={(e) => setTakeoverText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleTakeover(); }
                }}
              />
              <Button variant="primary" size="sm" onClick={handleTakeover} disabled={!takeoverText.trim()}>
                <Hand size={14} /> 发送
              </Button>
            </div>
          )}
          {conv?.status === 'pending_approval' && (
            <div className="flex items-center gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20">
              <span className="text-sm text-[color:var(--text)]">对话已完成，等待您的审批</span>
              <div className="ml-auto flex gap-2">
                <Button variant="primary" size="sm" onClick={() => handleApprove(true)}>
                  <CheckCircle size={14} /> 批准
                </Button>
                <Button variant="ghost" size="sm" onClick={() => handleApprove(false)}>
                  <XCircle size={14} /> 拒绝
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 右侧：信息面板 */}
      <aside className="w-[280px] shrink-0 border-l border-[color:var(--line)] bg-[color:var(--bg-elev)]/50 p-4 overflow-auto scrollable space-y-4">
        {/* Agent 信息 */}
        <div className="space-y-3">
          <div className="flex items-center gap-2 p-2 rounded-lg bg-[color:var(--bg-soft)]">
            <div className="w-6 h-6 rounded-full bg-gradient-to-br from-[color:var(--accent)] to-[#5e8bff] flex items-center justify-center">
              <Sparkles size={10} className="text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[10px] text-[color:var(--accent)] font-medium">己方</div>
              <div className="text-xs text-[color:var(--text)] truncate">{localAgentName}</div>
            </div>
          </div>
          <div className="flex items-center gap-2 p-2 rounded-lg bg-[color:var(--bg-soft)]">
            <div className="w-6 h-6 rounded-full bg-gradient-to-br from-violet-500 to-fuchsia-500 flex items-center justify-center">
              <Shield size={10} className="text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[10px] text-violet-600 dark:text-violet-400 font-medium">对方</div>
              <div className="text-xs text-[color:var(--text)] truncate">{remoteAgentName}</div>
            </div>
          </div>
        </div>

        <div className="space-y-3">
          <InfoRow icon={Target} label="目标" value={conv?.goal || '—'} />
          <InfoRow icon={Clock} label="轮次" value={`${conv?.current_round || 0} / ${conv?.max_rounds || 10}`} />
          <InfoRow icon={MessageSquare} label="发起方" value={conv?.initiated_by === 'local' ? '己方' : '对方'} />
        </div>

        {/* 摘要 — 使用 Markdown 渲染 */}
        {conv?.summary && (
          <div className="space-y-1.5">
            <div className="text-xs font-medium text-[color:var(--text-soft)] flex items-center gap-1.5">
              <BookOpen size={12} /> 摘要
            </div>
            <div className="text-xs text-[color:var(--text)] bg-[color:var(--bg-soft)] p-2.5 rounded-lg prose-a2a">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{conv.summary}</ReactMarkdown>
            </div>
          </div>
        )}

        {/* 关键决策 — 结构化卡片 + Markdown */}
        {conv?.decisions_json && conv.decisions_json !== '[]' && (
          <div className="space-y-1.5">
            <div className="text-xs font-medium text-[color:var(--text-soft)] flex items-center gap-1.5">
              <FileText size={12} /> 关键决策
            </div>
            {(() => {
              try {
                const decisions = JSON.parse(conv.decisions_json);
                return decisions.map((d, i) => (
                  <div key={i} className="p-2.5 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200/50 dark:border-green-800/30">
                    <div className="text-xs text-[color:var(--text)] prose-a2a">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{d.content}</ReactMarkdown>
                    </div>
                  </div>
                ));
              } catch { return null; }
            })()}
          </div>
        )}
      </aside>
    </div>
  );
}

export function StatusBadge({ status }) {
  const map = {
    active: { label: '进行中', cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300' },
    paused: { label: '已暂停', cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' },
    completed: { label: '已完成', cls: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' },
    terminated: { label: '已终止', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' },
    pending_approval: { label: '待审批', cls: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300' },
    pending_remote: { label: '等待对方', cls: 'bg-gray-100 text-gray-600 dark:bg-gray-700/30 dark:text-gray-300' },
    pending_incoming: { label: '待接受', cls: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300' },
    failed: { label: '失败', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' },
    rejected: { label: '已拒绝', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' },
  };
  const info = map[status] || { label: status || '未知', cls: 'bg-gray-100 text-gray-600' };
  return <span className={cn('text-[10px] px-2 py-0.5 rounded-full font-medium', info.cls)}>{info.label}</span>;
}

function InfoRow({ icon: Icon, label, value }) {
  return (
    <div className="flex items-start gap-2">
      <Icon size={14} className="text-[color:var(--text-faint)] mt-0.5 shrink-0" />
      <div>
        <div className="text-[10px] text-[color:var(--text-faint)]">{label}</div>
        <div className="text-xs text-[color:var(--text)]">{value}</div>
      </div>
    </div>
  );
}
