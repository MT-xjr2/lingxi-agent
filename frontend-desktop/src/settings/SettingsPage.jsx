import { useState, useEffect, useCallback } from 'react';
import { useStore } from '../state/useStore';
import { Cpu, BarChart3, Palette, BrainCircuit, Wifi, Info, RefreshCw, Download, CheckCircle2, Loader2 } from 'lucide-react';
import { ProfilesPage } from './ProfilesPage';
import { UsagePage } from './UsagePage';
import { AppearancePage } from './AppearancePage';
import { MemoryPage } from './MemoryPage';
import NexusSettingsPage from './NexusSettingsPage';
import { cn } from '../ui/cn';
import { Button, Card } from '../ui/primitives';

const TABS = [
  { id: 'profiles',   label: '模型与接入点', icon: Cpu },
  { id: 'memory',     label: '长期记忆',     icon: BrainCircuit },
  { id: 'nexus',      label: '网络与协作',   icon: Wifi },
  { id: 'usage',      label: '用量',         icon: BarChart3 },
  { id: 'appearance', label: '外观',         icon: Palette },
  { id: 'about',      label: '关于与更新',   icon: Info },
];

function AboutPage() {
  const [version, setVersion] = useState('');
  const [updateStatus, setUpdateStatus] = useState(null);

  useEffect(() => {
    window.electronAPI?.getVersion?.().then(v => setVersion(v || ''));
    const unsub = window.electronAPI?.onUpdateStatus?.((data) => {
      setUpdateStatus(data);
    });
    return () => { unsub?.(); };
  }, []);

  const handleCheckUpdate = useCallback(() => {
    setUpdateStatus({ status: 'checking' });
    window.electronAPI?.checkForUpdate?.().then((result) => {
      if (result?.status === 'dev') {
        setUpdateStatus({ status: 'dev' });
      }
    }).catch(() => {
      setUpdateStatus({ status: 'error', error: '检查失败' });
    });
  }, []);

  const statusLabel = (() => {
    if (!updateStatus) return null;
    switch (updateStatus.status) {
      case 'checking': return { icon: Loader2, text: '正在检查更新...', cls: 'text-[color:var(--text-soft)]', spin: true };
      case 'available': return { icon: Download, text: `发现新版本 ${updateStatus.version}`, cls: 'text-[color:var(--accent)]' };
      case 'downloading': return { icon: Download, text: `正在下载 ${updateStatus.percent || 0}%`, cls: 'text-[color:var(--accent)]' };
      case 'downloaded': return { icon: CheckCircle2, text: `${updateStatus.version} 已下载，重启即可更新`, cls: 'text-emerald-600' };
      case 'up-to-date': return { icon: CheckCircle2, text: '已是最新版本', cls: 'text-emerald-600' };
      case 'dev': return { icon: Info, text: '开发模式，跳过更新检查', cls: 'text-amber-600' };
      case 'error': return { icon: Info, text: `更新出错: ${updateStatus.error || '未知错误'}`, cls: 'text-red-500' };
      default: return null;
    }
  })();

  return (
    <div className="p-6 max-w-xl space-y-6">
      <div>
        <h2 className="text-lg font-bold text-[color:var(--text)]">关于灵犀</h2>
        <p className="text-sm text-[color:var(--text-soft)] mt-1">本地优先的桌面 AI Agent 工作台</p>
      </div>
      <Card className="p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium text-[color:var(--text)]">当前版本</div>
            <div className="text-xs text-[color:var(--text-faint)] mt-0.5">v{version || '—'}</div>
          </div>
          <Button variant="ghost" size="sm" onClick={handleCheckUpdate}
            disabled={updateStatus?.status === 'checking' || updateStatus?.status === 'downloading'}>
            <RefreshCw size={14} className={cn(updateStatus?.status === 'checking' && 'animate-spin')} /> 检查更新
          </Button>
        </div>
        {statusLabel && (
          <div className={cn('flex items-center gap-2 text-sm', statusLabel.cls)}>
            <statusLabel.icon size={14} className={statusLabel.spin ? 'animate-spin' : ''} />
            <span>{statusLabel.text}</span>
          </div>
        )}
        {updateStatus?.status === 'downloading' && updateStatus.percent > 0 && (
          <div className="w-full h-1.5 rounded-full bg-[color:var(--bg-soft)] overflow-hidden">
            <div className="h-full rounded-full bg-[color:var(--accent)] transition-all duration-300"
              style={{ width: `${updateStatus.percent}%` }} />
          </div>
        )}
      </Card>
      <div className="text-xs text-[color:var(--text-faint)] space-y-1">
        <p>Electron + React + Go</p>
      </div>
    </div>
  );
}

export function SettingsPage() {
  const tab = useStore((s) => s.settingsTab);
  const setTab = useStore((s) => s.setSettingsTab);

  return (
    <div className="flex-1 flex min-h-0">
      <aside className="w-56 border-r border-[color:var(--line)] py-4 px-2 shrink-0">
        <div className="px-3 pb-2 text-xs text-[color:var(--text-faint)]">设置</div>
        <nav className="space-y-0.5">
          {TABS.map((t) => {
            const Icon = t.icon;
            const active = tab === t.id;
            return (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={cn(
                  'w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition',
                  active
                    ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent)] font-medium'
                    : 'hover:bg-[color:var(--bg-soft)] text-[color:var(--text)]'
                )}
              >
                <Icon size={14} /> {t.label}
              </button>
            );
          })}
        </nav>
      </aside>
      <div className="flex-1 overflow-y-auto scrollable">
        {tab === 'profiles' && <ProfilesPage />}
        {tab === 'memory' && <MemoryPage />}
        {tab === 'nexus' && <div className="p-6"><NexusSettingsPage /></div>}
        {tab === 'usage' && <UsagePage />}
        {tab === 'appearance' && <AppearancePage />}
        {tab === 'about' && <AboutPage />}
      </div>
    </div>
  );
}
