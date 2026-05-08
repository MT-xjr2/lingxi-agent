import { useState, useEffect, useCallback, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Search, Users, MessageSquare, RefreshCw, Sparkles, Wifi,
  Check, X, Globe, Radio, Radar, Send, Trash2,
} from 'lucide-react';
import { api, wsClient } from '../api/client';
import { Button, Card, Badge } from '../ui/primitives';
import { cn } from '../ui/cn';
import A2AConversationView, { StatusBadge } from './A2AConversationView';
import StartA2AModal from './StartA2AModal';

/* ─── 左侧边栏列表项组件 ──────────────────────────────────────── */

const Avatar = ({ name, online, avatar_url, size = 'md', color = 'accent' }) => {
  const sizeClass = size === 'sm' ? 'w-8 h-8 text-xs' : 'w-10 h-10 text-sm';
  const dotSize = size === 'sm' ? 'w-2 h-2' : 'w-2.5 h-2.5';
  const colorMap = {
    accent: 'bg-[color:var(--accent-soft)] text-[color:var(--accent)]',
    purple: 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400',
    emerald: 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400',
    amber: 'bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400',
  };
  return (
    <div className="relative shrink-0">
      {avatar_url ? (
        <img src={avatar_url} alt="" className={cn(sizeClass, 'rounded-xl object-cover')} />
      ) : (
        <div className={cn(sizeClass, 'rounded-xl flex items-center justify-center font-bold', colorMap[color] || colorMap.accent)}>
          {(name || '?')[0]}
        </div>
      )}
      {online !== undefined && (
        <div className={cn(
          'absolute -bottom-0.5 -right-0.5 rounded-full border-2 border-[color:var(--bg-elev)]',
          dotSize,
          online ? 'bg-emerald-400' : 'bg-gray-300 dark:bg-gray-600'
        )} />
      )}
    </div>
  );
};

const ConversationItem = ({ conv, active, onClick, onDelete }) => (
  <div
    onClick={onClick}
    className={cn(
      'w-full flex items-center gap-3 px-3 py-2.5 mx-2 rounded-lg transition-all duration-150 text-left group cursor-pointer',
      active
        ? 'bg-[color:var(--accent-soft)] border-l-2 border-[color:var(--accent)]'
        : 'hover:bg-[color:var(--bg-soft)] border-l-2 border-transparent'
    )}
  >
    <div className="relative shrink-0">
      <div className={cn(
        'w-10 h-10 rounded-xl flex items-center justify-center text-sm font-bold',
        conv.status === 'active' ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400'
          : 'bg-[color:var(--bg-soft)] text-[color:var(--text-faint)]'
      )}>
        <MessageSquare size={18} />
      </div>
      {conv.status === 'active' && (
        <div className="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full bg-emerald-400 border-2 border-[color:var(--bg-elev)] animate-pulse" />
      )}
    </div>
    <div className="flex-1 min-w-0">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-[color:var(--text)] truncate">{conv.topic || '未命名对话'}</span>
        <div className="flex items-center gap-1">
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(conv.id); }}
            className="hidden group-hover:flex w-5 h-5 items-center justify-center rounded text-[color:var(--text-faint)] hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition"
            title="删除对话"
          >
            <Trash2 size={11} />
          </button>
          <StatusBadge status={conv.status} />
        </div>
      </div>
      <div className="text-[10px] text-[color:var(--text-faint)] mt-0.5 truncate">
        与 {conv.remote_peer_nickname || '对方'}{conv.remote_agent_name ? ` / ${conv.remote_agent_name}` : ''}
      </div>
    </div>
  </div>
);

/* ─── 右侧：发现面板（在线 peers，可直接发起对话）──────────────── */

