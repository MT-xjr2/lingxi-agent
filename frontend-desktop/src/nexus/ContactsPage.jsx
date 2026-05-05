import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Users, Check, X, Trash2, MessageSquare, MessagesSquare } from 'lucide-react';
import { api, wsClient } from '../api/client';
import { useStore } from '../state/useStore';
import { Button, Card, Modal } from '../ui/primitives';
import StartA2AModal from './StartA2AModal';

export default function ContactsPage() {
  const [contacts, setContacts] = useState([]);
  const [startModalOpen, setStartModalOpen] = useState(false);
  const [selectedContact, setSelectedContact] = useState(null);
  const setView = useStore((s) => s.setView);

  const refresh = async () => {
    try {
      const data = await api.listContacts();
      setContacts(data || []);
    } catch {}
  };

  useEffect(() => {
    refresh();
  }, []);

  // 监听 WS 事件刷新
  useEffect(() => {
    const unsub = wsClient.on((msg) => {
      if (msg.event === 'nexus_connect_request' || msg.event === 'nexus_connect_response') {
        refresh();
      }
    });
    return unsub;
  }, []);

  const handleRespond = async (id, accept) => {
    try {
      await api.respondConnect(id, accept);
      refresh();
    } catch {}
  };

  const handleDelete = async (id) => {
    try {
      await api.deleteContact(id);
      refresh();
    } catch {}
  };

  const handleStartConversation = (contact) => {
    setSelectedContact(contact);
    setStartModalOpen(true);
  };

  const pending = contacts.filter(c => c.status === 'pending_incoming');
  const connected = contacts.filter(c => c.status === 'connected');
  const other = contacts.filter(c => c.status !== 'connected' && c.status !== 'pending_incoming');

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-[color:var(--accent-soft)] flex items-center justify-center">
            <Users size={20} className="text-[color:var(--accent)]" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-[color:var(--text)]">联系人</h1>
            <p className="text-sm text-[color:var(--text-soft)]">已建联的灵犀实例</p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={() => setView('a2a')}>
          <MessagesSquare size={14} />
          Agent 对话
        </Button>
      </div>

      {/* 待处理的建联请求 */}
      {pending.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-[color:var(--text-soft)]">待处理请求</h2>
          <AnimatePresence>
            {pending.map((c) => (
              <motion.div key={c.id} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                <Card className="p-3 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-8 h-8 rounded-lg bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center text-sm font-bold text-amber-600">
                      {(c.nickname || '?')[0]}
                    </div>
                    <div>
                      <div className="text-sm font-medium text-[color:var(--text)]">{c.nickname || '未知'}</div>
                      <div className="text-xs text-[color:var(--text-faint)]">{c.host}:{c.port}</div>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="primary" size="sm" onClick={() => handleRespond(c.id, true)}>
                      <Check size={14} /> 同意
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleRespond(c.id, false)}>
                      <X size={14} /> 拒绝
                    </Button>
                  </div>
                </Card>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}

      {/* 已建联 */}
      {connected.length > 0 ? (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-[color:var(--text-soft)]">已建联 ({connected.length})</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {connected.map((c) => (
              <Card key={c.id} className="p-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center text-sm font-bold text-green-600">
                    {(c.nickname || '?')[0]}
                  </div>
                  <div>
                    <div className="text-sm font-medium text-[color:var(--text)]">{c.nickname || '未知'}</div>
                    <div className="text-xs text-[color:var(--text-faint)]">{c.host}:{c.port}</div>
                  </div>
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" onClick={() => handleStartConversation(c)} title="发起对话">
                    <MessageSquare size={14} />
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => handleDelete(c.id)} title="解除建联">
                    <Trash2 size={14} className="text-red-400" />
                  </Button>
                </div>
              </Card>
            ))}
          </div>
        </div>
      ) : (
        <div className="text-center py-12 space-y-3">
          <div className="w-14 h-14 mx-auto rounded-2xl bg-[color:var(--bg-soft)] flex items-center justify-center">
            <Users size={24} className="text-[color:var(--text-faint)]" />
          </div>
          <p className="text-[color:var(--text-soft)]">还没有已建联的联系人</p>
          <p className="text-xs text-[color:var(--text-faint)]">前往"附近"面板发起建联请求</p>
        </div>
      )}

      {/* 其他状态 */}
      {other.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-[color:var(--text-faint)]">其他</h2>
          {other.map((c) => (
            <Card key={c.id} className="p-2 flex items-center justify-between opacity-60">
              <span className="text-xs text-[color:var(--text-soft)]">{c.nickname} — {c.status}</span>
              <Button variant="ghost" size="sm" onClick={() => handleDelete(c.id)}>
                <Trash2 size={12} />
              </Button>
            </Card>
          ))}
        </div>
      )}

      <StartA2AModal
        open={startModalOpen}
        onClose={() => setStartModalOpen(false)}
        contact={selectedContact}
      />
    </div>
  );
}
