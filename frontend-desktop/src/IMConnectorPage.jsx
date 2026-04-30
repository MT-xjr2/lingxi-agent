import { useState, useEffect } from 'react';

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

function ConnectorCard({ connector, onToggle, onEdit, onDelete }) {
  const platform = PLATFORMS.find(p => p.id === connector.platform);
  return (
    <div className={`connector-card${connector.enabled ? ' enabled' : ''}`}>
      <div className="connector-card-header">
        <span className="connector-platform-icon">{platform?.icon || '🔌'}</span>
        <div className="connector-card-info">
          <div className="connector-platform-name">{platform?.label || connector.platform}</div>
          <div className={`connector-status-badge${connector.running ? ' running' : connector.enabled ? ' enabled' : ''}`}>
            {connector.running ? '● 运行中' : connector.enabled ? '○ 已启用' : '○ 已停用'}
          </div>
        </div>
        <div className="connector-card-actions">
          <button className="connector-edit-btn" onClick={() => onEdit(connector)} title="编辑配置">✏️</button>
          <button
            className={`connector-toggle-btn${connector.enabled ? ' active' : ''}`}
            onClick={() => onToggle(connector)}
            title={connector.enabled ? '停用' : '启用'}
          >
            {connector.enabled ? '停用' : '启用'}
          </button>
          <button className="connector-delete-btn" onClick={() => onDelete(connector)} title="删除">✕</button>
        </div>
      </div>
      {connector.enabled && (
        <div className="connector-card-meta">
          <span className="connector-meta-item">
            会话模式：{SESSION_MODES.find(m => m.value === (connector.parsedConfig?.session_mode || 'per_group'))?.label || '按群共享'}
          </span>
          <span className="connector-meta-item">
            TTL：{connector.parsedConfig?.session_ttl_hours || 24}h
          </span>
        </div>
      )}
    </div>
  );
}

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
      if (!fields[f.key]?.trim()) {
        setError(`请填写 ${f.label}`);
        return;
      }
    }
    setSaving(true);
    try {
      const config = { ...fields, session_mode: sessionMode, session_ttl_hours: Number(ttlHours) };
      const r = await fetch('/api/im-connectors', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ platform, config }),
      });
      if (!r.ok) {
        const d = await r.json();
        setError(d.error || '保存失败');
        return;
      }
      onSave();
    } catch (e) {
      setError('保存失败：' + e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="connector-form-overlay" onClick={e => e.target === e.currentTarget && onCancel()}>
      <div className="connector-form-modal">
        <div className="connector-form-header">
          <h3>{isEdit ? '编辑连接器' : '添加 IM 连接器'}</h3>
          <button className="connector-form-close" onClick={onCancel}>✕</button>
        </div>

        {!isEdit && (
          <div className="connector-form-section">
            <label className="connector-form-label">平台</label>
            <div className="connector-platform-tabs">
              {PLATFORMS.map(p => (
                <button
                  key={p.id}
                  className={`connector-platform-tab${platform === p.id ? ' active' : ''}`}
                  onClick={() => { setPlatform(p.id); setFields({}); }}
                >
                  {p.icon} {p.label}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="connector-form-section">
          <div className="connector-form-section-title">平台凭证</div>
          {platformDef.fields.map(f => (
            <div key={f.key} className="connector-form-field">
              <label className="connector-form-label">{f.label}</label>
              <input
                className="connector-form-input"
                type={f.type}
                placeholder={f.placeholder}
                value={fields[f.key] || ''}
                onChange={e => setFields(prev => ({ ...prev, [f.key]: e.target.value }))}
                autoComplete="off"
              />
            </div>
          ))}
        </div>

        <div className="connector-form-section">
          <div className="connector-form-section-title">会话管理</div>
          <div className="connector-form-field">
            <label className="connector-form-label">会话粒度</label>
            <div className="connector-mode-list">
              {SESSION_MODES.map(m => (
                <label key={m.value} className={`connector-mode-item${sessionMode === m.value ? ' active' : ''}`}>
                  <input
                    type="radio"
                    name="session_mode"
                    value={m.value}
                    checked={sessionMode === m.value}
                    onChange={() => setSessionMode(m.value)}
                  />
                  <div className="connector-mode-content">
                    <span className="connector-mode-label">{m.label}</span>
                    <span className="connector-mode-desc">{m.desc}</span>
                  </div>
                </label>
              ))}
            </div>
          </div>
          {sessionMode !== 'stateless' && (
            <div className="connector-form-field">
              <label className="connector-form-label">
                上下文有效期（小时）
                <span className="connector-form-hint">超过此时间不活跃后自动开启新对话，0 表示永不重置</span>
              </label>
              <input
                className="connector-form-input connector-form-input-sm"
                type="number"
                min="0"
                max="720"
                value={ttlHours}
                onChange={e => setTtlHours(e.target.value)}
              />
            </div>
          )}
        </div>

        {platform === 'wecom' && (
          <div className="connector-form-tip">
            <span className="connector-form-tip-icon">ℹ️</span>
            企业微信需要公网 IP 或内网穿透，回调地址填写：<code>http://你的IP:3001/api/wecom/callback</code>
          </div>
        )}
        {platform === 'dingtalk' && (
          <div className="connector-form-tip">
            <span className="connector-form-tip-icon">ℹ️</span>
            钉钉 Stream 模式无需公网 IP，在开发者后台将消息接收模式设为 Stream 即可。
          </div>
        )}
        {platform === 'feishu' && (
          <div className="connector-form-tip">
            <span className="connector-form-tip-icon">ℹ️</span>
            飞书长连接模式无需公网 IP，在开发者后台开启机器人能力并订阅「接收消息」事件即可。
          </div>
        )}

        {error && <div className="connector-form-error">{error}</div>}

        <div className="connector-form-footer">
          <button className="connector-form-cancel" onClick={onCancel}>取消</button>
          <button className="connector-form-save" onClick={handleSave} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function IMConnectorPage({ onBack }) {
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
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchConnectors(); }, []);

  const handleToggle = async (connector) => {
    setTogglingId(connector.platform);
    try {
      const action = connector.enabled ? 'disable' : 'enable';
      const r = await fetch(`/api/im-connectors/${connector.platform}/${action}`, { method: 'PUT' });
      if (r.ok) await fetchConnectors();
    } finally {
      setTogglingId(null);
    }
  };

  const handleDelete = async (connector) => {
    if (!confirm(`确认删除 ${connector.platform} 连接器？配置将被清除。`)) return;
    await fetch(`/api/im-connectors/${connector.platform}`, { method: 'DELETE' });
    await fetchConnectors();
  };

  const handleEdit = (connector) => {
    setEditingConnector(connector);
    setShowForm(true);
  };

  const handleFormSave = async () => {
    setShowForm(false);
    setEditingConnector(null);
    await fetchConnectors();
  };

  const handleFormCancel = () => {
    setShowForm(false);
    setEditingConnector(null);
  };

  return (
    <div className="page-container">
      {showForm && (
        <ConnectorForm
          initial={editingConnector}
          onSave={handleFormSave}
          onCancel={handleFormCancel}
        />
      )}

      <div className="page-header">
        <button className="back-btn" onClick={onBack}>← 返回</button>
        <h2 className="page-title">
          <span className="page-title-icon">🔗</span>
          IM 连接器
        </h2>
        <button className="add-connector-btn" onClick={() => { setEditingConnector(null); setShowForm(true); }}>
          ＋ 添加连接器
        </button>
      </div>

      <div className="page-desc">
        连接钉钉、飞书、企业微信等 IM 平台，让 AI 助理直接在群聊中响应消息、执行技能。
      </div>

      <div className="page-body">
        {loading ? (
          <div className="page-loading">加载中...</div>
        ) : connectors.length === 0 ? (
          <div className="connector-empty">
            <div className="connector-empty-icon">🔌</div>
            <div className="connector-empty-title">还没有配置任何 IM 连接器</div>
            <div className="connector-empty-desc">点击右上角「添加连接器」开始配置</div>
            <button className="connector-empty-btn" onClick={() => setShowForm(true)}>添加第一个连接器</button>
          </div>
        ) : (
          <div className="connector-list">
            {connectors.map(c => (
              <ConnectorCard
                key={c.platform}
                connector={{ ...c, _toggling: togglingId === c.platform }}
                onToggle={handleToggle}
                onEdit={handleEdit}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