const DiscoveryPanel = ({ peers, wanPeers, wanStatus, loading, onRefresh, onRefreshWAN, onStartChat }) => (
  <div className="flex-1 flex flex-col min-h-0 overflow-auto scrollable p-6">
    <div className="flex items-center justify-between mb-6">
      <div>
        <h2 className="text-lg font-bold text-[color:var(--text)]">在线节点</h2>
        <p className="text-xs text-[color:var(--text-faint)] mt-0.5">发现附近和远程的灵犀实例，直接发起对话</p>
      </div>
      <div className="flex gap-2">
        <Button variant="ghost" size="sm" onClick={onRefresh} disabled={loading}>
          <RefreshCw size={12} className={cn(loading && 'animate-spin')} /> 扫描
        </Button>
      </div>
    </div>

    {/* LAN 节点 */}
    {peers.length > 0 && (
      <div className="mb-6">
        <div className="flex items-center gap-2 mb-3">
          <Radio size={12} className="text-blue-500" />
          <span className="text-xs font-medium text-[color:var(--text-soft)]">局域网 ({peers.length})</span>
        </div>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {peers.map((peer) => {
            let agents = [];
            try { agents = JSON.parse(peer.agents_json || '[]'); } catch {}
            return (
              <Card key={peer.id} className="p-4 space-y-2.5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <Avatar name={peer.nickname} online={true} />
                    <div>
                      <div className="text-sm font-medium text-[color:var(--text)]">{peer.nickname || '未知'}</div>
                      <div className="text-[10px] text-[color:var(--text-faint)]">{peer.host}:{peer.port}</div>
                    </div>
                  </div>
                  <Button variant="primary" size="sm" onClick={() => onStartChat(peer, 'lan')}>
                    <Send size={12} /> 对话
                  </Button>
                </div>
                {agents.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {agents.map((a, i) => (
                      <Badge key={i} variant="soft" className="text-[9px] gap-1"><Sparkles size={8} />{a.name}</Badge>
                    ))}
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      </div>
    )}

    {/* WAN 节点 */}
    <div>
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Globe size={12} className="text-purple-500" />
          <span className="text-xs font-medium text-[color:var(--text-soft)]">广域网 ({wanPeers.length})</span>
          <div className={cn('w-1.5 h-1.5 rounded-full', wanStatus?.connected ? 'bg-emerald-400' : 'bg-gray-300')} />
        </div>
        <Button variant="ghost" size="sm" onClick={onRefreshWAN} disabled={!wanStatus?.connected}>
          <RefreshCw size={11} /> 刷新
        </Button>
      </div>

      {!wanStatus?.enabled ? (
        <Card className="p-8 text-center">
          <Globe size={32} className="mx-auto text-[color:var(--text-faint)] mb-3" />
          <p className="text-sm text-[color:var(--text-soft)]">广域网未启用</p>
          <p className="text-xs text-[color:var(--text-faint)] mt-1">前往「设置 → Nexus 网络」启用</p>
        </Card>
      ) : wanPeers.length === 0 ? (
        <Card className="p-8 text-center">
          <Radar size={32} className="mx-auto text-[color:var(--text-faint)] mb-3" />
          <p className="text-sm text-[color:var(--text-soft)]">未发现远程节点</p>
          <p className="text-xs text-[color:var(--text-faint)] mt-1">等待其他用户上线</p>
        </Card>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {wanPeers.map((peer) => (
            <Card key={peer.instance_id} className="p-4 space-y-2.5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <Avatar name={peer.nickname} avatar_url={peer.avatar_url} online={true} color="purple" />
                  <div>
                    <div className="text-sm font-medium text-[color:var(--text)]">{peer.nickname || '远程用户'}</div>
                    <div className="flex items-center gap-1 text-[10px] text-[color:var(--text-faint)]">
                      <Globe size={8} /> {peer.platform || '未知'} {peer.device_name ? `· ${peer.device_name}` : ''}
                    </div>
                  </div>
                </div>
                <Button variant="primary" size="sm" onClick={() => onStartChat(peer, 'wan')}>
                  <Send size={12} /> 对话
                </Button>
              </div>
              {peer.agents && peer.agents.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {peer.agents.map((a, i) => (
                    <Badge key={i} variant="soft" className="text-[9px] gap-1"><Sparkles size={8} />{a.name}</Badge>
                  ))}
                </div>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>

    {peers.length === 0 && (!wanStatus?.enabled || wanPeers.length === 0) && (
      <Card className="p-12 text-center mt-4">
        <Wifi size={40} className="mx-auto text-[color:var(--text-faint)] mb-4 opacity-40" />
        <p className="text-sm font-medium text-[color:var(--text-soft)]">暂未发现任何节点</p>
        <p className="text-xs text-[color:var(--text-faint)] mt-2">确保其他设备已开启「对外可见」模式</p>
      </Card>
    )}
  </div>
);

/* ─── 右侧：空状态 ────────────────────────────────────────────── */

const EmptyMainPanel = () => (
  <div className="flex-1 flex flex-col items-center justify-center p-8 opacity-60">
    <div className="w-20 h-20 rounded-3xl bg-[color:var(--bg-soft)] flex items-center justify-center mb-6">
      <Globe size={36} className="text-[color:var(--text-faint)]" />
    </div>
    <h3 className="text-base font-semibold text-[color:var(--text-soft)]">Nexus 网络</h3>
    <p className="text-xs text-[color:var(--text-faint)] mt-2 text-center max-w-[240px]">
      选择左侧的对话或点击「发现」探索在线节点并发起对话
    </p>
  </div>
);

/* ─── 主组件 ──────────────────────────────────────────────────── */

export default function NexusPage() {
  const [peers, setPeers] = useState([]);
  const [conversations, setConversations] = useState([]);
  const [peersLoading, setPeersLoading] = useState(false);
  const [wanPeers, setWanPeers] = useState([]);
  const [wanStatus, setWanStatus] = useState(null);
  const [agentList, setAgentList] = useState([]);

  const [selected, setSelected] = useState('discovery');
  const [searchQuery, setSearchQuery] = useState('');

  const [acceptingConvId, setAcceptingConvId] = useState(null);
  const [selectedAgentId, setSelectedAgentId] = useState('');

  const [startModalOpen, setStartModalOpen] = useState(false);
  const [selectedPeer, setSelectedPeer] = useState(null);

  // ─── 数据加载 ─────────────────────────────────────────────────
  const refreshPeers = useCallback(async () => {
    setPeersLoading(true);
    try { setPeers(await api.listPeers() || []); } catch {}
    setPeersLoading(false);
  }, []);

  const refreshConversations = useCallback(async () => {
    try { setConversations(await api.listA2AConversations() || []); } catch {}
  }, []);

  const refreshWANPeers = useCallback(async () => {
    try { setWanPeers(await api.listWANPeers() || []); } catch {}
  }, []);

  const refreshWANStatus = useCallback(async () => {
    try { setWanStatus(await api.getWANStatus() || null); } catch {}
  }, []);

  useEffect(() => {
    refreshPeers();
    refreshConversations();
    refreshWANStatus();
    refreshWANPeers();
    api.listAgents().then(setAgentList).catch(() => {});
    const t1 = setInterval(refreshPeers, 15000);
    const t2 = setInterval(refreshConversations, 5000);
    const t3 = setInterval(refreshWANPeers, 20000);
    return () => { clearInterval(t1); clearInterval(t2); clearInterval(t3); };
  }, [refreshPeers, refreshConversations, refreshWANPeers, refreshWANStatus]);

  // ─── WebSocket 事件 ────────────────────────────────────────────
  useEffect(() => {
    const unsub = wsClient.on((msg) => {
      if (msg.event === 'a2a_conversation_request' || msg.event === 'a2a_conversation_invite' ||
          msg.event === 'a2a_status_change' || msg.event === 'a2a_message') {
        refreshConversations();
      }
      if (msg.event === 'wan_peer_online' || msg.event === 'wan_peer_offline' || msg.event === 'wan_peers_updated') {
        refreshWANPeers();
      }
      if (msg.event === 'wan_connection_status') {
        refreshWANStatus();
      }
      if (msg.event === 'wan_delivery_failed') {
        refreshConversations();
      }
    });
    return unsub;
  }, [refreshConversations, refreshWANPeers, refreshWANStatus]);

  // ─── 操作 ─────────────────────────────────────────────────────
  const handleStartChat = (peer, type) => {
    const normalizedPeer = type === 'wan'
      ? { instance_id: peer.instance_id, nickname: peer.nickname, avatar_url: peer.avatar_url }
      : { instance_id: peer.id, nickname: peer.nickname };
    setSelectedPeer(normalizedPeer);
    setStartModalOpen(true);
  };

  const handleAcceptConv = async (convId) => {
    if (!selectedAgentId) return;
    try {
      await api.acceptRemoteConversation(convId, Number(selectedAgentId));
      // 先更新本地状态让对话立即可见（不等异步 refresh）
      setConversations(prev => prev.map(c =>
        c.id === convId ? { ...c, status: 'active' } : c
      ));
      setAcceptingConvId(null);
      setSelectedAgentId('');
      setSelected({ type: 'conv', id: convId });
      refreshConversations();
    } catch {}
  };

  const handleRejectConv = async (convId) => {
    try { await api.rejectRemoteConversation(convId); refreshConversations(); } catch {}
  };

  const handleDeleteConv = async (convId) => {
    try {
      await api.deleteA2AConversation(convId);
      refreshConversations();
      if (selected?.type === 'conv' && selected?.id === convId) setSelected('discovery');
    } catch {}
  };

  // ─── 派生数据 ─────────────────────────────────────────────────
  const pendingConvs = conversations.filter(c => c.status === 'pending_incoming');
  const activeConvs = conversations.filter(c => c.status === 'active');
  const otherConvs = conversations.filter(c => !['active', 'pending_incoming'].includes(c.status));

  const filteredConvs = useMemo(() => {
    if (!searchQuery) return [...activeConvs, ...otherConvs];
    const q = searchQuery.toLowerCase();
    return [...activeConvs, ...otherConvs].filter(c => (c.topic || '').toLowerCase().includes(q) || (c.remote_peer_nickname || '').toLowerCase().includes(q));
  }, [activeConvs, otherConvs, searchQuery]);

  // ─── 对话详情视图 ─────────────────────────────────────────────
  if (selected?.type === 'conv' && selected?.id) {
    const conv = conversations.find(c => c.id === selected.id);
    if (conv && conv.status !== 'pending_incoming') {
      return (
        <A2AConversationView
          convId={selected.id}
          onBack={() => setSelected(null)}
        />
      );
    }
  }

  // ─── 主界面 ───────────────────────────────────────────────────
  return (
    <div className="flex-1 flex min-h-0">
      {/* 左侧边栏 */}
      <aside className="w-[280px] shrink-0 flex flex-col border-r border-[color:var(--line)] bg-[color:var(--bg-elev)]/80 backdrop-blur">
        {/* 顶部搜索 + 发现按钮 */}
        <div className="p-3 space-y-2 border-b border-[color:var(--line)]">
          <div className="flex items-center gap-2">
            <div className="flex-1 relative">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[color:var(--text-faint)]" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="搜索对话..."
                className="w-full h-8 pl-8 pr-3 rounded-lg bg-[color:var(--bg-soft)] border border-[color:var(--line)] text-xs text-[color:var(--text)] placeholder:text-[color:var(--text-faint)] focus:outline-none focus:border-[color:var(--accent)] transition"
              />
            </div>
            <button
              onClick={() => setSelected('discovery')}
              className={cn(
                'w-8 h-8 rounded-lg flex items-center justify-center transition',
                selected === 'discovery'
                  ? 'bg-[color:var(--accent)] text-white'
                  : 'bg-[color:var(--bg-soft)] text-[color:var(--text-faint)] hover:text-[color:var(--accent)] hover:bg-[color:var(--accent-soft)]'
              )}
              title="发现在线节点"
            >
              <Radar size={15} />
            </button>
          </div>
        </div>

        {/* 列表区域 */}
        <div className="flex-1 overflow-auto scrollable py-2">
          {/* 待处理对话邀请 */}
          <AnimatePresence>
            {pendingConvs.length > 0 && (
              <div className="mb-2">
                <div className="px-4 py-1 text-[10px] font-semibold text-red-500 uppercase tracking-wider flex items-center gap-1.5">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse" />
                  待处理 ({pendingConvs.length})
                </div>
                {pendingConvs.map(c => (
                  <div key={`cv-${c.id}`} className="px-3 py-2.5 mx-2 mb-1 rounded-lg bg-blue-50/80 dark:bg-blue-900/20 border border-blue-200/60 dark:border-blue-700/40">
                    <div className="flex items-center gap-2.5 mb-2">
                      <Avatar name={c.remote_peer_nickname} size="sm" color="purple" />
                      <div className="flex-1 min-w-0">
                        <div className="text-xs font-medium text-[color:var(--text)] truncate">{c.topic || '对话邀请'}</div>
                        <div className="text-[10px] text-blue-600 dark:text-blue-400">{c.remote_peer_nickname} / {c.remote_agent_name || 'Agent'}</div>
                      </div>
                    </div>
                    {acceptingConvId === c.id ? (
                      <div className="flex items-center gap-1.5">
                        <select value={selectedAgentId} onChange={(e) => setSelectedAgentId(e.target.value)}
                          className="flex-1 h-6 text-[10px] rounded border border-[color:var(--line)] bg-[color:var(--bg)] text-[color:var(--text)] px-1.5">
                          <option value="">选择 Agent...</option>
                          {agentList.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
                        </select>
                        <button onClick={() => handleAcceptConv(c.id)} disabled={!selectedAgentId}
                          className="h-6 px-2 rounded bg-emerald-500 text-white text-[10px] font-medium disabled:opacity-40">确认</button>
                        <button onClick={() => setAcceptingConvId(null)}
                          className="h-6 px-2 rounded bg-gray-200 dark:bg-gray-700 text-[color:var(--text-faint)] text-[10px]">取消</button>
                      </div>
                    ) : (
                      <div className="flex gap-1.5">
                        <button onClick={() => { setAcceptingConvId(c.id); setSelectedAgentId(''); }}
                          className="h-6 px-2 rounded bg-emerald-500 text-white text-[10px] font-medium flex items-center gap-1">
                          <Check size={10} /> 接受
                        </button>
                        <button onClick={() => handleRejectConv(c.id)}
                          className="h-6 px-2 rounded bg-gray-200 dark:bg-gray-700 text-[color:var(--text-faint)] text-[10px] flex items-center gap-1">
                          <X size={10} /> 拒绝
                        </button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </AnimatePresence>

          {/* 对话列表 */}
          {filteredConvs.length > 0 && (
            <div className="mb-2">
              <div className="px-4 py-1 text-[10px] font-semibold text-[color:var(--text-faint)] uppercase tracking-wider">
                对话 ({filteredConvs.length})
              </div>
              {filteredConvs.map(c => (
                <ConversationItem key={c.id} conv={c}
                  active={selected?.type === 'conv' && selected?.id === c.id}
                  onClick={() => setSelected({ type: 'conv', id: c.id })}
                  onDelete={handleDeleteConv}
                />
              ))}
            </div>
          )}

          {filteredConvs.length === 0 && pendingConvs.length === 0 && (
            <div className="text-center py-12 px-4">
              <Users size={24} className="mx-auto text-[color:var(--text-faint)] mb-2 opacity-40" />
              <p className="text-xs text-[color:var(--text-faint)]">暂无对话</p>
              <p className="text-[10px] text-[color:var(--text-faint)] mt-1">点击右上角「发现」找到在线节点并发起对话</p>
            </div>
          )}
        </div>

        {/* 底部状态栏 */}
        <div className="px-3 py-2 border-t border-[color:var(--line)] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1 text-[10px] text-[color:var(--text-faint)]">
              <Radio size={9} className="text-blue-400" />
              <span>{peers.length} LAN</span>
            </div>
            <div className="flex items-center gap-1 text-[10px] text-[color:var(--text-faint)]">
              <Globe size={9} className="text-purple-400" />
              <span>{wanPeers.length} WAN</span>
            </div>
          </div>
          <div className={cn('w-1.5 h-1.5 rounded-full', wanStatus?.connected ? 'bg-emerald-400' : 'bg-gray-300')} title={wanStatus?.connected ? 'WAN 已连接' : 'WAN 未连接'} />
        </div>
      </aside>

      {/* 右侧主区 */}
      <main className="flex-1 flex flex-col min-h-0">
        {selected === 'discovery' ? (
          <DiscoveryPanel
            peers={peers} wanPeers={wanPeers} wanStatus={wanStatus}
            loading={peersLoading} onRefresh={refreshPeers} onRefreshWAN={refreshWANPeers}
            onStartChat={handleStartChat}
          />
        ) : selected?.type === 'conv' ? (
          <A2AConversationView convId={selected.id} onBack={() => setSelected(null)} />
        ) : (
          <EmptyMainPanel />
        )}
      </main>

      <StartA2AModal
        open={startModalOpen}
        onClose={() => { setStartModalOpen(false); refreshConversations(); }}
        peer={selectedPeer}
        onCreated={(convId) => { refreshConversations(); setSelected({ type: 'conv', id: convId }); }}
      />
    </div>
  );
}
