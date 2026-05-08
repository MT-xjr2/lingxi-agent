import { useEffect, useState, useMemo } from 'react';
import {
  Plus, Pencil, Trash2, Zap, ExternalLink, ShieldCheck, Loader2,
  CheckCircle2, AlertCircle, Eye, EyeOff, Search, ChevronDown, ChevronRight,
  ArrowLeft, Sparkles, Telescope, Cloud, Globe, Server, Brain, Cpu, Bot,
  MessageSquare, CircuitBoard, Layers, Star, Flame, Box, Settings2,
} from 'lucide-react';
import { useStore } from '../state/useStore';
import { api, electron } from '../api/client';
import { Button, Input, Modal, Badge, Card, EmptyState } from '../ui/primitives';
import { cn } from '../ui/cn';

// ─── 供应商视觉主题映射 ──────────────────────────────────────────
const PROVIDER_THEME = {
  anthropic_official:  { icon: Sparkles,     gradient: 'from-amber-500 to-orange-600',       label: 'Claude',      color: '#d97706' },
  dashscope_anthropic: { icon: Cloud,        gradient: 'from-orange-400 to-amber-600',       label: 'DashScope',   color: '#f59e0b' },
  deepseek_anthropic:  { icon: Telescope,    gradient: 'from-blue-500 to-indigo-600',        label: 'DeepSeek',    color: '#6366f1' },
  deepseek_openai:     { icon: Telescope,    gradient: 'from-blue-500 to-indigo-600',        label: 'DeepSeek',    color: '#6366f1' },
  qwen_openai:         { icon: Cloud,        gradient: 'from-orange-500 to-red-500',         label: 'Qwen',        color: '#f97316' },
  doubao_openai:       { icon: Flame,        gradient: 'from-rose-500 to-pink-600',          label: 'Doubao',      color: '#f43f5e' },
  glm_openai:          { icon: Brain,        gradient: 'from-cyan-500 to-blue-600',          label: 'GLM',         color: '#06b6d4' },
  moonshot_openai:     { icon: Star,         gradient: 'from-violet-500 to-purple-600',      label: 'Kimi',        color: '#8b5cf6' },
  gemini_openai:       { icon: Globe,        gradient: 'from-blue-400 via-emerald-400 to-amber-400', label: 'Gemini', color: '#3b82f6' },
  openrouter_openai:   { icon: Layers,       gradient: 'from-emerald-500 to-teal-600',       label: 'OpenRouter',  color: '#10b981' },
  groq_openai:         { icon: Zap,          gradient: 'from-amber-400 to-orange-500',       label: 'Groq',        color: '#f59e0b' },
  siliconflow_openai:  { icon: CircuitBoard, gradient: 'from-sky-500 to-blue-600',           label: 'SiliconFlow', color: '#0ea5e9' },
  ollama_openai:       { icon: Server,       gradient: 'from-slate-500 to-slate-700',        label: 'Ollama',      color: '#64748b' },
  openai_official:     { icon: Bot,          gradient: 'from-emerald-500 to-green-600',      label: 'OpenAI',      color: '#10b981' },
  custom_anthropic:    { icon: Settings2,    gradient: 'from-gray-500 to-gray-600',          label: '自定义',      color: '#6b7280' },
  custom_openai:       { icon: Settings2,    gradient: 'from-gray-500 to-gray-600',          label: '自定义',      color: '#6b7280' },
};

const DEFAULT_THEME = { icon: Cpu, gradient: 'from-gray-400 to-gray-600', label: '', color: '#9ca3af' };

