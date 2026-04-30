import { BlocksRenderer, parseAssistantContent, UsageFooter } from './blocks';

export function UserBubble({ content }) {
  return (
    <div className="flex justify-end my-3">
      <div className="user-bubble">{content}</div>
    </div>
  );
}

export function AssistantBubble({ message, live = false, liveBlocks = null }) {
  const blocks = liveBlocks || parseAssistantContent(message?.content || '');
  return (
    <div className="flex justify-start my-3">
      <div className="assistant-bubble">
        <BlocksRenderer blocks={blocks} live={live} />
        {!live && message?.usage && <UsageFooter usageJSON={message.usage} />}
      </div>
    </div>
  );
}
