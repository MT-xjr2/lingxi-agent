import { useStore } from '../state/useStore';
import { Brain, Search, Hammer, CheckCircle2 } from 'lucide-react';
import { AnimatePresence, motion } from 'framer-motion';
import { cn } from '../ui/primitives';

const MAP = {
  THINKING:  { label: '正在思考', Icon: Brain,  cls: 'text-[color:var(--accent)]' },
  CHECKING:  { label: '正在查阅', Icon: Search, cls: 'text-sky-500' },
  EXECUTING: { label: '执行技能', Icon: Hammer, cls: 'text-amber-500' },
  DONE:      { label: '已完成',   Icon: CheckCircle2, cls: 'text-emerald-500' },
};

export function AgentStatePill() {
  const state = useStore((s) => s.agentState);
  const isStreaming = useStore((s) => s.isStreaming);
  if (!isStreaming && state !== 'DONE') return null;
  const conf = MAP[state] || MAP.THINKING;
  const { Icon } = conf;
  return (
    <AnimatePresence>
      <motion.div
        key={state}
        initial={{ opacity: 0, y: -6 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0 }}
        className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full surface-soft text-xs font-medium"
      >
        <Icon size={14} className={cn(conf.cls, isStreaming && 'animate-breathe')} />
        <span className="text-[color:var(--text-soft)]">{conf.label}</span>
      </motion.div>
    </AnimatePresence>
  );
}
