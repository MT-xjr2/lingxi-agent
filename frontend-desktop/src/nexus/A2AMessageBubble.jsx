import { useState, useCallback } from 'react';
import { motion } from 'framer-motion';
import {
  FileText, CheckCircle2, AlertTriangle, MessageCircle,
  Sparkles, Shield, Copy, Check, User,
} from 'lucide-react';
import { BlocksRenderer } from '../chat/blocks';
import { parseAssistantContent } from '../chat/blockUtils';
import { cn } from '../ui/cn';

function LocalAgentAvatar({ name }) {
  return (
    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[color:var(--accent)] to-[#5e8bff] flex items-center justify-center shrink-0 shadow-sm self-start mt-0.5"
      title={`己方: ${name || 'Agent'}`}>
      <Sparkles size={14} className="text-white" />
    </div>
  );
}

function RemoteAgentAvatar({ name }) {
  return (
    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-violet-500 to-fuchsia-500 flex items-center justify-center shrink-0 shadow-sm self-start mt-0.5"
      title={`对方: ${name || 'Agent'}`}>
      <Shield size={14} className="text-white" />
    </div>
  );
}

function HumanAvatar() {
  return (
    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-amber-500 to-orange-500 flex items-center justify-center shrink-0 shadow-sm self-start mt-0.5">
      <User size={14} className="text-white" />
    </div>
  );
}

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    if (!text) return;
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    });
  }, [text]);

  return (
    <button
      onClick={handleCopy}
      className="p-1 rounded-md text-[color:var(--text-faint)] hover:text-[color:var(--accent)]
        hover:bg-[color:var(--bg-soft)] transition opacity-0 group-hover/a2a:opacity-100"
      title="复制内容"
    >
      {copied ? <Check size={12} className="text-emerald-500" /> : <Copy size={12} />}
    </button>
  );
}

function StructuredCard({ message, icon: Icon, iconColor, label, labelColor, bgCls }) {
  const blocks = parseAssistantContent(message.content);
  const isLocal = message.sender === 'local';
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      className={cn('max-w-[85%] rounded-xl border p-3 space-y-2 group/a2a', bgCls)}
    >
      <div className="flex items-center gap-1.5">
        <Icon size={12} className={iconColor} />
        <span className={cn('text-[10px] font-semibold uppercase', labelColor)}>{label}</span>
        <span className="text-[10px] text-[color:var(--text-faint)] ml-auto flex items-center gap-1">
          {isLocal ? '己方' : '对方'} · {message.sender_agent_name}
          <CopyButton text={message.content} />
        </span>
      </div>
      <div className="text-sm text-[color:var(--text)]">
        <BlocksRenderer blocks={blocks} live={false} />
      </div>
    </motion.div>
  );
}

export default function A2AMessageBubble({ message }) {
  const isLocal = message.sender === 'local';
  const isHuman = message.sender === 'human';

  if (message.msg_type === 'proposal') {
    return (
      <StructuredCard
        message={message}
        icon={FileText} iconColor="text-blue-600" label="提案" labelColor="text-blue-600"
        bgCls="bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-700"
      />
    );
  }

  if (message.msg_type === 'decision') {
    return (
      <StructuredCard
        message={message}
        icon={CheckCircle2} iconColor="text-green-600" label="决策" labelColor="text-green-600"
        bgCls="bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-700"
      />
    );
  }

  if (message.msg_type === 'handoff') {
    return (
      <motion.div
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        className="max-w-[85%] mx-auto rounded-xl border p-3 space-y-2 bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-700"
      >
        <div className="flex items-center gap-1.5">
          <AlertTriangle size={12} className="text-amber-600" />
          <span className="text-[10px] font-semibold text-amber-600 uppercase">请求人类介入</span>
        </div>
        <div className="text-sm text-[color:var(--text)]">
          <BlocksRenderer blocks={parseAssistantContent(message.content)} live={false} />
        </div>
      </motion.div>
    );
  }

  if (message.msg_type === 'close') {
    return (
      <motion.div
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        className="max-w-[85%] mx-auto rounded-xl border p-3 space-y-2 bg-[color:var(--bg-soft)] border-[color:var(--line)]"
      >
        <div className="flex items-center gap-1.5">
          <MessageCircle size={12} className="text-[color:var(--text-soft)]" />
          <span className="text-[10px] font-semibold text-[color:var(--text-soft)] uppercase">对话结束</span>
        </div>
        <div className="text-sm text-[color:var(--text)]">
          <BlocksRenderer blocks={parseAssistantContent(message.content)} live={false} />
        </div>
      </motion.div>
    );
  }

  // 人类接管消息 — 靠右对齐
  if (isHuman) {
    return (
      <div className="flex justify-end gap-2.5 my-3 group/a2a">
        <div className="max-w-[75%] rounded-2xl px-4 py-2.5 text-sm bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 text-[color:var(--text)]">
          <div className="text-[10px] text-amber-600 dark:text-amber-400 font-medium mb-1 flex items-center gap-1">
            人类接管
            <CopyButton text={message.content} />
          </div>
          <BlocksRenderer blocks={parseAssistantContent(message.content)} live={false} />
        </div>
        <HumanAvatar />
      </div>
    );
  }

  // 普通 Agent 消息 — 用 BlocksRenderer 渲染 markdown
  const blocks = parseAssistantContent(message.content);

  return (
    <div className="flex justify-start gap-2.5 my-3 group/a2a">
      {isLocal ? (
        <LocalAgentAvatar name={message.sender_agent_name} />
      ) : (
        <RemoteAgentAvatar name={message.sender_agent_name} />
      )}
      <div className={cn(
        'max-w-[80%] rounded-2xl px-4 py-2.5 text-sm',
        isLocal
          ? 'assistant-bubble border-l-[3px] border-l-[color:var(--accent)]'
          : 'assistant-bubble border-l-[3px] border-l-violet-500'
      )}>
        <div className={cn(
          'text-[10px] font-medium mb-1.5 flex items-center gap-1.5',
          isLocal ? 'text-[color:var(--accent)]' : 'text-violet-600 dark:text-violet-400'
        )}>
          <span>{isLocal ? '己方' : '对方'}</span>
          <span className="text-[color:var(--text-faint)]">·</span>
          <span className="text-[color:var(--text-soft)]">{message.sender_agent_name}</span>
          <CopyButton text={typeof message.content === 'string' ? message.content : ''} />
        </div>
        <BlocksRenderer blocks={blocks} live={false} />
      </div>
    </div>
  );
}

