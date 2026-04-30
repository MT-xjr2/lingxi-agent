import { useStore } from '../state/useStore';
import { Card } from '../ui/primitives';
import { Sun, Moon, Sparkles } from 'lucide-react';

const THEMES = [
  { id: 'light',    name: '浅色',     icon: Sun,      preview: 'linear-gradient(135deg,#ffffff,#eef0f6)' },
  { id: 'dark',     name: '深色',     icon: Moon,     preview: 'linear-gradient(135deg,#181c28,#0b0d12)' },
  { id: 'midnight', name: '午夜紫',   icon: Sparkles, preview: 'linear-gradient(135deg,#1a1140,#0b0d12)' },
];

export function AppearancePage() {
  const theme = useStore((s) => s.theme);
  const setTheme = useStore((s) => s.setTheme);
  return (
    <div className="max-w-3xl mx-auto py-6 px-6">
      <h1 className="text-xl font-semibold mb-4">外观</h1>
      <Card>
        <div className="font-medium mb-3">主题</div>
        <div className="grid grid-cols-3 gap-3">
          {THEMES.map((t) => {
            const I = t.icon;
            return (
              <button
                key={t.id}
                onClick={() => setTheme(t.id)}
                className={`surface p-3 text-left transition ${theme === t.id ? 'ring-2 ring-[color:var(--accent)] border-[color:var(--accent)]' : 'hover:border-[color:var(--accent)]'}`}
              >
                <div className="h-20 rounded-md mb-2" style={{ background: t.preview }} />
                <div className="text-sm font-medium flex items-center gap-2"><I size={14} /> {t.name}</div>
              </button>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
