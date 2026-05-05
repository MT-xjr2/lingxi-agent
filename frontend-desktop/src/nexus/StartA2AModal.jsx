import { useState, useEffect } from 'react';
import { api } from '../api/client';
import { Modal, Button, Input, Textarea, Select } from '../ui/primitives';

export default function StartA2AModal({ open, onClose, contact, onCreated }) {
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

  useEffect(() => {
    if (open) {
      api.listAgents().then(setAgents).catch(() => {});
    }
  }, [open]);

  const handleSubmit = async () => {
    if (!form.local_agent_id || !contact) return;
    setSubmitting(true);
    try {
      const res = await api.createA2AConversation({
        local_agent_id: Number(form.local_agent_id),
        remote_peer_id: contact.peer_id,
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

  return (
    <Modal open={open} onClose={onClose} title="发起 Agent 对话" width={520}>
      <div className="space-y-4">
        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">己方 Agent</label>
          <Select
            value={form.local_agent_id}
            onChange={(e) => setForm({ ...form, local_agent_id: e.target.value })}
          >
            <option value="">选择 Agent...</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>{a.name}</option>
            ))}
          </Select>
        </div>

        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">对话主题</label>
          <Input
            value={form.topic}
            onChange={(e) => setForm({ ...form, topic: e.target.value })}
            placeholder="如：采购议价、技术评审..."
          />
        </div>

        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">期望目标</label>
          <Input
            value={form.goal}
            onChange={(e) => setForm({ ...form, goal: e.target.value })}
            placeholder="对话结束时应达成的目标"
          />
        </div>

        <div>
          <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">初始提示</label>
          <Textarea
            value={form.initial_prompt}
            onChange={(e) => setForm({ ...form, initial_prompt: e.target.value })}
            placeholder="给己方 Agent 的详细指令..."
            rows={3}
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

        <div className="text-xs text-[color:var(--text-faint)] p-2 rounded-lg bg-[color:var(--bg-soft)]">
          对方将收到对话请求通知，可以选择使用哪个 Agent 参与对话。
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={!form.local_agent_id || submitting}
          >
            {submitting ? '发送中...' : '发送对话请求'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
