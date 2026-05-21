import { useEffect, useMemo, useState } from 'react';
import { Users, Plus, X, Globe, Wifi } from 'lucide-react';
import { api } from '../api/client';
import { Modal, Button, Input, Textarea, Select } from '../ui/primitives';

export default function CreateGroupModal({ open, onClose, onCreated }) {
  const [agents, setAgents] = useState([]);
  const [lanPeers, setLanPeers] = useState([]);
  const [wanPeers, setWanPeers] = useState([]);

  const [topic, setTopic] = useState('');
  const [goal, setGoal] = useState('');
  const [localAgentIds, setLocalAgentIds] = useState([]);
  const [remoteMembers, setRemoteMembers] = useState([]);
  const [submitting, setSubmitting] = useState(false);

  const [pickPeer, setPickPeer] = useState('');
  const [pickAgentName, setPickAgentName] = useState('');

  useEffect(() => {
    if (!open) return;
    api.listAgents().then(setAgents).catch(() => {});
    api.listPeers().then((p) => setLanPeers(p || [])).catch(() => {});
    api.listWANPeers().then((p) => setWanPeers(p || [])).catch(() => {});
  }, [open]);

  const allPeers = useMemo(() => {
    const map = new Map();
    for (const p of lanPeers) {
      map.set(p.id, { id: p.id, nickname: p.nickname || p.id, agents_json: p.agents_json, type: 'lan' });
    }
    for (const p of wanPeers) {
      const id = p.instance_id || p.id;
      if (!map.has(id)) {
        map.set(id, { id, nickname: p.nickname || id, agents_json: JSON.stringify(p.agents || []), type: 'wan' });
      }
    }
    return Array.from(map.values());
  }, [lanPeers, wanPeers]);

  const peerAgents = useMemo(() => {
    const peer = allPeers.find((p) => p.id === pickPeer);
    if (!peer || !peer.agents_json) return [];
    try {
      const arr = typeof peer.agents_json === 'string' ? JSON.parse(peer.agents_json) : peer.agents_json;
      return Array.isArray(arr) ? arr : [];
    } catch {
      return [];
    }
  }, [pickPeer, allPeers]);

  const reset = () => {
    setTopic('');
    setGoal('');
    setLocalAgentIds([]);
    setRemoteMembers([]);
    setPickPeer('');
    setPickAgentName('');
  };

  const handleAddLocalAgent = (id) => {
    if (!id) return;
    if (localAgentIds.includes(id)) return;
    setLocalAgentIds([...localAgentIds, id]);
  };

  const handleRemoveLocalAgent = (id) => {
    setLocalAgentIds(localAgentIds.filter((x) => x !== id));
  };

  const handleAddRemoteMember = () => {
    if (!pickPeer || !pickAgentName.trim()) return;
    const peer = allPeers.find((p) => p.id === pickPeer);
    if (!peer) return;
    const exists = remoteMembers.some((m) => m.peer_id === pickPeer && m.agent_name === pickAgentName);
    if (exists) return;
    setRemoteMembers([...remoteMembers, {
      peer_id: pickPeer,
      peer_nickname: peer.nickname,
      agent_name: pickAgentName.trim(),
    }]);
    setPickAgentName('');
  };

  const handleRemoveRemoteMember = (peerId, agentName) => {
    setRemoteMembers(remoteMembers.filter((m) => !(m.peer_id === peerId && m.agent_name === agentName)));
  };

  const handleSubmit = async () => {
    if (!topic.trim() || (localAgentIds.length === 0 && remoteMembers.length === 0)) return;
    setSubmitting(true);
    try {
      const res = await api.createGroupChat({
        topic,
        goal,
        local_agent_ids: localAgentIds,
        remote_members: remoteMembers,
      });
      onCreated?.(res?.id);
      reset();
      onClose();
    } catch {}
    setSubmitting(false);
  };

  return (
    <Modal open={open} onClose={() => { reset(); onClose(); }} title="创建群聊" width={620}>
      <div className="space-y-4 max-h-[70vh] overflow-auto scrollable pr-1">
        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">主题</label>
          <Input value={topic} onChange={(e) => setTopic(e.target.value)} placeholder="比如：技术评审、周末去哪玩" autoFocus />
        </div>
        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">目标（可选）</label>
          <Textarea value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="希望聊出什么结果" rows={2} />
        </div>

        <div className="text-[11px] text-[color:var(--text-faint)] p-2 rounded-lg bg-[color:var(--bg-soft)]">
          建群后 Agent 会自动开始聊天，你也可以随时插话、@ 某人或回复消息。
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-xs font-medium text-[color:var(--text-soft)]">本端 Agent</label>
            <Select value="" onChange={(e) => handleAddLocalAgent(parseInt(e.target.value))} className="!w-auto !inline-block">
              <option value="">+ 添加 Agent</option>
              {agents.filter((a) => !localAgentIds.includes(a.id)).map((a) => (
                <option key={a.id} value={a.id}>{a.name}</option>
              ))}
            </Select>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {localAgentIds.length === 0 && (
              <span className="text-[11px] text-[color:var(--text-faint)]">建议至少 1 个本端 Agent</span>
            )}
            {localAgentIds.map((id) => {
              const agent = agents.find((a) => a.id === id);
              if (!agent) return null;
              return (
                <span key={id} className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-[color:var(--accent-soft)] text-[color:var(--accent)] text-xs">
                  {agent.name}
                  <button onClick={() => handleRemoveLocalAgent(id)}><X size={11} /></button>
                </span>
              );
            })}
          </div>
        </div>

        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">远端成员（其他实例的 Agent）</label>
          <div className="flex gap-2 mb-2">
            <Select value={pickPeer} onChange={(e) => { setPickPeer(e.target.value); setPickAgentName(''); }}>
              <option value="">选择 Peer</option>
              {allPeers.map((p) => (
                <option key={p.id} value={p.id}>{p.type === 'wan' ? '🌐' : '📡'} {p.nickname}</option>
              ))}
            </Select>
            {peerAgents.length > 0 ? (
              <Select value={pickAgentName} onChange={(e) => setPickAgentName(e.target.value)}>
                <option value="">选择 Agent</option>
                {peerAgents.map((a, i) => (
                  <option key={i} value={a.name || a.public_name || ''}>{a.name || a.public_name}</option>
                ))}
              </Select>
            ) : (
              <Input
                value={pickAgentName}
                onChange={(e) => setPickAgentName(e.target.value)}
                placeholder={pickPeer ? '输入对方 Agent 名' : '先选择 Peer'}
                disabled={!pickPeer}
              />
            )}
            <Button size="sm" onClick={handleAddRemoteMember} disabled={!pickPeer || !pickAgentName.trim()}>
              <Plus size={12} /> 添加
            </Button>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {remoteMembers.length === 0 && <span className="text-[11px] text-[color:var(--text-faint)]">可选</span>}
            {remoteMembers.map((m, i) => {
              const peer = allPeers.find((p) => p.id === m.peer_id);
              return (
                <span key={i} className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 text-xs">
                  {peer?.type === 'wan' ? <Globe size={10} /> : <Wifi size={10} />}
                  {m.peer_nickname} / {m.agent_name}
                  <button onClick={() => handleRemoveRemoteMember(m.peer_id, m.agent_name)}><X size={11} /></button>
                </span>
              );
            })}
          </div>
        </div>
      </div>

      <div className="flex justify-end gap-2 pt-3 border-t border-[color:var(--line)] mt-3">
        <Button variant="ghost" onClick={() => { reset(); onClose(); }}>取消</Button>
        <Button
          onClick={handleSubmit}
          disabled={submitting || !topic.trim() || (localAgentIds.length === 0 && remoteMembers.length === 0)}
        >
          <Users size={14} />
          {submitting ? '创建中…' : '创建群聊'}
        </Button>
      </div>
    </Modal>
  );
}
