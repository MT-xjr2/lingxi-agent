import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Link2, Plus, Pencil, Trash2, Power, PowerOff, Loader2, Info, Radio,
} from 'lucide-react';
import { Button, Card, Badge, Modal, Input, Select } from './ui/primitives';
import { cn } from './ui/cn';

const PLATFORMS = [
  { id: 'dingtalk', label: '钉钉', icon: '📌', fields: [
    { key: 'client_id',     label: 'Client ID',     placeholder: '应用的 Client ID（AppKey）', type: 'text' },
    { key: 'client_secret', label: 'Client Secret', placeholder: '应用的 Client Secret（AppSecret）', type: 'password' },
  ]},
  { id: 'feishu', label: '飞书', icon: '🪶', fields: [
    { key: 'app_id',     label: 'App ID',     placeholder: '飞书应用的 App ID', type: 'text' },
    { key: 'app_secret', label: 'App Secret', placeholder: '飞书应用的 App Secret', type: 'password' },
  ]},
  { id: 'wecom', label: '企业微信', icon: '💼', fields: [
    { key: 'corp_id',          label: 'Corp ID',          placeholder: '企业 ID（corpid）', type: 'text' },
    { key: 'agent_id',         label: 'Agent ID',         placeholder: '应用 AgentId', type: 'text' },
    { key: 'secret',           label: 'Secret',           placeholder: '应用 Secret', type: 'password' },
    { key: 'token',            label: 'Token',            placeholder: '消息接收 Token', type: 'text' },
    { key: 'encoding_aes_key', label: 'EncodingAESKey',   placeholder: '消息加解密 Key（43位）', type: 'password' },
  ]},
];

const SESSION_MODES = [
  { value: 'per_group',      label: '按群共享',   desc: '同一个群内所有人共享一个对话上下文（推荐）' },
  { value: 'per_user',       label: '按人独立',   desc: '同一个用户跨群共享，不同用户独立' },
  { value: 'per_group_user', label: '按群+人',    desc: '同一个群内每个人独立上下文' },
  { value: 'stateless',      label: '无状态',     desc: '每条消息独立，不保留任何上下文' },
];

