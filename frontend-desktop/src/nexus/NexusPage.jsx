import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Radar, Users, MessageSquare, RefreshCw, UserPlus, Sparkles, Wifi,
  Check, X, Trash2, Clock, Target,
  ArrowLeft, Globe, Zap, Activity, Radio, Shield, Link2,
} from 'lucide-react';
import { api, wsClient } from '../api/client';
import { Button, Card, Badge, Select } from '../ui/primitives';
import { cn } from '../ui/cn';
import A2AConversationView, { StatusBadge } from './A2AConversationView';
import StartA2AModal from './StartA2AModal';

const TABS = [
  { id: 'network', label: '发现', icon: Radar },
  { id: 'contacts', label: '联系人', icon: Users },
  { id: 'conversations', label: '对话', icon: MessageSquare },
];

/* ─── 子组件 ─────────────────────────────────────────────────────── */

const EmptyState = ({ icon: Icon, title, subtitle }) => (
  <div className="text-center py-16 space-y-4">
    <div className="w-14 h-14 mx-auto rounded-2xl bg-[color:var(--bg-soft)] flex items-center justify-center">
      <Icon size={24} className="text-[color:var(--text-faint)]" />
    </div>
    <p className="text-sm font-medium text-[color:var(--text-soft)]">{title}</p>
    <p className="text-xs text-[color:var(--text-faint)]">{subtitle}</p>
  </div>
);

