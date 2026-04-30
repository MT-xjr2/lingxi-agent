import { useState } from 'react';
import { useStore } from '../state/useStore';
import { Plus, MessageSquare, Trash2, Search } from 'lucide-react';
import { Input, cn } from './primitives';

export function SidebarSessions() {
  const sessions = useStore((s) => s.sessions);
  const activeId = useStore((s) => s.activeSessionId);
  const setActive = useStore((s) => s.setActiveSession);
  const createSession = useStore((s) => s.createSession);
  const deleteSession = useStore((s) => s.deleteSession);
  const setView = useStore((s) => s.setView);

  const [q, setQ] = useState('');
  const filtered = sessions.filter((s) => !q || (s.title || '').toLowerCase().includes(q.toLowerCase()));

  return (
    <div className="flex flex-col h-full p-3 gap-3">
      <button
        onClick={async () => { await createSession(); setView('chat'); }}
        className="flex items-center gap-2 px-3 h-10 rounded-lg bg-[color:var(--accent)] text-white hover:bg-accent-600 transition shadow-soft"
      >
        <Plus size={16} /> 新对话
      </button>
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[color:var(--text-faint)]" />
        <Input className="pl-8 h-9" placeholder="搜索对话…" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      <div className="flex-1 overflow-y-auto scrollable -mx-1 px-1 space-y-0.5">
        {filtered.map((s) => (
          <SessionItem
            key={s.id}
            session={s}
            active={s.id === activeId}
            onClick={() => { setActive(s.id); setView('chat'); }}
            onDelete={() => deleteSession(s.id)}
          />
        ))}
        {filtered.length === 0 && (
          <div className="px-3 py-6 text-xs text-[color:var(--text-faint)] text-center">暂无对话</div>
        )}
      </div>
    </div>
  );
}

function SessionItem({ session, active, onClick, onDelete }) {
  return (
    <div
      onClick={onClick}
      className={cn(
        'group flex items-center gap-2 px-2.5 py-2 rounded-lg cursor-pointer transition',
        active ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent)]' : 'hover:bg-[color:var(--bg-soft)] text-[color:var(--text)]',
      )}
    >
      <MessageSquare size={14} className="shrink-0 opacity-70" />
      <div className="flex-1 min-w-0">
        <div className="text-sm truncate">{session.title || '新对话'}</div>
        <div className="text-[11px] text-[color:var(--text-faint)] truncate">
          {session.message_count || 0} 条消息
        </div>
      </div>
      <button
        className="opacity-0 group-hover:opacity-100 transition text-[color:var(--text-faint)] hover:text-red-500 p-1"
        onClick={(e) => { e.stopPropagation(); if (confirm('删除该对话？')) onDelete(); }}
      >
        <Trash2 size={14} />
      </button>
    </div>
  );
}