function ConnectorForm({ initial, onSave, onCancel }) {
  const isEdit = !!initial;
  const [platform, setPlatform] = useState(initial?.platform || 'dingtalk');
  const [fields, setFields] = useState(() => {
    if (initial?.parsedConfig) {
      const { session_mode, session_ttl_hours, ...rest } = initial.parsedConfig;
      return rest;
    }
    return {};
  });
  const [sessionMode, setSessionMode] = useState(initial?.parsedConfig?.session_mode || 'per_group');
  const [ttlHours, setTtlHours] = useState(initial?.parsedConfig?.session_ttl_hours ?? 24);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const platformDef = PLATFORMS.find(p => p.id === platform);

  const handleSave = async () => {
    setError('');
    for (const f of platformDef.fields) {
      if (!fields[f.key]?.trim()) { setError(`请填写 ${f.label}`); return; }
    }
    setSaving(true);
    try {
      const config = { ...fields, session_mode: sessionMode, session_ttl_hours: Number(ttlHours) };
      const r = await fetch('/api/im-connectors', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ platform, config }),
      });
      if (!r.ok) { const d = await r.json(); setError(d.error || '保存失败'); return; }
      onSave();
    } catch (e) { setError('保存失败：' + e.message); }
    finally { setSaving(false); }
  };

  const tips = {
    wecom: { icon: Info, text: '企业微信需要公网 IP 或内网穿透，回调地址填写：', code: 'http://你的IP:3001/api/wecom/callback' },
    dingtalk: { icon: Info, text: '钉钉 Stream 模式无需公网 IP，在开发者后台将消息接收模式设为 Stream 即可。' },
    feishu: { icon: Info, text: '飞书长连接模式无需公网 IP，在开发者后台开启机器人能力并订阅「接收消息」事件即可。' },
  };
  const tip = tips[platform];

  return (
    <Modal open onClose={onCancel} title={isEdit ? '编辑连接器' : '添加 IM 连接器'} width={520}>
      <div className="space-y-5">
        {!isEdit && (
          <div>
            <div className="text-xs font-medium text-[color:var(--text-faint)] uppercase tracking-wide mb-2">平台</div>
            <div className="flex gap-2 flex-wrap">
              {PLATFORMS.map(p => (
                <button key={p.id} onClick={() => { setPlatform(p.id); setFields({}); }} className={cn(
                  'px-3 py-2 rounded-lg border text-sm transition',
                  platform === p.id
                    ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)] text-[color:var(--accent)] font-medium'
                    : 'border-[color:var(--line)] text-[color:var(--text-soft)] hover:bg-[color:var(--bg-soft)]'
                )}>
                  {p.icon} {p.label}
                </button>
              ))}
            </div>
          </div>
        )}

        <div>
          <div className="text-xs font-medium text-[color:var(--text-faint)] uppercase tracking-wide mb-3 pb-1.5 border-b border-[color:var(--line)]">平台凭证</div>
          <div className="space-y-3">
            {platformDef.fields.map(f => (
              <div key={f.key}>
                <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">{f.label}</label>
                <Input type={f.type} placeholder={f.placeholder} value={fields[f.key] || ''} onChange={e => setFields(prev => ({ ...prev, [f.key]: e.target.value }))} autoComplete="off" />
              </div>
            ))}
          </div>
        </div>

        <div>
          <div className="text-xs font-medium text-[color:var(--text-faint)] uppercase tracking-wide mb-3 pb-1.5 border-b border-[color:var(--line)]">会话管理</div>
          <div className="text-xs font-medium text-[color:var(--text-soft)] mb-2">会话粒度</div>
          <div className="space-y-2">
            {SESSION_MODES.map(m => (
              <label key={m.value} className={cn(
                'flex items-start gap-2.5 p-3 rounded-lg border cursor-pointer transition',
                sessionMode === m.value ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)]' : 'border-[color:var(--line)] hover:bg-[color:var(--bg-soft)]'
              )}>
                <input type="radio" name="session_mode" value={m.value} checked={sessionMode === m.value} onChange={() => setSessionMode(m.value)} className="mt-0.5 accent-[color:var(--accent)]" />
                <div>
                  <div className="text-sm font-medium">{m.label}</div>
                  <div className="text-xs text-[color:var(--text-faint)]">{m.desc}</div>
                </div>
              </label>
            ))}
          </div>
          {sessionMode !== 'stateless' && (
            <div className="mt-3">
              <label className="text-xs font-medium text-[color:var(--text-soft)] mb-1 block">
                上下文有效期（小时）
                <span className="font-normal text-[color:var(--text-faint)] ml-1">超过此时间不活跃后自动开启新对话，0 表示永不重置</span>
              </label>
              <Input type="number" min="0" max="720" className="w-32" value={ttlHours} onChange={e => setTtlHours(e.target.value)} />
            </div>
          )}
        </div>

        {tip && (
          <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg bg-blue-500/10 border border-blue-500/20 text-xs text-[color:var(--text-soft)] leading-relaxed">
            <Info size={14} className="shrink-0 mt-0.5 text-blue-400" />
            <span>{tip.text}{tip.code && <code className="bg-blue-500/10 px-1.5 py-0.5 rounded font-mono text-[11px] ml-1">{tip.code}</code>}</span>
          </div>
        )}

        {error && <div className="px-3 py-2.5 rounded-lg bg-red-500/10 border border-red-500/30 text-red-500 text-sm">{error}</div>}

        <div className="flex justify-end gap-2 pt-3 border-t border-[color:var(--line)]">
          <Button variant="outline" onClick={onCancel}>取消</Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? <><Loader2 size={14} className="animate-spin" />保存中...</> : '保存'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

