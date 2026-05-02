import { useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useStore, initStore } from '../state/useStore';
import { SidebarSessions } from './SidebarSessions';
import { ModelSwitcher } from './ModelSwitcher';
import { RouterPill } from './RouterPill';
import { ChatView } from '../chat/ChatView';
import { AgentStatePill } from '../chat/AgentStatePill';
import { SettingsPage } from '../settings/SettingsPage';
import SkillsPage from '../SkillsPage';
import KnowledgePage from '../KnowledgePage';
import IMConnectorPage from '../IMConnectorPage';
import MCPPage from '../MCPPage';
import AgentFactoryPage from '../AgentFactoryPage';
import { ToastStack } from './primitives';
import { cn } from './cn';
import { MessageSquare, Settings as SettingsIcon, Brain, BookOpen, MessageCircle, Plug, Sparkles } from 'lucide-react';

const pageMotion = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -4 },
  transition: { duration: 0.18, ease: [0.2, 0.8, 0.2, 1] },
};

const NAV_GROUPS = [
  {
    label: '工作台',
    items: [
      { id: 'chat', label: '对话', icon: MessageSquare, hint: '日常问答与任务协作' },
      { id: 'agents', label: '智能体', icon: Sparkles, hint: '创建与管理专属角色' },
    ],
  },
  {
    label: '能力管理',
    items: [
      { id: 'skills', label: '技能', icon: Brain, hint: '扩展可执行能力' },
      { id: 'knowledge', label: '知识库', icon: BookOpen, hint: '上传资料与检索增强' },
      { id: 'mcp', label: 'MCP', icon: Plug, hint: '连接外部工具服务' },
      { id: 'im', label: 'IM 接入', icon: MessageCircle, hint: '飞书/钉钉/企微接入' },
    ],
  },
  {
    label: '系统',
    items: [
      { id: 'settings', label: '设置', icon: SettingsIcon, hint: '模型、用量与外观' },
    ],
  },
];

export function AppShell() {
  const view = useStore((s) => s.view);
  const setView = useStore((s) => s.setView);
  const notifications = useStore((s) => s.notifications);

  useEffect(() => {
    initStore();
  }, []);

  return (
    <div className="h-screen flex flex-col bg-[color:var(--bg)]">
      {/* 顶部栏（macOS 红绿灯让出空间） */}
      <header className="app-drag h-12 flex items-center justify-between px-4 border-b border-[color:var(--line)] glass relative">
        <div className="absolute left-0 right-0 top-0 h-px bg-gradient-to-r from-transparent via-[color:var(--accent)]/40 to-transparent" />
        <div className="flex items-center gap-2 pl-16">
          <img src="/logo.png" alt="灵犀" className="w-7 h-7 rounded-lg shadow-soft ring-1 ring-[color:var(--accent-soft)]" />
          <div className="text-sm font-semibold tracking-tight text-gradient">灵犀</div>
          <div className="ml-3"><AgentStatePill /></div>
        </div>
        <div className="app-no-drag flex items-center gap-2">
          <RouterPill />
          <ModelSwitcher />
        </div>
      </header>

      <div className="flex-1 flex min-h-0">
        {/* 左侧导航 + 会话列表 */}
        <aside className="w-64 shrink-0 border-r border-[color:var(--line)] bg-[color:var(--bg-elev)]/80 backdrop-blur flex flex-col">
          <nav className="px-3 pt-3 pb-2 space-y-4" aria-label="主导航">
            {NAV_GROUPS.map((group) => (
              <div key={group.label}>
                <div className="px-2 mb-1.5 text-[11px] font-medium tracking-wide text-[color:var(--text-faint)]">
                  {group.label}
                </div>
                <div className="space-y-1">
                  {group.items.map((n) => {
                    const Icon = n.icon;
                    const active = view === n.id;
                    return (
                      <button
                        key={n.id}
                        onClick={() => setView(n.id)}
                        className={cn(
                          'w-full flex items-center gap-3 px-2.5 py-2 rounded-xl text-left transition-all duration-200',
                          active
                            ? 'bg-gradient-to-r from-[color:var(--accent-soft)] to-transparent text-[color:var(--accent)] shadow-[inset_0_0_0_1px_var(--accent-soft)]'
                            : 'text-[color:var(--text-soft)] hover:bg-[color:var(--bg-soft)] hover:text-[color:var(--text)]'
                        )}
                        aria-current={active ? 'page' : undefined}
                        title={`${n.label} · ${n.hint}`}
                      >
                        <span className={cn(
                          'w-8 h-8 rounded-lg flex items-center justify-center shrink-0',
                          active ? 'bg-[color:var(--bg-elev)] shadow-soft' : 'bg-[color:var(--bg-soft)]'
                        )}>
                          <Icon size={16} />
                        </span>
                        <span className="min-w-0">
                          <span className="block text-sm font-medium truncate">{n.label}</span>
                          <span className="block text-[11px] text-[color:var(--text-faint)] truncate">{n.hint}</span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </nav>
          <div className="flex-1 min-h-0">
            {(view === 'chat') ? <SidebarSessions /> : null}
          </div>
        </aside>

        {/* 主区 */}
        <main className="flex-1 flex flex-col min-h-0 relative">
          <AnimatePresence mode="wait">
            {view === 'chat' && (
              <motion.div key="chat" className="flex-1 flex flex-col min-h-0" {...pageMotion}>
                <ChatView />
              </motion.div>
            )}
            {view === 'settings' && (
              <motion.div key="settings" className="flex-1 flex flex-col min-h-0" {...pageMotion}>
                <SettingsPage />
              </motion.div>
            )}
            {view === 'agents' && (
              <motion.div key="agents" className="flex-1 overflow-auto scrollable bg-[color:var(--bg)] p-6" {...pageMotion}>
                <AgentFactoryPage onBack={() => setView('chat')} />
              </motion.div>
            )}
            {view === 'mcp' && (
              <motion.div key="mcp" className="flex-1 overflow-auto scrollable bg-[color:var(--bg)] p-6" {...pageMotion}>
                <MCPPage onBack={() => setView('chat')} />
              </motion.div>
            )}
            {view === 'skills' && (
              <motion.div key="skills" className="flex-1 overflow-auto scrollable bg-[color:var(--bg)] p-4" {...pageMotion}>
                <SkillsPage />
              </motion.div>
            )}
            {view === 'knowledge' && (
              <motion.div key="knowledge" className="flex-1 overflow-auto scrollable bg-[color:var(--bg)] p-4" {...pageMotion}>
                <KnowledgePage />
              </motion.div>
            )}
            {view === 'im' && (
              <motion.div key="im" className="flex-1 overflow-auto scrollable bg-[color:var(--bg)] p-4" {...pageMotion}>
                <IMConnectorPage />
              </motion.div>
            )}
          </AnimatePresence>
        </main>
      </div>

      <ToastStack items={notifications} />
    </div>
  );
}