export function A2AAgentBubble({ message, isLocal, agentName, live = false, liveBlocks = null }) {
  const blocks = liveBlocks || parseAssistantContent(message?.content || '');
  const displayName = agentName || '未知 Agent';

  return (
    <div className="flex justify-start gap-2.5 my-3 group/a2a">
      {isLocal ? (
        <LocalAgentAvatar name={displayName} />
      ) : (
        <RemoteAgentAvatar name={displayName} />
      )}
      <div className={cn(
        'max-w-[80%] rounded-2xl px-4 py-2.5 text-sm',
        isLocal
          ? 'assistant-bubble border-l-[3px] border-l-[color:var(--accent)]'
          : 'assistant-bubble border-l-[3px] border-l-violet-500',
        live && 'streaming-pulse'
      )}>
        <div className={cn(
          'text-[10px] font-medium mb-1.5 flex items-center gap-1.5',
          isLocal ? 'text-[color:var(--accent)]' : 'text-violet-600 dark:text-violet-400'
        )}>
          <span>{isLocal ? '己方' : '对方'}</span>
          <span className="text-[color:var(--text-faint)]">·</span>
          <span className="text-[color:var(--text-soft)]">{displayName}</span>
        </div>
        <BlocksRenderer blocks={blocks} live={live} />
        {!live && message?.usage && (
          <div className="mt-1 text-[10px] text-[color:var(--text-faint)]">
            {(() => { try { const u = typeof message.usage === 'string' ? JSON.parse(message.usage) : message.usage; return u?.model || ''; } catch { return ''; } })()}
          </div>
        )}
      </div>
    </div>
  );
}

export function A2AStreamingIndicator({ agentName, isLocal }) {
  return (
    <div className="flex justify-start gap-2.5 my-3 enter-up">
      {isLocal ? (
        <LocalAgentAvatar name={agentName} />
      ) : (
        <RemoteAgentAvatar name={agentName} />
      )}
      <div className={cn(
        'assistant-bubble thinking-shimmer flex items-center gap-3 text-[color:var(--text-soft)]',
        !isLocal && 'border-l-[3px] border-l-violet-500'
      )}>
        <div className="flex items-end gap-[3px] h-5">
          <span className="neural-bar" style={{ animationDelay: '0s' }} />
          <span className="neural-bar" style={{ animationDelay: '0.15s' }} />
          <span className="neural-bar" style={{ animationDelay: '0.3s' }} />
        </div>
        <span className="text-sm">
          <span className={cn(
            'text-[10px] font-medium mr-1.5',
            isLocal ? 'text-[color:var(--accent)]' : 'text-violet-600 dark:text-violet-400'
          )}>
            {isLocal ? '己方' : '对方'}
          </span>
          {agentName || 'Agent'} 正在思考...
        </span>
      </div>
    </div>
  );
}