export default function IMConnectorPage() {
  const [connectors, setConnectors] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingConnector, setEditingConnector] = useState(null);
  const [togglingId, setTogglingId] = useState(null);

  const fetchConnectors = async () => {
    setLoading(true);
    try {
      const r = await fetch('/api/im-connectors');
      const data = await r.json();
      const list = (data || []).map(c => {
        let parsedConfig = {};
        try { parsedConfig = JSON.parse(c.config); } catch {}
        return { ...c, parsedConfig };
      });
      setConnectors(list);
    } finally { setLoading(false); }
  };

  useEffect(() => { fetchConnectors(); }, []);

  const handleToggle = async (connector) => {
    setTogglingId(connector.platform);
    try {
      const action = connector.enabled ? 'disable' : 'enable';
      const r = await fetch(`/api/im-connectors/${connector.platform}/${action}`, { method: 'PUT' });
      if (r.ok) await fetchConnectors();
    } finally { setTogglingId(null); }
  };

  const handleDelete = async (connector) => {
    if (!confirm(`确认删除 ${connector.platform} 连接器？配置将被清除。`)) return;
    await fetch(`/api/im-connectors/${connector.platform}`, { method: 'DELETE' });
    await fetchConnectors();
  };

  return (
    <div className="max-w-5xl mx-auto">
      {showForm && (
        <ConnectorForm
          initial={editingConnector}
          onSave={async () => { setShowForm(false); setEditingConnector(null); await fetchConnectors(); }}
          onCancel={() => { setShowForm(false); setEditingConnector(null); }}
        />
      )}

      <div className="relative overflow-hidden rounded-2xl mb-6 p-6 surface-grad">
        <div className="absolute -right-20 -top-20 w-64 h-64 rounded-full bg-gradient-to-br from-[color:var(--accent)]/30 to-transparent blur-3xl pointer-events-none" />
        <div className="relative flex items-center gap-4">
          <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-[color:var(--accent)] to-[#5e8bff] text-white flex items-center justify-center shadow-glow">
            <Link2 size={26} />
          </div>
          <div className="flex-1">
            <div className="text-2xl font-semibold tracking-tight text-gradient">IM 连接器</div>
            <div className="text-sm text-[color:var(--text-soft)]">连接钉钉、飞书、企业微信，让 AI 助理直接在群聊中响应</div>
          </div>
          <Button onClick={() => { setEditingConnector(null); setShowForm(true); }}>
            <Plus size={14} /> 添加连接器
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="py-20 text-center text-[color:var(--text-faint)]">
          <Loader2 size={24} className="animate-spin mx-auto mb-3" />加载中...
        </div>
      ) : connectors.length === 0 ? (
        <div className="py-20 text-center">
          <Link2 size={40} className="mx-auto mb-3 text-[color:var(--accent)] opacity-50" />
          <p className="text-[color:var(--text-soft)]">还没有配置任何 IM 连接器</p>
          <p className="text-xs text-[color:var(--text-faint)] mt-1">点击上方「添加连接器」开始配置</p>
          <Button className="mt-4" onClick={() => setShowForm(true)}>
            <Plus size={14} /> 添加第一个连接器
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <AnimatePresence>
            {connectors.map(c => (
              <motion.div key={c.platform} layout initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, scale: 0.95 }}>
                <ConnectorCard
                  connector={{ ...c, _toggling: togglingId === c.platform }}
                  onToggle={handleToggle}
                  onEdit={conn => { setEditingConnector(conn); setShowForm(true); }}
                  onDelete={handleDelete}
                />
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}
    </div>
  );
}

function ConnectorCard({ connector, onToggle, onEdit, onDelete }) {
  const platform = PLATFORMS.find(p => p.id === connector.platform);
  return (
    <Card className={cn('transition-all hover:-translate-y-0.5 hover:shadow-glow group', connector.enabled && 'border-[color:var(--accent)]/40')}>
      <div className="flex items-center gap-3">
        <span className="text-2xl">{platform?.icon || '🔌'}</span>
        <div className="flex-1 min-w-0">
          <div className="font-medium">{platform?.label || connector.platform}</div>
          <div className="flex items-center gap-1.5 mt-0.5">
            {connector.running ? (
              <Badge tone="success"><Radio size={10} className="animate-pulse" /> 运行中</Badge>
            ) : connector.enabled ? (
              <Badge tone="accent">已启用</Badge>
            ) : (
              <Badge tone="default">已停用</Badge>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1.5 shrink-0 opacity-0 group-hover:opacity-100 transition">
          <Button size="sm" variant="ghost" onClick={() => onEdit(connector)}><Pencil size={14} /></Button>
          <Button size="sm" variant={connector.enabled ? 'outline' : 'default'} onClick={() => onToggle(connector)} disabled={connector._toggling}>
            {connector._toggling ? <Loader2 size={12} className="animate-spin" /> : connector.enabled ? <><PowerOff size={12} /> 停用</> : <><Power size={12} /> 启用</>}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => onDelete(connector)}><Trash2 size={14} /></Button>
        </div>
      </div>
      {connector.enabled && (
        <div className="flex gap-4 mt-3 pt-3 border-t border-[color:var(--line)] text-xs text-[color:var(--text-faint)]">
          <span>会话模式：{SESSION_MODES.find(m => m.value === (connector.parsedConfig?.session_mode || 'per_group'))?.label || '按群共享'}</span>
          <span>TTL：{connector.parsedConfig?.session_ttl_hours || 24}h</span>
        </div>
      )}
    </Card>
  );
}
