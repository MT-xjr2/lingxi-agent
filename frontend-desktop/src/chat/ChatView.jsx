import { MessageList } from './MessageList';
import { Composer } from './Composer';

export function ChatView() {
  return (
    <div className="flex-1 flex flex-col min-h-0">
      <MessageList />
      <Composer />
    </div>
  );
}
