import { create } from 'zustand';
import { api, wsClient } from '../api/client';

// 快捷回复建议生成（基于助手回复内容关键词匹配）
function generateQuickReplies(text) {
  if (!text || text.length < 10) return [];
  const replies = [];

  if (/代码|函数|代码块|实现|编程|程序/.test(text)) {
    replies.push('请解释这段代码的工作原理');
    replies.push('能否优化一下这段代码？');
    replies.push('请为这段代码添加注释');
  } else if (/翻译|translation/i.test(text)) {
    replies.push('翻译得很好，再翻译一段');
    replies.push('改为更口语化的表达');
    replies.push('帮我校对一下语法');
  } else if (/步骤|方案|计划|方法|建议/.test(text)) {
    replies.push('请详细展开第一步');
    replies.push('有没有其他替代方案？');
    replies.push('请总结一下要点');
  } else if (/表格|数据|分析|统计/.test(text)) {
    replies.push('请用图表展示');
    replies.push('帮我进一步分析');
    replies.push('导出为 CSV 格式');
  } else if (/总结|摘要|要点/.test(text)) {
    replies.push('请更详细地展开');
    replies.push('能否用列表形式重新整理？');
  } else {
    replies.push('继续');
    replies.push('请详细说明');
    replies.push('帮我总结一下');
  }

  return replies.slice(0, 3);
}