const PROVIDER_MODELS = {
  anthropic_official: [
    { group: '旗舰', models: ['claude-sonnet-4-20250514', 'claude-opus-4-20250514'] },
    { group: '快速', models: ['claude-haiku-4-20250514'] },
    { group: '上一代', models: ['claude-3-5-sonnet-20241022', 'claude-3-5-haiku-20241022'] },
  ],
  openai_official: [
    { group: '旗舰', models: ['gpt-4o', 'gpt-4o-2024-11-20', 'o3-mini'] },
    { group: '快速', models: ['gpt-4o-mini'] },
    { group: '推理', models: ['o1', 'o1-mini', 'o3', 'o4-mini'] },
  ],
  deepseek_anthropic: [
    { group: '通用', models: ['deepseek-chat', 'deepseek-reasoner'] },
  ],
  deepseek_openai: [
    { group: '通用', models: ['deepseek-chat', 'deepseek-reasoner'] },
  ],
  qwen_openai: [
    { group: '旗舰', models: ['qwen-max', 'qwen-max-latest'] },
    { group: '长上下文', models: ['qwen-long', 'qwen-plus'] },
    { group: '快速', models: ['qwen-turbo', 'qwen-turbo-latest'] },
  ],
  dashscope_anthropic: [
    { group: 'Claude', models: ['claude-sonnet-4-20250514', 'claude-3-5-sonnet-20241022'] },
    { group: 'Qwen', models: ['qwen-max', 'qwen-plus', 'qwen-turbo'] },
  ],
  doubao_openai: [
    { group: '通用', models: ['doubao-1.5-pro-32k', 'doubao-1.5-pro-256k', 'doubao-1.5-lite-32k'] },
    { group: '推理', models: ['doubao-1.5-thinking-pro', 'doubao-1.5-thinking-pro-250415'] },
  ],
  glm_openai: [
    { group: '通用', models: ['glm-4-plus', 'glm-4-flash', 'glm-4-long', 'glm-4-flashx'] },
  ],
  moonshot_openai: [
    { group: '通用', models: ['moonshot-v1-8k', 'moonshot-v1-32k', 'moonshot-v1-128k'] },
  ],
  gemini_openai: [
    { group: '旗舰', models: ['gemini-2.5-pro', 'gemini-2.5-flash'] },
    { group: '上一代', models: ['gemini-2.0-flash', 'gemini-1.5-pro', 'gemini-1.5-flash'] },
  ],
  groq_openai: [
    { group: '快速', models: ['llama-3.3-70b-versatile', 'llama-3.1-8b-instant', 'mixtral-8x7b-32768'] },
  ],
  siliconflow_openai: [
    { group: '通用', models: ['deepseek-ai/DeepSeek-V3', 'Qwen/Qwen2.5-72B-Instruct', 'meta-llama/Llama-3.3-70B-Instruct'] },
  ],
  openrouter_openai: [
    { group: '热门', models: ['anthropic/claude-sonnet-4', 'openai/gpt-4o', 'google/gemini-2.5-pro', 'deepseek/deepseek-chat'] },
  ],
};

function getProviderTheme(code) {
  return PROVIDER_THEME[code] || DEFAULT_THEME;
}

// ─── 供应商图标组件 ──────────────────────────────────────────────
function ProviderIcon({ code, size = 'md', className }) {
  const theme = getProviderTheme(code);
  const Icon = theme.icon;
  const sizes = { sm: 'w-8 h-8', md: 'w-10 h-10', lg: 'w-12 h-12' };
  const iconSizes = { sm: 14, md: 18, lg: 22 };
  return (
    <div className={cn(
      sizes[size], 'rounded-xl flex items-center justify-center shrink-0 bg-gradient-to-br text-white shadow-sm',
      theme.gradient, className
    )}>
      <Icon size={iconSizes[size]} />
    </div>
  );
}