const NetworkPanel = ({ peers, loading, onRefresh, onConnect, isAlreadyConnected }) => (
  <div className="space-y-4">
    <div className="flex items-center justify-between">
      <div className="text-xs font-medium text-[color:var(--text-soft)]">发现的节点 ({peers.length})</div>
      <Button variant="ghost" size="sm" onClick={onRefresh} disabled={loading}>
        <RefreshCw size={12} className={cn(loading && 'animate-spin')} />
        扫描
      </Button>
    </div>

    {peers.length === 0 ? (
      <EmptyState icon={Wifi} title="未发现节点" subtitle="确保其他设备已开启「对外可见」模式" />
    ) : (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        {peers.map((peer) => {
          let agents = [];
          try { agents = JSON.parse(peer.agents_json || '[]'); } catch {}
          const linked = isAlreadyConnected(peer);
          return (
            <motion.div key={peer.id} initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }}>
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <div className="relative">
                      <div className="w-9 h-9 rounded-lg bg-[color:var(--accent-soft)] flex items-center justify-center text-sm font-bold text-[color:var(--accent)]">
                        {(peer.nickname || '?')[0]}
                      </div>
                      <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full bg-emerald-400 border-2 border-[color:var(--bg-elev)]" />
                    </div>
                    <div>
                      <div className="text-sm font-medium text-[color:var(--text)]">{peer.nickname || '未知实例'}</div>
                      <div className="text-[10px] text-[color:var(--text-faint)]">{peer.host}:{peer.port}</div>
                    </div>
                  </div>
                  {linked ? (
                    <Badge variant="soft" className="text-[10px]">
                      <Link2 size={10} className="mr-1" /> 已建联
                    </Badge>
                  ) : (
                    <Button variant="ghost" size="sm" onClick={() => onConnect(peer)}>
                      <UserPlus size={12} /> 建联
                    </Button>
                  )}
                </div>
                {agents.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="text-[10px] text-[color:var(--text-faint)]">公开 Agent</div>
                    {agents.map((agent, idx) => (
                      <div key={idx} className="flex items-center gap-2 p-2 rounded-md bg-[color:var(--bg-soft)]">
                        <Sparkles size={10} className="text-[color:var(--accent)] shrink-0" />
                        <span className="text-[11px] font-medium text-[color:var(--text)]">{agent.name}</span>
                        <div className="flex gap-1 flex-wrap ml-auto">
                          {(agent.capability_tags || []).map((tag, ti) => (
                            <Badge key={ti} variant="soft" className="text-[9px] px-1.5 py-0">{tag}</Badge>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            </motion.div>
          );
        })}
      </div>
    )}
  </div>
);

const ContactsPanel = ({ pending, connected, onRespond, onDelete, onStartConversation }) => (
  <div className="space-y-6">
    {pending.length > 0 && (
      <div className="space-y-2">
        <div className="text-xs font-medium text-amber-600 dark:text-amber-400 flex items-center gap-2">
          <div className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
          待处理请求 ({pending.length})
        </div>
        {pending.map((c) => (
          <motion.div key={c.id} initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
            <Card className="p-3 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center text-sm font-bold text-amber-600 dark:text-amber-400">
                  {(c.nickname || '?')[0]}
                </div>
                <div>
                  <div className="text-sm font-medium text-[color:var(--text)]">{c.nickname || '未知'}</div>
                  <div className="text-[10px] text-[color:var(--text-faint)]">{c.host}:{c.port}</div>
                </div>
              </div>
              <div className="flex gap-2">
                <Button variant="primary" size="sm" onClick={() => onRespond(c.id, true)}>
                  <Check size={14} /> 同意
                </Button>
                <Button variant="ghost" size="sm" onClick={() => onRespond(c.id, false)}>
                  <X size={14} /> 拒绝
                </Button>
              </div>
            </Card>
          </motion.div>
        ))}
      </div>
    )}
    {connected.length > 0 ? (
      <div className="space-y-2">
        <div className="text-xs font-medium text-[color:var(--text-soft)]">已建联 ({connected.length})</div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
          {connected.map((c) => (
            <Card key={c.id} className="p-3 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="relative">
                  <div className="w-8 h-8 rounded-lg bg-emerald-100 dark:bg-emerald-900/30 flex items-center justify-center text-sm font-bold text-emerald-600 dark:text-emerald-400">
                    {(c.nickname || '?')[0]}
                  </div>
                  <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full bg-emerald-400 border-2 border-[color:var(--bg-elev)]" />
                </div>
                <div>
                  <div className="text-sm font-medium text-[color:var(--text)]">{c.nickname || '未知'}</div>
                  <div className="text-[10px] text-[color:var(--text-faint)]">{c.host}:{c.port}</div>
                </div>
              </div>
              <div className="flex gap-1">
                <Button variant="ghost" size="sm" onClick={() => onStartConversation(c)} title="发起对话">
                  <MessageSquare size={14} />
                </Button>
                <Button variant="ghost" size="sm" onClick={() => onDelete(c.id)} title="解除建联">
                  <Trash2 size={14} className="text-red-400" />
                </Button>
              </div>
            </Card>
          ))}
        </div>
      </div>
    ) : (
      <EmptyState icon={Users} title="没有已建联的节点" subtitle="前往「发现」面板发起建联请求" />
    )}
  </div>
);

const ConversationsPanel = ({ conversations, onSelect }) => {
  if (conversations.length === 0) {
    return <EmptyState icon={MessageSquare} title="没有对话" subtitle="前往「联系人」面板，选择节点发起 Agent 对话" />;
  }
  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-[color:var(--text-soft)]">全部对话 ({conversations.length})</div>
      <div className="space-y-2">
        {conversations.map((c) => (
          <motion.div key={c.id} initial={{ opacity: 0, x: -8 }} animate={{ opacity: 1, x: 0 }}>
            <Card
              className="p-4 cursor-pointer hover:bg-[color:var(--bg-soft)] transition-all duration-200 group"
              onClick={() => onSelect(c.id)}
            >
              <div className="flex items-center justify-between">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    {c.status === 'active' && <div className="w-2 h-2 rounded-full bg-[color:var(--accent)] animate-pulse" />}
                    {c.status === 'pending_remote' && <div className="w-2 h-2 rounded-full bg-sky-400 animate-pulse" />}
                    {c.status === 'pending_incoming' && <div className="w-2 h-2 rounded-full bg-amber-400 animate-pulse" />}
                    <span className="text-sm font-medium text-[color:var(--text)] truncate">{c.topic || '未命名对话'}</span>
                    <StatusBadge status={c.status} />
                  </div>
                  <div className="text-[10px] text-[color:var(--text-faint)] ml-4">
                    与 {c.remote_peer_nickname || '对方'}{c.remote_agent_name ? ` / ${c.remote_agent_name}` : ''} · 第 {c.current_round}/{c.max_rounds} 轮
                  </div>
                </div>
                <ArrowLeft size={14} className="text-[color:var(--text-faint)] group-hover:text-[color:var(--accent)] rotate-180 transition" />
              </div>
            </Card>
          </motion.div>
        ))}
      </div>
    </div>
  );
};

/* ─── 主组件 ─────────────────────────────────────────────────────── */

export default function NexusPage() {
  const [activeTab, setActiveTab] = useState('network');
  const [peers, setPeers] = useState([]);
  const [contacts, setContacts] = useState([]);
  const [conversations, setConversations] = useState([]);
  const [peersLoading, setPeersLoading] = useState(false);
  const [activeConvId, setActiveConvId] = useState(null);
  const [startModalOpen, setStartModalOpen] = useState(false);
  const [selectedContact, setSelectedContact] = useState(null);
  const [connectNotif, setConnectNotif] = useState(null);
  const [convRequestNotif, setConvRequestNotif] = useState(null);
  const [convRequestAgentId, setConvRequestAgentId] = useState('');
  const [agentList, setAgentList] = useState([]);

  const refreshPeers = useCallback(async () => {
    setPeersLoading(true);
    try { setPeers(await api.listPeers() || []); } catch {}
    setPeersLoading(false);
  }, []);

  const refreshContacts = useCallback(async () => {
    try { setContacts(await api.listContacts() || []); } catch {}
  }, []);

  const refreshConversations = useCallback(async () => {
    try { setConversations(await api.listA2AConversations() || []); } catch {}
  }, []);

  // 初始加载 + 定时刷新（只刷新列表，不轮询详情）
  useEffect(() => {
    refreshPeers();
    refreshContacts();
    refreshConversations();
    const peerTimer = setInterval(refreshPeers, 15000);
    const convTimer = setInterval(refreshConversations, 15000);
    return () => { clearInterval(peerTimer); clearInterval(convTimer); };
  }, [refreshPeers, refreshContacts, refreshConversations]);

  // WebSocket 事件
  useEffect(() => {
    const unsub = wsClient.on((msg) => {
      if (msg.event === 'nexus_connect_request') {
        refreshContacts();
        try {
          const d = JSON.parse(msg.data);
          setConnectNotif(d);
          setActiveTab('contacts');
        } catch {}
      }
      if (msg.event === 'nexus_connect_response') {
        refreshContacts();
      }
      if (msg.event === 'a2a_conversation_request') {
        refreshConversations();
        try {
          const d = JSON.parse(msg.data);
          setConvRequestNotif(d);
          setConvRequestAgentId('');
          api.listAgents().then(setAgentList).catch(() => {});
          setActiveTab('conversations');
        } catch {}
      }
      if (msg.event === 'a2a_status_change') {
        refreshConversations();
      }
      if (msg.event === 'a2a_message') {
        refreshConversations();
      }
    });
    return unsub;
  }, [refreshContacts, refreshConversations]);

  const handleConnect = async (peer) => {
    try {
      await api.sendConnectRequest({
        peer_id: peer.id, nickname: peer.nickname,
        host: peer.host, port: peer.port,
      });
      refreshContacts();
    } catch {}
  };

  const handleRespond = async (id, accept) => {
    try { await api.respondConnect(id, accept); refreshContacts(); } catch {}
    setConnectNotif(null);
  };

  const handleDeleteContact = async (id) => {
    try { await api.deleteContact(id); refreshContacts(); } catch {}
  };

  const handleAcceptConvRequest = async () => {
    if (!convRequestNotif || !convRequestAgentId) return;
    try {
      await api.acceptRemoteConversation(convRequestNotif.id, Number(convRequestAgentId));
      setConvRequestNotif(null);
      refreshConversations();
      setActiveConvId(convRequestNotif.id);
    } catch {}
  };

  const handleRejectConvRequest = async () => {
    if (!convRequestNotif) return;
    try {
      await api.rejectRemoteConversation(convRequestNotif.id);
      setConvRequestNotif(null);
      refreshConversations();
    } catch {}
  };

  const handleConvCreated = (convId) => {
    refreshConversations();
    setActiveTab('conversations');
    setActiveConvId(convId);
  };

  const pendingContacts = contacts.filter(c => c.status === 'pending_incoming');
  const connected = contacts.filter(c => c.status === 'connected');
  const activeConvs = conversations.filter(c => c.status === 'active');

  const isAlreadyConnected = (peer) => {
    return contacts.some(c => c.peer_id === peer.id && (c.status === 'connected' || c.status === 'pending' || c.status === 'pending_incoming'));
  };

  const stats = [
    { label: '在线节点', value: peers.length, icon: Radio },
    { label: '已建联', value: connected.length, icon: Shield },
    { label: '活跃对话', value: activeConvs.length, icon: Zap },
    { label: '总对话', value: conversations.length, icon: Activity },
  ];

  // 对话详情视图 — 使用统一的 A2AConversationView（含流式 + 主聊天 Bubble）
  if (activeConvId) {
    return (
      <A2AConversationView
        convId={activeConvId}
        onBack={() => setActiveConvId(null)}
      />
    );
  }

  // 列表视图
  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-auto scrollable">
      <div className="relative z-10 flex-1 flex flex-col min-h-0">
        <div className="px-6 pt-6 pb-4 shrink-0">
          <div className="flex items-center gap-4 mb-6">
            <div className="w-10 h-10 rounded-xl bg-[color:var(--accent-soft)] flex items-center justify-center">
              <Globe size={22} className="text-[color:var(--accent)]" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-[color:var(--text)]">Nexus 网络</h1>
              <p className="text-xs text-[color:var(--text-faint)] mt-0.5">Agent 间通信与协作</p>
            </div>
          </div>

          <div className="grid grid-cols-4 gap-3 mb-6">
            {stats.map((s) => {
              const Icon = s.icon;
              return (
                <Card key={s.label} className="p-3 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-[color:var(--accent-soft)] flex items-center justify-center">
                    <Icon size={15} className="text-[color:var(--accent)]" />
                  </div>
                  <div>
                    <div className="text-lg font-bold text-[color:var(--text)]">{s.value}</div>
                    <div className="text-[10px] text-[color:var(--text-faint)]">{s.label}</div>
                  </div>
                </Card>
              );
            })}
          </div>

          <div className="flex gap-1 p-1 rounded-lg bg-[color:var(--bg-soft)] border border-[color:var(--line)] mb-4">
            {TABS.map((tab) => {
              const Icon = tab.icon;
              const active = activeTab === tab.id;
              const showContactBadge = tab.id === 'contacts' && pendingContacts.length > 0;
              const pendingIncoming = conversations.filter(c => c.status === 'pending_incoming').length;
              const showConvBadge = tab.id === 'conversations' && pendingIncoming > 0;
              return (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={cn(
                    'flex-1 flex items-center justify-center gap-2 py-2 px-3 rounded-md text-xs font-medium transition-all duration-200 relative',
                    active
                      ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent)] shadow-sm'
                      : 'text-[color:var(--text-soft)] hover:text-[color:var(--text)] hover:bg-[color:var(--bg-elev)]'
                  )}
                >
                  <Icon size={14} />
                  <span>{tab.label}</span>
                  {(showContactBadge || showConvBadge) && (
                    <span className="absolute -top-1 -right-1 w-4 h-4 rounded-full bg-red-500 text-white text-[9px] flex items-center justify-center font-bold animate-pulse">
                      {showContactBadge ? pendingContacts.length : pendingIncoming}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {/* 建联请求通知 */}
        <AnimatePresence>
          {connectNotif && (
            <motion.div
              initial={{ opacity: 0, y: -20, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -20, scale: 0.95 }}
              className="mx-6 mb-4 p-4 rounded-xl border border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/30 shadow-lg"
            >
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-amber-100 dark:bg-amber-800/50 flex items-center justify-center">
                  <UserPlus size={20} className="text-amber-600 dark:text-amber-400" />
                </div>
                <div className="flex-1">
                  <div className="text-sm font-semibold text-[color:var(--text)]">收到建联请求</div>
                  <div className="text-xs text-[color:var(--text-soft)] mt-0.5">
                    <span className="font-medium">{connectNotif.nickname || '未知实例'}</span> 请求与您建立连接
                  </div>
                </div>
                <div className="flex gap-2">
                  <Button variant="primary" size="sm" onClick={() => handleRespond(connectNotif.id, true)}>
                    <Check size={14} /> 同意
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => handleRespond(connectNotif.id, false)}>
                    <X size={14} /> 拒绝
                  </Button>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Agent 对话请求通知 — 改进版 */}
        <AnimatePresence>
          {convRequestNotif && (
            <motion.div
              initial={{ opacity: 0, y: -20, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -20, scale: 0.95 }}
              className="mx-6 mb-4 p-5 rounded-xl border border-blue-300 dark:border-blue-600 bg-blue-50 dark:bg-blue-900/30 shadow-lg"
            >
              <div className="space-y-4">
                <div className="flex items-center gap-3">
                  <div className="w-12 h-12 rounded-xl bg-blue-100 dark:bg-blue-800/50 flex items-center justify-center">
                    <MessageSquare size={24} className="text-blue-600 dark:text-blue-400" />
                  </div>
                  <div className="flex-1">
                    <div className="text-sm font-semibold text-[color:var(--text)]">收到 Agent 对话请求</div>
                    <div className="text-xs text-[color:var(--text-soft)] mt-0.5">
                      <span className="font-medium">{convRequestNotif.peer_nickname}</span> 的 <span className="font-medium">{convRequestNotif.agent_name}</span> 请求对话
                    </div>
                  </div>
                </div>

                {/* 详细信息 */}
                <div className="grid grid-cols-2 gap-3 p-3 rounded-lg bg-white/50 dark:bg-white/5">
                  <div>
                    <div className="text-[10px] text-[color:var(--text-faint)] mb-1">主题</div>
                    <div className="text-sm font-medium text-[color:var(--text)]">{convRequestNotif.topic || '—'}</div>
                  </div>
                  <div>
                    <div className="text-[10px] text-[color:var(--text-faint)] mb-1">目标</div>
                    <div className="text-sm text-[color:var(--text)]">{convRequestNotif.goal || '—'}</div>
                  </div>
                  <div>
                    <div className="text-[10px] text-[color:var(--text-faint)] mb-1">最大轮次</div>
                    <div className="text-sm text-[color:var(--text)]">{convRequestNotif.max_rounds || 10}</div>
                  </div>
                  <div>
                    <div className="text-[10px] text-[color:var(--text-faint)] mb-1">对方 Agent</div>
                    <div className="text-sm text-[color:var(--text)]">{convRequestNotif.agent_name || '—'}</div>
                  </div>
                </div>

                <div className="flex items-end gap-3">
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1.5 block">选择己方 Agent 参与对话</label>
                    <Select value={convRequestAgentId} onChange={(e) => setConvRequestAgentId(e.target.value)}>
                      <option value="">选择 Agent...</option>
                      {agentList.map((a) => (
                        <option key={a.id} value={a.id}>{a.name}</option>
                      ))}
                    </Select>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="primary" size="sm" onClick={handleAcceptConvRequest} disabled={!convRequestAgentId}>
                      <Check size={14} /> 接受对话
                    </Button>
                    <Button variant="ghost" size="sm" onClick={handleRejectConvRequest}>
                      <X size={14} /> 拒绝
                    </Button>
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <div className="flex-1 px-6 pb-6">
          <AnimatePresence mode="wait">
            {activeTab === 'network' && (
              <motion.div key="network" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
                <NetworkPanel peers={peers} loading={peersLoading} onRefresh={refreshPeers} onConnect={handleConnect} isAlreadyConnected={isAlreadyConnected} />
              </motion.div>
            )}
            {activeTab === 'contacts' && (
              <motion.div key="contacts" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
                <ContactsPanel
                  pending={pendingContacts} connected={connected}
                  onRespond={handleRespond} onDelete={handleDeleteContact}
                  onStartConversation={(c) => { setSelectedContact(c); setStartModalOpen(true); }}
                />
              </motion.div>
            )}
            {activeTab === 'conversations' && (
              <motion.div key="conversations" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
                <ConversationsPanel conversations={conversations} onSelect={setActiveConvId} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      <StartA2AModal
        open={startModalOpen}
        onClose={() => { setStartModalOpen(false); refreshConversations(); }}
        contact={selectedContact}
        onCreated={handleConvCreated}
      />
    </div>
  );
}
