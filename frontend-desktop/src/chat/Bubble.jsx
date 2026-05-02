import { useState, useCallback } from 'react';
import { Copy, Check } from 'lucide-react';
import { BlocksRenderer, UsageFooter } from './blocks';
import { parseAssistantContent } from './blockUtils';

function parseUserContent(content) {
  if (!content) return { text: '', images: [] };
  if (content[0] === '{') {
    try {
      const obj = JSON.parse(content);
      if (obj && (obj.text != null || Array.isArray(obj.images))) {
        return { text: obj.text || '', images: obj.images || [] };
      }
    } catch { /* fallthrough */ }
  }
  return { text: String(content), images: [] };
}

function extractTextFromBlocks(blocks) {
  return blocks
    .filter(b => b.type === 'text')
    .map(b => b.text)
    .join('\n\n')
    .trim();
}

export function UserBubble({ content }) {
  const { text, images } = parseUserContent(content);
  return (
    <div className="flex justify-end my-3">
      <div className="user-bubble">
        {images.length > 0 && (
          <div className={`grid gap-2 ${images.length === 1 ? 'grid-cols-1' : 'grid-cols-2'} ${text ? 'mb-2' : ''}`}>
            {images.map((src, i) => (
              <a key={i} href={src} target="_blank" rel="noreferrer" className="block">
                <img src={src} className="rounded-lg max-h-56 max-w-[240px] object-cover ring-1 ring-white/30 shadow-soft hover:scale-[1.02] transition" alt="" />
              </a>
            ))}
          </div>
        )}
        {text && <div>{text}</div>}
      </div>
    </div>
  );
}

export function AssistantBubble({ message, live = false, liveBlocks = null }) {
  const blocks = liveBlocks || parseAssistantContent(message?.content || '');
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(() => {
    const text = extractTextFromBlocks(blocks);
    if (!text) return;
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    });
  }, [blocks]);

  const hasText = blocks.some(b => b.type === 'text' && b.text?.trim());

  return (
    <div className="group/msg flex justify-start my-3">
      <div className="assistant-bubble relative">
        <BlocksRenderer blocks={blocks} live={live} />
        {!live && message?.usage && <UsageFooter usageJSON={message.usage} />}

        {!live && hasText && (
          <button
            onClick={handleCopy}
            className="absolute -right-1 top-1 opacity-0 group-hover/msg:opacity-100 transition
              p-1.5 rounded-md bg-[color:var(--bg-soft)] border border-[color:var(--line)]
              hover:border-[color:var(--accent)] hover:bg-[color:var(--bg-elev)]
              text-[color:var(--text-faint)] hover:text-[color:var(--accent)]"
            title="复制内容"
          >
            {copied ? <Check size={14} className="text-emerald-500" /> : <Copy size={14} />}
          </button>
        )}
      </div>
    </div>
  );
}
