import { useEffect, useRef, useCallback, useMemo } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useStore } from '../state/useStore';
import { UserBubble, AssistantBubble } from './Bubble';
import { Sparkles } from 'lucide-react';

const VIRTUALIZE_THRESHOLD = 60;

export function MessageList() {
  const messages = useStore((s) => s.messages);
  const liveBlocks = useStore((s) => s.liveBlocks);
  const isStreaming = useStore((s) => s.isStreaming);
  const activeProfile = useStore((s) => s.activeProfile);
  const scrollRef = useRef(null);
  const stickToBottomRef = useRef(true);
  const userScrolledRef = useRef(false);

  const items = useMemo(() => {
    const list = messages.map(m => ({ type: 'message', message: m }));
    if (isStreaming && liveBlocks.length > 0) {
      list.push({ type: 'live', liveBlocks });
    } else if (isStreaming && liveBlocks.length === 0) {
      list.push({ type: 'connecting' });
    }
    return list;
  }, [messages, liveBlocks, isStreaming]);

  const shouldVirtualize = items.length > VIRTUALIZE_THRESHOLD;

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

  useEffect(() => {
    if (stickToBottomRef.current) scrollToBottom();
  }, [messages, liveBlocks, isStreaming, scrollToBottom]);

  if (items.length === 0) {
    return (
      <div ref={scrollRef} className="flex-1 overflow-y-auto scrollable">
        <Empty profileName={activeProfile?.name || activeProfile?.model} />
      </div>
    );
  }

  if (shouldVirtualize) {
    return <VirtualizedList items={items} scrollRef={scrollRef} onScroll={handleScroll} />;
  }

  return (
    <div ref={scrollRef} className="flex-1 overflow-y-auto scrollable px-6 pb-2" onScroll={handleScroll}>
      <div className="max-w-3xl mx-auto py-6">
        {items.map((item, i) => (
          <MessageItem key={item.message?.id || `special-${i}`} item={item} />
        ))}
      </div>
    </div>
  );
}

function VirtualizedList({ items, scrollRef, onScroll }) {
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 120,
    overscan: 8,
  });

  return (
    <div ref={scrollRef} className="flex-1 overflow-y-auto scrollable px-6 pb-2" onScroll={onScroll}>
      <div className="max-w-3xl mx-auto py-6 relative" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map(row => (
          <div
            key={row.key}
            data-index={row.index}
            ref={virtualizer.measureElement}
            className="absolute left-0 right-0"
            style={{ transform: `translateY(${row.start}px)` }}
          >
            <MessageItem item={items[row.index]} />
          </div>
        ))}
      </div>
    </div>
  );
}

function MessageItem({ item }) {
  if (item.type === 'live') {
    return <div className="enter-up"><AssistantBubble live liveBlocks={item.liveBlocks} /></div>;
  }
  if (item.type === 'connecting') {
    return (
      <div className="flex justify-start my-3 enter-up">
        <div className="assistant-bubble flex items-center gap-2 text-[color:var(--text-soft)]">
          <span className="w-1.5 h-1.5 rounded-full bg-[color:var(--accent)] animate-breathe" />
          正在连接灵犀…
        </div>
      </div>
    );
  }
  const m = item.message;
  return (
    <div className="enter-up">
      {m.role === 'user' ? <UserBubble content={m.content} /> : <AssistantBubble message={m} />}
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
