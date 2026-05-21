import { Crown } from 'lucide-react';
import { Modal, Badge } from '../ui/primitives';
import GroupMemberAvatar from './GroupMemberAvatar';

export default function GroupMemberDrawer({ open, onClose, room, members, onDoubleClickAvatar }) {
  if (!open) return null;
  const joined = (members || []).filter((m) => m.status === 'joined' || m.role === 'human');
  const invited = (members || []).filter((m) => m.status === 'invited');

  return (
    <Modal open={open} onClose={onClose} title={`群成员（${joined.length}）`} width={420} footer={null}>
      <div className="space-y-1 max-h-[60vh] overflow-y-auto scrollable">
        {joined.map((m) => (
          <div
            key={m.id || `human-${m.agent_name}`}
            onDoubleClick={() => onDoubleClickAvatar?.(m)}
            className="flex items-center gap-3 px-2 py-2 rounded-lg hover:bg-[color:var(--bg-soft)] cursor-pointer"
          >
            <GroupMemberAvatar member={m} size={40} />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate flex items-center gap-1.5">
                {m.display_name || m.agent_name}
                {m.role === 'human' && (
                  <Badge tone="success">人类</Badge>
                )}
                {room?.host_peer_id === m.peer_id && m.role !== 'human' && (
                  <Crown size={11} className="text-amber-500" />
                )}
              </div>
              <div className="text-[11px] text-[color:var(--text-faint)] truncate">
                {m.role === 'human' ? '你（可 @ 昵称插话）' : (m.peer_nickname || m.peer_id || 'Agent')}
              </div>
            </div>
            {m.role === 'human' ? (
              <Badge tone="success">你</Badge>
            ) : m.is_local ? (
              <Badge tone="accent">本端</Badge>
            ) : null}
          </div>
        ))}
        {invited.length > 0 && (
          <>
            <div className="text-[11px] text-[color:var(--text-faint)] mt-3 px-2">待加入</div>
            {invited.map((m) => (
              <div key={m.id} className="flex items-center gap-3 px-2 py-2 rounded-lg opacity-70">
                <GroupMemberAvatar member={m} size={40} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm truncate">{m.display_name || m.agent_name}</div>
                </div>
                <Badge tone="warn">待加入</Badge>
              </div>
            ))}
          </>
        )}
      </div>
      <div className="text-[11px] text-[color:var(--text-faint)] mt-3 text-center">
        双击成员可在输入框中 @ ta
      </div>
    </Modal>
  );
}
