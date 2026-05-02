import { useStore } from '../state/useStore';
import { Cpu, BarChart3, Palette } from 'lucide-react';
import { ProfilesPage } from './ProfilesPage';
import { UsagePage } from './UsagePage';
import { AppearancePage } from './AppearancePage';
import { cn } from '../ui/cn';

const TABS = [
  { id: 'profiles',   label: '模型与接入点', icon: Cpu },
  { id: 'usage',      label: '用量',         icon: BarChart3 },
  { id: 'appearance', label: '外观',         icon: Palette },
];

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
        {tab === 'usage' && <UsagePage />}
        {tab === 'appearance' && <AppearancePage />}
      </div>
    </div>
  );
}