// ─── 主页面 ──────────────────────────────────────────────────────
export function ProfilesPage() {
  const providers = useStore((s) => s.providers);
  const profiles = useStore((s) => s.profiles);
  const refreshProfiles = useStore((s) => s.refreshProfiles);
  const activate = useStore((s) => s.activateProfile);
  const pushNotification = useStore((s) => s.pushNotification);

  const [editing, setEditing] = useState(null);
  const [testStates, setTestStates] = useState({});

  useEffect(() => { refreshProfiles(); }, []);

  const handleDelete = async (p) => {
    if (!confirm(`删除接入点「${p.name}」？`)) return;
    await api.deleteProfile(p.id);
    await refreshProfiles();
  };

  const handleTest = async (p) => {
    setTestStates((s) => ({ ...s, [p.id]: 'testing' }));
    try {
      let token = '';
      if (p.auth_token_cipher) {
        token = await electron.decryptSecret(p.auth_token_cipher);
      }
      const r = await api.testProfile(p.id, { token });
      setTestStates((s) => ({ ...s, [p.id]: r.ok ? 'success' : 'fail' }));
      if (!r.ok) pushNotification({ title: '连接失败', body: r.error || '请检查配置' });
    } catch {
      setTestStates((s) => ({ ...s, [p.id]: 'fail' }));
    }
    setTimeout(() => setTestStates((s) => { const n = { ...s }; delete n[p.id]; return n; }), 4000);
  };

  const providerMap = useMemo(() => {
    const m = {};
    providers.forEach((p) => { m[p.id] = p; });
    return m;
  }, [providers]);

  return (
    <div className="max-w-5xl mx-auto py-6 px-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">模型与接入点</h1>
          <p className="text-sm text-[color:var(--text-soft)] mt-1">
            选择供应商，填入密钥即可开始使用
          </p>
        </div>
        <Button onClick={() => setEditing({ __new: true })}>
          <Plus size={14} /> 新建接入点
        </Button>
      </div>

      {profiles.length === 0 ? (
        <EmptyState
          icon={Cpu}
          title="还没有接入点"
          description="点击「新建接入点」选择供应商并填入密钥，即可开始对话"
          action={
            <Button onClick={() => setEditing({ __new: true })}>
              <Plus size={14} /> 新建接入点
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {profiles.map((p) => (
            <ProfileCard
              key={p.id}
              profile={p}
              provider={providerMap[p.provider_id]}
              testState={testStates[p.id]}
              onActivate={() => activate(p.id)}
              onTest={() => handleTest(p)}
              onEdit={() => setEditing(p)}
              onDelete={() => handleDelete(p)}
            />
          ))}
        </div>
      )}

      {editing && (
        <ProfileEditor
          providers={providers}
          profile={editing.__new ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={async () => {
            setEditing(null);
            await refreshProfiles();
          }}
        />
      )}
    </div>
  );
}

// ─── 接入点卡片 ──────────────────────────────────────────────────
function ProfileCard({ profile: p, provider, testState, onActivate, onTest, onEdit, onDelete }) {
  const theme = getProviderTheme(provider?.code);
  const testIcon = testState === 'testing' ? Loader2
    : testState === 'success' ? CheckCircle2
    : testState === 'fail' ? AlertCircle : null;

  return (
    <Card className={cn(
      'group relative flex flex-col gap-3 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md overflow-hidden',
      p.is_active && 'ring-1 ring-[color:var(--accent)]/40'
    )}>
      {/* 顶部渐变色条 */}
      <div className={cn('absolute top-0 left-0 right-0 h-0.5 bg-gradient-to-r', theme.gradient)} />

      <div className="flex items-start justify-between gap-3 pt-1">
        <div className="flex items-center gap-3 min-w-0">
          <ProviderIcon code={provider?.code} />
          <div className="min-w-0">
            <div className="font-medium truncate flex items-center gap-2">
              {p.name}
              {p.is_active && (
                <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md text-[10px] font-semibold bg-emerald-500/10 text-emerald-600">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
                  使用中
                </span>
              )}
            </div>
            <div className="text-xs text-[color:var(--text-faint)] truncate mt-0.5 flex items-center gap-1.5">
              <Badge tone={p.provider_protocol === 'openai' ? 'info' : 'default'} className="!text-[10px] !px-1.5 !py-0">
                {p.provider_protocol === 'openai' ? 'OpenAI' : 'Anthropic'}
              </Badge>
              <span className="truncate">{p.model || provider?.default_model || '默认模型'}</span>
            </div>
          </div>
        </div>
      </div>

      {/* 密钥信息 */}
      <div className="flex items-center gap-1.5 text-xs text-[color:var(--text-faint)]">
        <ShieldCheck size={11} />
        <span>{p.auth_token_mask || '未设置密钥'}</span>
      </div>

      {/* 操作栏 */}
      <div className="flex items-center gap-1.5 pt-0.5 border-t border-[color:var(--line)]/50">
        {!p.is_active && (
          <Button size="sm" variant="soft" onClick={onActivate} className="text-xs">
            <Zap size={12} /> 激活
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={onTest} className="text-xs gap-1">
          {testIcon ? (
            <span className={cn(
              testState === 'testing' && 'animate-spin',
              testState === 'success' && 'text-emerald-500',
              testState === 'fail' && 'text-red-500',
            )}>
              {testState === 'testing' && <Loader2 size={12} />}
              {testState === 'success' && <CheckCircle2 size={12} />}
              {testState === 'fail' && <AlertCircle size={12} />}
            </span>
          ) : null}
          {testState === 'testing' ? '测试中…' : testState === 'success' ? '连接成功' : testState === 'fail' ? '连接失败' : '测试连接'}
        </Button>
        <div className="flex-1" />
        <Button size="sm" variant="ghost" onClick={onEdit} className="opacity-0 group-hover:opacity-100 transition-opacity">
          <Pencil size={13} />
        </Button>
        <Button size="sm" variant="ghost" onClick={onDelete} className="opacity-0 group-hover:opacity-100 transition-opacity text-red-500/70 hover:text-red-500">
          <Trash2 size={13} />
        </Button>
      </div>
    </Card>
  );
}

// ─── 供应商选择网格 ──────────────────────────────────────────────
function ProviderGrid({ providers, selectedId, onSelect }) {
  const [search, setSearch] = useState('');

  const filtered = useMemo(() => {
    if (!search.trim()) return providers;
    const q = search.toLowerCase();
    return providers.filter((p) =>
      p.name.toLowerCase().includes(q) || p.code.toLowerCase().includes(q) ||
      (getProviderTheme(p.code).label || '').toLowerCase().includes(q)
    );
  }, [providers, search]);

  const anthropicProviders = filtered.filter((p) => p.protocol === 'anthropic');
  const openaiProviders = filtered.filter((p) => p.protocol === 'openai');

  return (
    <div className="space-y-4">
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[color:var(--text-faint)]" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索供应商…"
          className="pl-9"
          autoFocus
        />
      </div>

      {anthropicProviders.length > 0 && (
        <ProviderSection title="Anthropic 协议（直连）" providers={anthropicProviders} selectedId={selectedId} onSelect={onSelect} />
      )}
      {openaiProviders.length > 0 && (
        <ProviderSection title="OpenAI 兼容协议（经本地路由层翻译）" providers={openaiProviders} selectedId={selectedId} onSelect={onSelect} />
      )}

      {filtered.length === 0 && (
        <div className="text-center py-8 text-sm text-[color:var(--text-faint)]">
          未找到匹配的供应商
        </div>
      )}
    </div>
  );
}

function ProviderSection({ title, providers, selectedId, onSelect }) {
  return (
    <div>
      <div className="text-xs font-medium text-[color:var(--text-faint)] mb-2 uppercase tracking-wider">{title}</div>
      <div className="grid grid-cols-3 gap-2">
        {providers.map((p) => {
          const theme = getProviderTheme(p.code);
          const Icon = theme.icon;
          const selected = selectedId === p.id;
          return (
            <button
              key={p.id}
              onClick={() => onSelect(p)}
              className={cn(
                'surface p-3 text-left transition-all duration-200 hover:-translate-y-0.5 cursor-pointer',
                selected
                  ? 'ring-2 ring-[color:var(--accent)] border-[color:var(--accent)] shadow-glow'
                  : 'hover:border-[color:var(--accent)]/50 hover:shadow-md'
              )}
            >
              <div className={cn('w-9 h-9 rounded-lg flex items-center justify-center bg-gradient-to-br text-white mb-2', theme.gradient)}>
                <Icon size={16} />
              </div>
              <div className="text-sm font-medium truncate">{theme.label || p.name}</div>
              <div className="text-[11px] text-[color:var(--text-faint)] truncate mt-0.5">
                {p.default_model || '—'}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ─── 两步式编辑器弹窗 ────────────────────────────────────────────
function ProfileEditor({ providers, profile, onClose, onSaved }) {
  const isEdit = !!profile;
  const [step, setStep] = useState(isEdit ? 2 : 1);
  const [selectedProvider, setSelectedProvider] = useState(
    isEdit ? providers.find((p) => p.id === profile.provider_id) : null
  );
  const [name, setName] = useState(profile?.name || '');
  const [baseUrl, setBaseUrl] = useState(profile?.base_url || '');
  const [model, setModel] = useState(profile?.model || '');
  const [token, setToken] = useState('');
  const [transformer, setTransformer] = useState(profile?.transformer || '');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);
  const pushNotification = useStore((s) => s.pushNotification);

  const isOpenAI = selectedProvider?.protocol === 'openai';

  const handlePickProvider = (p) => {
    setSelectedProvider(p);
    if (!isEdit) {
      setName(p.name);
      setBaseUrl(p.default_base_url || '');
      setModel(p.default_model || '');
      try {
        const meta = JSON.parse(p.usage_api_meta || '{}');
        if (meta.transformer) setTransformer(meta.transformer);
      } catch {}
    }
    setStep(2);
  };

  const handleSave = async () => {
    if (!name.trim()) return pushNotification({ title: '请填写名称', body: '' });
    if (!selectedProvider) return pushNotification({ title: '请选择供应商', body: '' });
    if (!isEdit && !token.trim()) return pushNotification({ title: '请填写密钥', body: '' });
    setSaving(true);
    try {
      let cipher = '';
      let mask = profile?.auth_token_mask || '';
      if (token) {
        cipher = await electron.encryptSecret(token);
        mask = maskToken(token);
      }
      await api.saveProfile({
        id: profile?.id || 0,
        name,
        provider_id: selectedProvider.id,
        base_url: baseUrl,
        model,
        auth_token_cipher: cipher,
        auth_token_mask: mask,
        extra: '{}',
        transformer: isOpenAI ? transformer : '',
      });
      if (profile?.is_active && token) {
        await electron.pushActiveSecret(profile.id);
      }
      pushNotification({ title: isEdit ? '已保存修改' : '已添加接入点', body: name });
      onSaved();
    } catch (e) {
      pushNotification({ title: '保存失败', body: e.message });
    } finally {
      setSaving(false);
    }
  };

  const theme = selectedProvider ? getProviderTheme(selectedProvider.code) : null;

  // 第一步：选择供应商
  if (step === 1) {
    return (
      <Modal open onClose={onClose} title="选择供应商" width={680}>
        <ProviderGrid
          providers={providers}
          selectedId={selectedProvider?.id}
          onSelect={handlePickProvider}
        />
      </Modal>
    );
  }

  // 第二步：填写配置
  const SelIcon = theme?.icon || Cpu;
  return (
    <Modal
      open
      onClose={onClose}
      title={isEdit ? '编辑接入点' : '配置接入点'}
      width={520}
      footer={
        <>
          {!isEdit && (
            <Button variant="ghost" onClick={() => setStep(1)} className="mr-auto">
              <ArrowLeft size={14} /> 重选供应商
            </Button>
          )}
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? <Loader2 size={14} className="animate-spin" /> : null}
            {isEdit ? '保存' : '添加'}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        {/* 当前供应商摘要 */}
        <div className="flex items-center gap-3 p-3 rounded-xl bg-[color:var(--bg-soft)] border border-[color:var(--line)]">
          <div className={cn('w-10 h-10 rounded-lg flex items-center justify-center bg-gradient-to-br text-white', theme?.gradient)}>
            <SelIcon size={18} />
          </div>
          <div className="min-w-0">
            <div className="font-medium text-sm">{selectedProvider?.name}</div>
            <div className="text-xs text-[color:var(--text-faint)] flex items-center gap-1.5">
              <Badge tone={isOpenAI ? 'info' : 'default'} className="!text-[10px] !px-1.5 !py-0">
                {isOpenAI ? 'OpenAI 兼容' : 'Anthropic'}
              </Badge>
              {selectedProvider?.default_model && (
                <span className="truncate">{selectedProvider.default_model}</span>
              )}
            </div>
          </div>
          {!isEdit && (
            <button
              onClick={() => setStep(1)}
              className="ml-auto text-xs text-[color:var(--accent)] hover:underline shrink-0"
            >
              更换
            </button>
          )}
        </div>

        {/* 密钥 - 最突出的输入 */}
        <Field label={isEdit ? '密钥（留空则保留旧值）' : 'API Key'}>
          <div className="relative">
            <Input
              type={showToken ? 'text' : 'password'}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={isEdit ? '••••••••（不修改请留空）' : 'sk-...'}
              autoComplete="off"
              className="pr-9 h-11 text-base"
              autoFocus={!isEdit}
            />
            <button
              type="button"
              onClick={() => setShowToken((v) => !v)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 w-7 h-7 inline-flex items-center justify-center rounded-md text-[color:var(--text-faint)] hover:bg-[color:var(--bg-soft)] hover:text-[color:var(--text)]"
            >
              {showToken ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
          </div>
          <div className="mt-1.5 flex items-center justify-between">
            <div className="text-[11px] text-[color:var(--text-faint)] flex items-center gap-1">
              <ShieldCheck size={11} /> Keychain 加密存储
            </div>
            {selectedProvider?.doc_url && (
              <button
                onClick={() => electron.openExternal(selectedProvider.doc_url)}
                className="inline-flex items-center gap-1 text-[11px] text-[color:var(--accent)] hover:underline"
              >
                <ExternalLink size={10} /> 获取密钥
              </button>
            )}
          </div>
        </Field>

        {/* 高级设置折叠区 */}
        <div>
          <button
            type="button"
            onClick={() => setShowAdvanced((v) => !v)}
            className="flex items-center gap-1 text-xs text-[color:var(--text-soft)] hover:text-[color:var(--text)] transition-colors"
          >
            {showAdvanced ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            高级设置
            <span className="text-[color:var(--text-faint)]">（名称、模型、URL）</span>
          </button>
          {showAdvanced && (
            <div className="mt-3 space-y-3 pl-0.5">
              <Field label="名称">
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={selectedProvider?.name || ''} />
              </Field>
              <Field label="模型">
                <ModelComboBox
                  value={model}
                  onChange={setModel}
                  providerCode={selectedProvider?.code}
                  placeholder={selectedProvider?.default_model || ''}
                />
              </Field>
              <Field label="Base URL">
                <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder={selectedProvider?.default_base_url || 'https://...'} />
              </Field>
              {isOpenAI && (
                <Field label="Transformer">
                  <Input value={transformer} onChange={(e) => setTransformer(e.target.value)} placeholder="留空 = 自动" />
                </Field>
              )}
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}

// ─── 模型选择 ComboBox ──────────────────────────────────────────
function ModelComboBox({ value, onChange, providerCode, placeholder }) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const groups = PROVIDER_MODELS[providerCode] || [];

  const filtered = groups.map(g => ({
    ...g,
    models: g.models.filter(m => m.toLowerCase().includes(filter.toLowerCase())),
  })).filter(g => g.models.length > 0);

  return (
    <div className="relative">
      <Input
        value={value}
        onChange={(e) => { onChange(e.target.value); setFilter(e.target.value); }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 200)}
        placeholder={placeholder}
      />
      {open && groups.length > 0 && (
        <div className="absolute z-50 top-full left-0 right-0 mt-1 max-h-[240px] overflow-y-auto rounded-lg border border-[color:var(--line)] bg-[color:var(--bg-elev)] shadow-lg scrollable">
          {filtered.length === 0 && (
            <div className="px-3 py-2 text-xs text-[color:var(--text-faint)]">无匹配模型（可手动输入）</div>
          )}
          {filtered.map(g => (
            <div key={g.group}>
              <div className="px-3 py-1 text-[10px] font-semibold text-[color:var(--text-faint)] uppercase tracking-wider bg-[color:var(--bg-soft)]">{g.group}</div>
              {g.models.map(m => (
                <button key={m} type="button"
                  onMouseDown={(e) => { e.preventDefault(); onChange(m); setOpen(false); }}
                  className={cn('w-full text-left px-3 py-1.5 text-sm hover:bg-[color:var(--accent-soft)] transition',
                    m === value && 'text-[color:var(--accent)] font-medium'
                  )}
                >{m}</button>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ─── 工具组件 ────────────────────────────────────────────────────
function Field({ label, children }) {
  return (
    <label className="block">
      <div className="text-xs font-medium text-[color:var(--text-soft)] mb-1.5">{label}</div>
      {children}
    </label>
  );
}

function maskToken(t) {
  if (!t) return '';
  if (t.length <= 8) return '****';
  return t.slice(0, 4) + '••••' + t.slice(-4);
}
