import { useEffect, useRef } from 'react';
import { useStore } from '../state/useStore';
import { UserBubble, AssistantBubble } from './Bubble';
import { Sparkles } from 'lucide-react';

export function MessageList() {
  const messages = useStore((s) => s.messages);
  const liveBlocks = useStore((s) => s.liveBlocks);
  const isStreaming = useStore((s) => s.isStreaming);
  const activeProfile = useStore((s) => s.activeProfile);
  const ref = useRef(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
  }, [messages, liveBlocks, isStreaming]);

  if (messages.length === 0 && !isStreaming) {
    return (
      <div ref={ref} className="flex-1 overflow-y-auto scrollable">
        <Empty profileName={activeProfile?.name || activeProfile?.model} />
      </div>
    );
  }

  return (
    <div ref={ref} className="flex-1 overflow-y-auto scrollable px-6 pb-2">
      <div className="max-w-3xl mx-auto py-6">
        {messages.map((m) => (
          <div key={m.id} className="enter-up">
            {m.role === 'user'
              ? <UserBubble content={m.content} />
              : <AssistantBubble message={m} />}
          </div>
        ))}
        {isStreaming && liveBlocks.length > 0 && (
          <div className="enter-up"><AssistantBubble live liveBlocks={liveBlocks} /></div>
        )}
        {isStreaming && liveBlocks.length === 0 && (
          <div className="flex justify-start my-3 enter-up">
            <div className="assistant-bubble flex items-center gap-2 text-[color:var(--text-soft)]">
              <span className="w-1.5 h-1.5 rounded-full bg-[color:var(--accent)] animate-breathe" />
              正在连接灵犀…
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Empty({ profileName }) {
  const examples = [
    '帮我把这周的会议纪要整理成行动项',
    '解释一下 transformer 的注意力机制',
    '写一个 Python 脚本批量重命名图片',
    '把这段中文翻译成地道的英文',
  ];
  const sendMessage = useStore((s) => s.sendMessage);
  return (
    <div className="h-full flex flex-col items-center justify-center px-6 py-10 text-center">
      <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-accent to-accent-700 text-white flex items-center justify-center shadow-glow mb-5 pulse-ring">
        <Sparkles size={30} />
      </div>
      <h2 className="text-3xl font-semibold tracking-tight">
        你好，我是<span className="text-gradient">灵犀</span>
      </h2>
      <p className="mt-2 text-[color:var(--text-soft)]">
        {profileName ? `当前接入：${profileName}` : '随时为你查信息、写内容、整理思路'}
      </p>
      <div className="mt-8 grid grid-cols-1 sm:grid-cols-2 gap-3 w-full max-w-2xl">
        {examples.map((q, i) => (
          <button
            key={q}
            style={{ animationDelay: `${i * 60}ms` }}
            className="surface surface-hover text-left px-4 py-3 hover:border-[color:var(--accent)] enter-up"
            onClick={() => sendMessage({ message: q })}
          >
            <div className="text-sm">{q}</div>
          </button>
        ))}
      </div>
    </div>
  );
}