// 全局状态：会话、当前激活档案、用量、消息流
export const useStore = create((set, get) => ({
  // ─── 主题 ────────────────────────────────────────────────────
  theme: localStorage.getItem('lingxi-theme') || 'light',
  setTheme: (t) => {
    localStorage.setItem('lingxi-theme', t);
    document.documentElement.setAttribute('data-theme', t);
    set({ theme: t });
  },

  // ─── 视图 ────────────────────────────────────────────────────
  view: 'chat', // chat | settings | skills | knowledge | im
  setView: (v) => set({ view: v }),
  settingsTab: 'profiles', // profiles | usage | appearance
  setSettingsTab: (t) => set({ settingsTab: t }),
  sidebarCollapsed: false,
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

  // ─── 会话 ────────────────────────────────────────────────────
  sessions: [],
  activeSessionId: null,
  setActiveSession: async (id) => {
    set({ activeSessionId: id, messages: [], liveBlocks: [] });
    if (id) {
      wsClient.subscribe(id);
      const msgs = await api.listMessages(id).catch(() => []);
      set({ messages: msgs });
    }
  },
  refreshSessions: async () => {
    const agentId = get().activeAgentId;
    const sessions = await api.listSessions(agentId).catch(() => []);
    set({ sessions });
    return sessions;
  },
  createSession: async (titleOrPayload) => {
    const activeAgentId = get().activeAgentId || 0;
    const payload = typeof titleOrPayload === 'string'
      ? { title: titleOrPayload || '新对话', agent_id: activeAgentId }
      : { title: '新对话', agent_id: activeAgentId, ...(titleOrPayload || {}) };
    const r = await api.createSession(payload);
    await get().refreshSessions();
    await get().setActiveSession(r.id);
    return r.id;
  },
  deleteSession: async (id) => {
    await api.deleteSession(id);
    const list = await get().refreshSessions();
    if (get().activeSessionId === id) {
      const next = list[0]?.id || null;
      await get().setActiveSession(next);
    }
  },
  batchDeleteSessions: async (ids) => {
    if (!ids || ids.length === 0) return;
    await api.batchDeleteSessions(ids);
    const list = await get().refreshSessions();
    if (ids.includes(get().activeSessionId)) {
      const next = list[0]?.id || null;
      await get().setActiveSession(next);
    }
  },
  renameSession: async (id, title) => {
    await api.renameSession(id, title);
    await get().refreshSessions();
  },
  pinSession: async (id, pinned) => {
    await api.pinSession(id, pinned);
    await get().refreshSessions();
  },

  // ─── 消息 ────────────────────────────────────────────────────
  messages: [],
  liveBlocks: [], // 流式中的 assistant block 数组：{type:'text'|'thinking'|'tool', text, name, done}
  agentState: 'IDLE', // IDLE | THINKING | CHECKING | EXECUTING | DONE
  isStreaming: false,
  startedAt: null,
  suggestedReplies: [], // 快捷回复建议

  // ─── 档案 ────────────────────────────────────────────────────
  providers: [],
  profiles: [],
  activeProfile: null,
  refreshProfiles: async () => {
    const [providers, profiles] = await Promise.all([
      api.listProviders().catch(() => []),
      api.listProfiles().catch(() => []),
    ]);
    const activeProfile = profiles.find((p) => p.is_active) || null;
    set({ providers, profiles, activeProfile });
  },
  activateProfile: async (id) => {
    await api.activateProfile(id);
    // 让 Electron 推送新明文
    if (window.electronAPI?.pushActiveSecret) {
      await window.electronAPI.pushActiveSecret(id);
    }
    await get().refreshProfiles();
  },

  // ─── 智能体 ────────────────────────────────────────────────
  agents: [],
  activeAgentId: Number(localStorage.getItem('lingxi-active-agent')) || 1,
  refreshAgents: async () => {
    const agents = await api.listAgents().catch(() => []);
    set({ agents });
    // 校正 activeAgentId：如果当前选的 agent 不存在，则回退到第一个 builtin
    const cur = get().activeAgentId;
    if (!agents.find((a) => a.id === cur)) {
      const fallback = (agents.find((a) => a.builtin) || agents[0]);
      if (fallback) {
        localStorage.setItem('lingxi-active-agent', String(fallback.id));
        set({ activeAgentId: fallback.id });
      }
    }
    return agents;
  },
  setActiveAgent: async (agentId) => {
    localStorage.setItem('lingxi-active-agent', String(agentId));
    set({ activeAgentId: agentId, activeSessionId: null, messages: [], liveBlocks: [] });
    const sessions = await get().refreshSessions();
    if (sessions.length > 0) {
      await get().setActiveSession(sessions[0].id);
    }
  },

  // ─── 用量摘要（顶部小标签）─────────────────────────────────
  todayUsage: { input_tokens: 0, output_tokens: 0, cost_usd: 0, requests: 0 },
  refreshTodayUsage: async () => {
    const u = await api.getUsage('today').catch(() => null);
    if (u) set({ todayUsage: u.today || u.summary });
  },

  // ─── 通知（toast） ──────────────────────────────────────────
  notifications: [],
  pushNotification: (n) => {
    const id = Date.now() + Math.random();
    set({ notifications: [...get().notifications, { id, ...n }] });
    setTimeout(() => {
      set({ notifications: get().notifications.filter((x) => x.id !== id) });
    }, 4000);
  },

  // ─── WS 处理 ────────────────────────────────────────────────
  handleWSEvent: (msg) => {
    const { event, data, sessionId } = msg;
    const state = get();

    // 处理远端 Agent 流式 token 转发（广播事件，无 sessionId）
    if (event === 'a2a_remote_stream') {
      try {
        const d = typeof data === 'string' ? JSON.parse(data) : data;
        const convId = d?.conversation_id;
        const streamEvent = d?.event;
        const streamData = d?.data || '';

        if (!convId) return;

        switch (streamEvent) {
          case 'stream_start':
            set({ a2aRemoteIsStreaming: true, a2aRemoteLiveBlocks: [], activeA2AConvId: convId });
            break;
          case 'stream_done':
            set({ a2aRemoteIsStreaming: false, a2aRemoteLiveBlocks: [] });
            break;
          case 'text': {
            const blocks = [...state.a2aRemoteLiveBlocks];
            const last = blocks[blocks.length - 1];
            if (last && last.type === 'text') last.text += streamData;
            else blocks.push({ type: 'text', text: streamData });
            set({ a2aRemoteLiveBlocks: blocks, a2aRemoteIsStreaming: true });
            break;
          }
          case 'thinking': {
            const blocks = [...state.a2aRemoteLiveBlocks];
            const last = blocks[blocks.length - 1];
            if (last && last.type === 'thinking') last.text += streamData;
            else blocks.push({ type: 'thinking', text: streamData });
            set({ a2aRemoteLiveBlocks: blocks, a2aRemoteIsStreaming: true });
            break;
          }
        }
      } catch {}
      return;
    }

    // 路由 A2A 会话的流式事件
    if (sessionId && sessionId === state.activeA2ASessionId && sessionId !== state.activeSessionId) {
      const streamEvents = ['agent_state', 'thinking', 'text', 'tool_start', 'tool_end', 'message_usage', 'done'];
      if (streamEvents.includes(event)) {
        let payload;
        try { payload = data ? JSON.parse(data) : null; } catch { payload = data; }
        switch (event) {
          case 'agent_state': {
            const s = (payload && payload.state) || 'IDLE';
            if (s === 'THINKING' && !state.a2aIsStreaming) {
              set({ a2aIsStreaming: true, a2aLiveBlocks: [] });
            }
            break;
          }
          case 'thinking': {
            const text = typeof payload === 'string' ? payload : (data || '');
            const blocks = [...state.a2aLiveBlocks];
            const last = blocks[blocks.length - 1];
            if (last && last.type === 'thinking') last.text += text;
            else blocks.push({ type: 'thinking', text });
            set({ a2aLiveBlocks: blocks });
            break;
          }
          case 'text': {
            const text = typeof payload === 'string' ? payload : (data || '');
            const blocks = [...state.a2aLiveBlocks];
            const last = blocks[blocks.length - 1];
            if (last && last.type === 'text') last.text += text;
            else blocks.push({ type: 'text', text });
            set({ a2aLiveBlocks: blocks });
            break;
          }
          case 'tool_start': {
            const blocks = [...state.a2aLiveBlocks];
            blocks.push({ type: 'tool', name: payload?.name || '', label: payload?.label || '执行技能', startedAt: Date.now(), done: false });
            set({ a2aLiveBlocks: blocks });
            break;
          }
          case 'tool_end': {
            if (payload?.hidden) break;
            const blocks = [...state.a2aLiveBlocks];
            for (let i = blocks.length - 1; i >= 0; i--) {
              if (blocks[i].type === 'tool' && !blocks[i].done) {
                blocks[i].done = true;
                blocks[i].endedAt = Date.now();
                if (payload && typeof payload === 'object') {
                  if (payload.input != null) blocks[i].input = payload.input;
                  if (payload.label) blocks[i].label = payload.label;
                  if (payload.ms != null) blocks[i].ms = payload.ms;
                  if (payload.status) blocks[i].status = payload.status;
                }
                break;
              }
            }
            set({ a2aLiveBlocks: blocks });
            break;
          }
          case 'message_usage':
          case 'done': {
            if (state.a2aIsStreaming) {
              set({ a2aLiveBlocks: [], a2aIsStreaming: false });
              // 重新拉取消息列表以获取最新的 assistant 消息
              const sid = state.activeA2ASessionId;
              if (sid) {
                api.listMessages(sid).then((m) => set({ a2aMessages: m })).catch(() => {});
              }
            }
            break;
          }
        }
        return;
      }
    }

    if (sessionId && sessionId !== state.activeSessionId) {
      if (event === 'profile_changed') {
        state.refreshProfiles();
      }
      return;
    }
    let payload;
    try { payload = data ? JSON.parse(data) : null; } catch { payload = data; }

    switch (event) {
      case 'agent_state': {
        const s = (payload && payload.state) || 'IDLE';
        set({ agentState: s });
        if (s === 'THINKING' && !state.isStreaming) {
          set({ isStreaming: true, startedAt: Date.now(), liveBlocks: [] });
        }
        break;
      }
      case 'thinking': {
        const text = typeof payload === 'string' ? payload : (data || '');
        const blocks = [...state.liveBlocks];
        const last = blocks[blocks.length - 1];
        if (last && last.type === 'thinking') last.text += text;
        else blocks.push({ type: 'thinking', text });
        set({ liveBlocks: blocks });
        break;
      }
      case 'text': {
        const text = typeof payload === 'string' ? payload : (data || '');
        const blocks = [...state.liveBlocks];
        const last = blocks[blocks.length - 1];
        if (last && last.type === 'text') last.text += text;
        else blocks.push({ type: 'text', text });
        set({ liveBlocks: blocks });
        break;
      }
      case 'tool_start': {
        const blocks = [...state.liveBlocks];
        blocks.push({
          type: 'tool',
          name: payload?.name || '',
          label: payload?.label || '执行技能',
          startedAt: Date.now(),
          done: false,
        });
        set({ liveBlocks: blocks });
        break;
      }
      case 'tool_end': {
        if (payload?.hidden) break;
        const blocks = [...state.liveBlocks];
        for (let i = blocks.length - 1; i >= 0; i--) {
          if (blocks[i].type === 'tool' && !blocks[i].done) {
            blocks[i].done = true;
            blocks[i].endedAt = Date.now();
            if (payload && typeof payload === 'object') {
              if (payload.input != null) blocks[i].input = payload.input;
              if (payload.label) blocks[i].label = payload.label;
              if (payload.ms != null) blocks[i].ms = payload.ms;
              if (payload.status) blocks[i].status = payload.status;
            }
            break;
          }
        }
        set({ liveBlocks: blocks });
        break;
      }
      case 'message_usage': {
        // 把 usage 附到当前流式产生的 assistant 消息上
        const usage = payload?.usage;
        const messageId = payload?.messageId;
        if (!usage) break;
        // 立即把 liveBlocks 固化为一条 assistant 消息（使用 server 给的 messageId）
        const finalBlocks = state.liveBlocks.filter((b) => b.text || b.type === 'tool');
        const newMsg = {
          id: messageId || -Date.now(),
          session_id: state.activeSessionId,
          role: 'assistant',
          content: JSON.stringify(finalBlocks),
          usage: JSON.stringify(usage),
          created_at: new Date().toISOString(),
        };
        set({
          messages: [...state.messages, newMsg],
          liveBlocks: [],
          isStreaming: false,
          agentState: 'DONE',
        });
        state.refreshTodayUsage();
        break;
      }
      case 'suggested_replies': {
        const replies = Array.isArray(payload) ? payload : [];
        set({ suggestedReplies: replies.slice(0, 3) });
        break;
      }
      case 'done': {
        // 兜底：如果没有 message_usage（旧消息无 usage），仍要清流
        if (state.isStreaming) {
          const finalBlocks = state.liveBlocks.filter((b) => b.text || b.type === 'tool');
          if (finalBlocks.length > 0) {
            const newMsg = {
              id: -Date.now(),
              session_id: state.activeSessionId,
              role: 'assistant',
              content: JSON.stringify(finalBlocks),
              usage: '',
              created_at: new Date().toISOString(),
            };
            set({ messages: [...state.messages, newMsg] });
          }
          set({ liveBlocks: [], isStreaming: false, agentState: 'DONE' });

          // 生成快捷回复建议
          const lastText = finalBlocks.filter(b => b.type === 'text').map(b => b.text).join('').slice(0, 500);
          const suggestions = generateQuickReplies(lastText);
          if (suggestions.length > 0) set({ suggestedReplies: suggestions });
        }
        // 重新拉取最新消息（保证持久化的版本与流式一致）
        if (state.activeSessionId) {
          api.listMessages(state.activeSessionId).then((m) => set({ messages: m })).catch(() => {});
        }
        break;
      }
      case 'profile_changed': {
        state.refreshProfiles();
        state.pushNotification({ title: '已切换模型', body: payload?.name || '激活档案已更新' });
        break;
      }
      case 'agent_changed': {
        state.refreshAgents();
        break;
      }
      case 'mcp_changed': {
        state.pushNotification({ title: 'MCP 配置已更新', body: '将在下次新对话生效' });
        break;
      }
      case 'notification': {
        if (payload) state.pushNotification(payload);
        break;
      }
      case 'desktop_notify': {
        const info = typeof payload === 'object' ? payload : {};
        const title = info.title || '灵犀 — 定时任务';
        const body = info.body || '任务已完成';
        state.pushNotification({ title, body });
        if (window.electronAPI?.showNotification) {
          window.electronAPI.showNotification(title, body);
        } else if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
          new Notification(title, { body });
        } else if (typeof Notification !== 'undefined' && Notification.permission !== 'denied') {
          Notification.requestPermission().then(perm => {
            if (perm === 'granted') new Notification(title, { body });
          });
        }
        break;
      }
      default: break;
    }
  },

  // ─── 发送消息 ──────────────────────────────────────────────
  sendMessage: async ({ message, images = [], useKB = false }) => {
    let sid = get().activeSessionId;
    if (!sid) {
      sid = await get().createSession();
    }
    // 立即在本地追加 user 消息（含图片预览，data: URL 直接可渲染）
    let localContent = message || (images.length ? '[图片]' : '');
    if (images.length > 0) {
      const previewImages = images.map((img) => `data:${img.mediaType};base64,${img.data}`);
      localContent = JSON.stringify({ text: message || '', images: previewImages });
    }
    const localUserMsg = {
      id: -Date.now(),
      session_id: sid,
      role: 'user',
      content: localContent,
      created_at: new Date().toISOString(),
    };
    set({
      messages: [...get().messages, localUserMsg],
      liveBlocks: [],
      isStreaming: true,
      startedAt: Date.now(),
      agentState: 'THINKING',
      suggestedReplies: [],
    });
    try {
      await api.sendChat({
        message,
        sessionId: String(sid),
        useKB,
        images,
      });
    } catch (e) {
      set({ isStreaming: false, agentState: 'IDLE' });
      get().pushNotification({ title: '发送失败', body: e.message });
    }
  },
  abort: async () => {
    const sid = get().activeSessionId;
    if (!sid) return;
    await api.abortChat(sid).catch(() => {});
    set({ isStreaming: false, agentState: 'IDLE' });
  },

  editAndResend: async (messageId, newContent) => {
    const { messages, activeSessionId, isStreaming } = get();
    if (isStreaming || !activeSessionId) return;
    const idx = messages.findIndex((m) => m.id === messageId);
    if (idx < 0) return;
    try {
      await api.updateMessage(messageId, newContent);
    } catch (e) {
      get().pushNotification({ title: '编辑失败', body: e.message });
      return;
    }
    const updated = { ...messages[idx], content: newContent };
    set({
      messages: [...messages.slice(0, idx), updated],
      liveBlocks: [],
      isStreaming: true,
      startedAt: Date.now(),
      agentState: 'THINKING',
    });
    try {
      await api.sendChat({ message: newContent, sessionId: String(activeSessionId) });
    } catch (e) {
      set({ isStreaming: false, agentState: 'IDLE' });
      get().pushNotification({ title: '重新生成失败', body: e.message });
    }
  },

  setFeedback: async (messageId, feedback) => {
    try {
      await api.setMessageFeedback(messageId, feedback);
    } catch (e) {
      get().pushNotification({ title: '反馈失败', body: e.message });
      return;
    }
    set({
      messages: get().messages.map((m) =>
        m.id === messageId ? { ...m, feedback } : m
      ),
    });
  },

  regenerate: async (messageId) => {
    const { messages, activeSessionId, isStreaming } = get();
    if (isStreaming || !activeSessionId) return;
    const idx = messages.findIndex((m) => m.id === messageId);
    if (idx < 0) return;
    let userMsg = null;
    for (let i = idx - 1; i >= 0; i--) {
      if (messages[i].role === 'user') { userMsg = messages[i]; break; }
    }
    if (!userMsg) return;
    let text = userMsg.content || '';
    let images = [];
    try {
      const obj = JSON.parse(text);
      if (obj?.text != null) {
        text = obj.text;
        images = (obj.images || []).filter(src => src.startsWith('data:')).map((src) => {
          const [header, b64] = src.split(',');
          const mt = (header.match(/data:(.*?);/) || [])[1] || 'image/png';
          return { mediaType: mt, data: b64 || '' };
        });
      }
    } catch {}
    set({
      messages: messages.slice(0, idx),
      liveBlocks: [],
      isStreaming: true,
      startedAt: Date.now(),
      agentState: 'THINKING',
    });
    try {
      await api.sendChat({ message: text, sessionId: String(activeSessionId), images });
    } catch (e) {
      set({ isStreaming: false, agentState: 'IDLE' });
      get().pushNotification({ title: '重新生成失败', body: e.message });
    }
  },

  // ── Project Nexus: A2A 状态 ─────────────────────────────────────
  nexusPeers: [],
  nexusContacts: [],
  a2aConversations: [],
  pendingConnectRequests: [],

  // A2A 会话流式状态（独立于主聊天）
  activeA2ASessionId: null,
  activeA2AConvId: null,
  a2aLiveBlocks: [],
  a2aIsStreaming: false,
  a2aRemoteLiveBlocks: [],
  a2aRemoteIsStreaming: false,
  a2aMessages: [],

  setActiveA2ASession: async (sessionId) => {
    set({
      activeA2ASessionId: sessionId,
      a2aLiveBlocks: [], a2aIsStreaming: false,
      a2aRemoteLiveBlocks: [], a2aRemoteIsStreaming: false,
      a2aMessages: [],
    });
    if (sessionId) {
      wsClient.subscribe(sessionId);
      const msgs = await api.listMessages(sessionId).catch(() => []);
      set({ a2aMessages: msgs });
    }
  },

  refreshA2AMessages: async () => {
    const sid = get().activeA2ASessionId;
    if (sid) {
      const msgs = await api.listMessages(sid).catch(() => []);
      set({ a2aMessages: msgs });
    }
  },

  refreshNexusPeers: async () => {
    try {
      const data = await api.listPeers();
      set({ nexusPeers: data || [] });
    } catch {}
  },
  refreshNexusContacts: async () => {
    try {
      const data = await api.listContacts();
      set({ nexusContacts: data || [] });
    } catch {}
  },
  refreshA2AConversations: async () => {
    try {
      const data = await api.listA2AConversations();
      set({ a2aConversations: data || [] });
    } catch {}
  },
}));

// 一次性初始化：主题与 WS
export function initStore() {
  const { theme, handleWSEvent, refreshSessions, refreshProfiles, refreshTodayUsage, setActiveSession } = useStore.getState();
  document.documentElement.setAttribute('data-theme', theme);

  wsClient.connect();
  wsClient.on(handleWSEvent);

  refreshProfiles();
  refreshTodayUsage();
  // 先加载 agents（影响 sessions 过滤）
  useStore.getState().refreshAgents().then(() => {
    refreshSessions().then((list) => {
      if (list.length > 0) setActiveSession(list[0].id);
    });
  });
}
