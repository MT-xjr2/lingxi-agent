import { useStore } from '../state/useStore';
import { Card } from '../ui/primitives';
import { cn } from '../ui/cn';
import { Sun, Moon, Sparkles, Zap, Leaf, Star } from 'lucide-react';

const THEMES = [
  { id: 'light',    name: '浅色',       icon: Sun,      preview: 'linear-gradient(135deg,#ffffff,#eef0f6)',               desc: '清新明亮' },
  { id: 'dark',     name: '深色',       icon: Moon,     preview: 'linear-gradient(135deg,#181c28,#0b0d12)',               desc: '护眼舒适' },
  { id: 'midnight', name: '午夜紫',     icon: Sparkles, preview: 'linear-gradient(135deg,#1a1140,#0b0d12)',               desc: '深邃优雅' },
  { id: 'cyber',    name: '赛博朋克',   icon: Zap,      preview: 'linear-gradient(135deg,#050510,#00e5ff)',               desc: '霓虹蓝绿' },
  { id: 'aurora',   name: '极光',       icon: Leaf,     preview: 'linear-gradient(135deg,#060d16 30%,#4ade80,#22d3ee)',   desc: '自然绿意' },
  { id: 'cosmos',   name: '星空',       icon: Star,     preview: 'linear-gradient(135deg,#04040c 30%,#c084fc,#f0abfc)',   desc: '梦幻紫粉' },
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
            const active = theme === t.id;
            return (
              <button
                key={t.id}
                onClick={() => setTheme(t.id)}
                className={cn(
                  'surface p-3 text-left transition-all duration-200 hover:-translate-y-0.5',
                  active ? 'ring-2 ring-[color:var(--accent)] border-[color:var(--accent)] shadow-glow' : 'hover:border-[color:var(--accent)]'
                )}
              >
                <div className="h-20 rounded-md mb-2 relative overflow-hidden" style={{ background: t.preview }}>
                  {active && (
                    <div className="absolute inset-0 flex items-center justify-center bg-black/20">
                      <span className="text-white text-xs font-medium px-2 py-0.5 rounded-full bg-white/20 backdrop-blur">当前</span>
                    </div>
                  )}
                </div>
                <div className="text-sm font-medium flex items-center gap-2"><I size={14} /> {t.name}</div>
                <div className="text-xs text-[color:var(--text-faint)] mt-0.5">{t.desc}</div>
              </button>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
