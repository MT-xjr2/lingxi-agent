import { useState, useEffect } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { api } from '../api/client';
import { Modal, Button, Input, Textarea, Select } from '../ui/primitives';

export default function StartA2AModal({ open, onClose, peer, onCreated }) {
  const [agents, setAgents] = useState([]);
  const [form, setForm] = useState({
    local_agent_id: '',
    topic: '',
    goal: '',
    initial_prompt: '',
    max_rounds: 10,
    require_approval: true,
  });
  const [submitting, setSubmitting] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  useEffect(() => {
    if (open) {
      api.listAgents().then(setAgents).catch(() => {});
    }
  }, [open]);

  const handleSubmit = async () => {
    if (!form.local_agent_id || !form.initial_prompt.trim() || !peer) return;
    setSubmitting(true);
    try {
      const res = await api.createA2AConversation({
        local_agent_id: Number(form.local_agent_id),
        remote_peer_id: peer.instance_id || peer.id || peer.peer_id,
        topic: form.topic,
        goal: form.goal,
        initial_prompt: form.initial_prompt,
        max_rounds: Number(form.max_rounds) || 10,
        require_approval: form.require_approval,
      });
      if (onCreated && res?.id) onCreated(res.id);
      onClose();
    } catch {}
    setSubmitting(false);
  };

  const peerName = peer?.nickname || peer?.name || '对方';

  return (
    <Modal open={open} onClose={onClose} title={`向 ${peerName} 的 Agent 提问`} width={520}>
      <div className="space-y-4">
        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">委托 Agent</label>
          <Select
            value={form.local_agent_id}
            onChange={(e) => setForm({ ...form, local_agent_id: e.target.value })}
          >
            <option value="">选择代理你提问的 Agent...</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>{a.name}</option>
            ))}
          </Select>
          <p className="text-[10px] text-[color:var(--text-faint)] mt-1">该 Agent 将以你的身份与对方对话</p>
        </div>

        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">你的提问</label>
          <Textarea
            value={form.initial_prompt}
            onChange={(e) => setForm({ ...form, initial_prompt: e.target.value })}
            placeholder="输入你想问对方 Agent 的问题或任务..."
            rows={4}
            autoFocus
          />
        </div>

        <button
          type="button"
          onClick={() => setShowAdvanced(!showAdvanced)}
          className="flex items-center gap-1 text-xs text-[color:var(--text-faint)] hover:text-[color:var(--text-soft)] transition-colors"
        >
          {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          高级选项
        </button>

        {showAdvanced && (
          <div className="space-y-3 pl-2 border-l-2 border-[color:var(--line)]">
            <div>
              <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">对话主题</label>
              <Input
                value={form.topic}
                onChange={(e) => setForm({ ...form, topic: e.target.value })}
                placeholder="可选，如：技术评审、方案讨论..."
              />
            </div>

            <div>
              <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">期望目标</label>
              <Input
                value={form.goal}
                onChange={(e) => setForm({ ...form, goal: e.target.value })}
                placeholder="可选，对话结束时应达成的目标"
              />
            </div>

            <div className="flex gap-4">
              <div className="flex-1">
                <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">最大轮次</label>
                <Input
                  type="number"
                  value={form.max_rounds}
                  onChange={(e) => setForm({ ...form, max_rounds: e.target.value })}
                  min={1}
                  max={100}
                />
              </div>
              <div className="flex-1 flex items-end">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={form.require_approval}
                    onChange={(e) => setForm({ ...form, require_approval: e.target.checked })}
                    className="rounded"
                  />
                  <span className="text-xs text-[color:var(--text-soft)]">结果需人类审批</span>
                </label>
              </div>
            </div>
          </div>
        )}

        <div className="text-xs text-[color:var(--text-faint)] p-2 rounded-lg bg-[color:var(--bg-soft)]">
          你的 Agent 将以第一人称与对方自然对话，可使用技能和知识库。
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={!form.local_agent_id || !form.initial_prompt.trim() || submitting}
          >
            {submitting ? '发送中...' : '发起对话'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
