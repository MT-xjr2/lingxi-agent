// 与后端 REST + WebSocket 通信的轻量封装

const baseHeaders = { 'Content-Type': 'application/json' };

async function req(method, path, body) {
  const opts = { method, headers: baseHeaders, credentials: 'include' };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const res = await fetch(path, opts);
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`HTTP ${res.status}: ${text}`);
  }
  const ct = res.headers.get('content-type') || '';
  if (ct.includes('application/json')) return res.json();
  return res.text();
}

export const api = {
  // auth
  getAuthStatus: () => req('GET', '/api/auth/status'),
  getCurrentUser: () => req('GET', '/api/auth/me'),
  guestLogin: (nickname) => req('POST', '/api/auth/guest', { nickname }),
  oauthCallback: (data) => req('POST', '/api/auth/oauth/callback', data),
  logout: () => req('POST', '/api/auth/logout'),
  listOAuthConfigs: () => req('GET', '/api/auth/oauth-configs'),
  saveOAuthConfig: (data) => req('POST', '/api/auth/oauth-configs', data),

  // sessions
  listSessions: (agentId) =>
    req('GET', `/api/sessions${agentId != null ? `?agent_id=${agentId}` : ''}`),
  createSession: (titleOrPayload) =>
    req('POST', '/api/sessions',
      typeof titleOrPayload === 'string'
        ? { title: titleOrPayload }
        : (titleOrPayload || {})
    ),
  renameSession: (id, title) => req('PATCH', `/api/sessions/${id}`, { title }),
  pinSession: (id, pinned) => req('PATCH', `/api/sessions/${id}`, { pinned }),
  deleteSession: (id) => req('DELETE', `/api/sessions/${id}`),
  batchDeleteSessions: (ids) => req('POST', '/api/sessions/batch-delete', { ids }),
  listMessages: (id) => req('GET', `/api/sessions/${id}/messages`),
  setSessionAgent: (id, agent_id) => req('POST', `/api/sessions/${id}/agent`, { agent_id }),

  // messages
  updateMessage: (id, content) => req('PUT', `/api/messages/${id}`, { content }),
  setMessageFeedback: (id, feedback) => req('POST', `/api/messages/${id}/feedback`, { feedback }),

  // chat
  sendChat: (payload) => req('POST', '/api/chat', payload),
  abortChat: (sessionId) => req('POST', '/api/chat/abort', { sessionId: String(sessionId) }),

  // providers + profiles
  listProviders: () => req('GET', '/api/providers'),
  listProfiles: (includeCipher) => req('GET', `/api/api-profiles${includeCipher ? '?include_cipher=1' : ''}`),
  saveProfile: (p) => req('POST', '/api/api-profiles', p),
  deleteProfile: (id) => req('DELETE', `/api/api-profiles/${id}`),
  activateProfile: (id) => req('POST', `/api/api-profiles/${id}/activate`),
  testProfile: (id, body) => req('POST', `/api/api-profiles/${id}/test`, body || {}),

  // skills / knowledge
  listSkills: () => req('GET', '/api/skills'),
  getSkillContent: (id) => req('GET', `/api/skills/${id}/content`),
  updateSkillContent: (id, files) => req('PUT', `/api/skills/${id}/content`, { files }),
  exportSkillUrl: (id) => `/api/skills/${id}/export`,
  searchMarketplace: (params = {}) => {
    const qs = new URLSearchParams(params).toString();
    return req('GET', `/api/skills/marketplace?${qs}`);
  },
  getMarketplaceSkill: (ns, slug) => req('GET', `/api/skills/marketplace/${ns}/${slug}`),
  installMarketplaceSkill: (data) => req('POST', '/api/skills/marketplace/install', data),
  getMarketplaceCategories: () => req('GET', '/api/skills/marketplace/categories'),
  listKnowledge: () => req('GET', '/api/knowledge'),

  // MCP
  listMCP: () => req('GET', '/api/mcp'),
  saveMCP: (m) => req('POST', '/api/mcp', m),
  deleteMCP: (id) => req('DELETE', `/api/mcp/${id}`),
  toggleMCP: (id, enabled) => req('POST', `/api/mcp/${id}/toggle`, { enabled }),

  // Agents（智能体工厂）
  listAgents: () => req('GET', '/api/agents'),
  getAgent: (id) => req('GET', `/api/agents/${id}`),
  saveAgent: (a) => req('POST', '/api/agents', a),
  deleteAgent: (id) => req('DELETE', `/api/agents/${id}`),

  // usage
  getUsage: (range = '7d') => req('GET', `/api/usage?range=${range}`),
  getQuota: (profileId) => req('GET', `/api/usage/quota?profile_id=${profileId}`),

  // router (bridge) status
  getRouterStatus: () => req('GET', '/api/router/status'),
  stopRouter: () => req('POST', '/api/router/stop'),

  // memories
  listMemories: (agentId = 0) => req('GET', `/api/memories?agent_id=${agentId}`),
  createMemory: (data) => req('POST', '/api/memories', data),
  deleteMemory: (id) => req('DELETE', `/api/memories/${id}`),
  clearMemories: (agentId = 0) => req('DELETE', `/api/memories/clear?agent_id=${agentId}`),

  // message pin
  toggleMessagePin: (id, pinned) => req('POST', `/api/messages/${id}/pin`, { pinned }),

  // 语音识别
  transcribeAudio: async (blob) => {
    const form = new FormData();
    form.append('file', blob, 'audio.webm');
    const res = await fetch('/api/transcribe', { method: 'POST', body: form, credentials: 'include' });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      let msg = `HTTP ${res.status}`;
      try { msg = JSON.parse(text).error || msg; } catch {}
      throw new Error(msg);
    }
    const data = await res.json();
    return data.text || '';
  },

  // scheduled tasks
  listScheduledTasks: () => req('GET', '/api/scheduled-tasks'),
  createScheduledTask: (data) => req('POST', '/api/scheduled-tasks', data),
  updateScheduledTask: (id, data) => req('PUT', `/api/scheduled-tasks/${id}`, data),
  deleteScheduledTask: (id) => req('DELETE', `/api/scheduled-tasks/${id}`),
  toggleScheduledTask: (id, enabled) => req('POST', `/api/scheduled-tasks/${id}/toggle`, { enabled }),
  triggerScheduledTask: (id) => req('POST', `/api/scheduled-tasks/${id}/run`),
  listScheduledTaskRuns: (id) => req('GET', `/api/scheduled-tasks/${id}/runs`),

  // ── Project Nexus: Agent-to-Agent ─────────────────────────────
  getNexusSettings: () => req('GET', '/api/nexus/settings'),
  updateNexusSettings: (data) => req('PUT', '/api/nexus/settings', data),
  listPeers: () => req('GET', '/api/peers'),
  listA2AConversations: () => req('GET', '/api/a2a-conversations'),
  getA2AConversation: (id) => req('GET', `/api/a2a-conversations/${id}`),
  createA2AConversation: (data) => req('POST', '/api/a2a-conversations', data),
  pauseA2AConversation: (id) => req('POST', `/api/a2a-conversations/${id}/pause`),
  takeoverA2AConversation: (id, content) => req('POST', `/api/a2a-conversations/${id}/takeover`, { content }),
  terminateA2AConversation: (id) => req('POST', `/api/a2a-conversations/${id}/terminate`),
  approveA2AConversation: (id, approved) => req('POST', `/api/a2a-conversations/${id}/approve`, { approved }),
  acceptRemoteConversation: (id, localAgentId) => req('POST', `/api/a2a-conversations/${id}/accept-remote`, { local_agent_id: localAgentId }),
  rejectRemoteConversation: (id) => req('POST', `/api/a2a-conversations/${id}/reject-remote`),
  deleteA2AConversation: (id) => req('DELETE', `/api/a2a-conversations/${id}`),
  getAgentNexusConfig: (id) => req('GET', `/api/agents/${id}/nexus-config`),
  updateAgentNexusConfig: (id, data) => req('PUT', `/api/agents/${id}/nexus-config`, data),

  // ── WAN (广域网) ──────────────────────────────────────────────
  listWANPeers: () => req('GET', '/api/wan/peers'),
  getWANStatus: () => req('GET', '/api/wan/status'),
};

// ─── WebSocket ────────────────────────────────────────────────────
export class WSClient {
  constructor() {
    this.ws = null;
    this.handlers = new Set();
    this.subscribed = new Set();
    this._closed = false;
    this._reconnectTimer = null;
  }
  on(handler) {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }
  connect(initialSessionId) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/api/ws${initialSessionId ? `?sessionId=${initialSessionId}` : ''}`;
    this.ws = new WebSocket(url);
    this.ws.onopen = () => {
      // 重新订阅
      for (const sid of this.subscribed) this._send({ type: 'subscribe', sessionId: sid });
    };
    this.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        for (const h of this.handlers) h(msg);
      } catch {}
    };
    this.ws.onclose = () => {
      if (this._closed) return;
      clearTimeout(this._reconnectTimer);
      this._reconnectTimer = setTimeout(() => this.connect(), 1500);
    };
    this.ws.onerror = () => { try { this.ws.close(); } catch {} };
  }
  subscribe(sessionId) {
    this.subscribed.add(sessionId);
    this._send({ type: 'subscribe', sessionId });
  }
  unsubscribe(sessionId) {
    this.subscribed.delete(sessionId);
    this._send({ type: 'unsubscribe', sessionId });
  }
  _send(obj) {
    if (this.ws && this.ws.readyState === 1) this.ws.send(JSON.stringify(obj));
  }
  close() { this._closed = true; try { this.ws?.close(); } catch {} }
}

export const wsClient = new WSClient();

// ─── Electron 桥接（可选）─────────────────────────────────────────
export const electron = {
  available: typeof window !== 'undefined' && !!window.electronAPI,
  encryptSecret: async (plain) => {
    if (window.electronAPI?.encryptSecret) return window.electronAPI.encryptSecret(plain);
    return 'b64:' + btoa(unescape(encodeURIComponent(plain)));
  },
  decryptSecret: async (cipher) => {
    if (window.electronAPI?.decryptSecret) return window.electronAPI.decryptSecret(cipher);
    if (cipher && cipher.startsWith('b64:')) {
      try { return decodeURIComponent(escape(atob(cipher.slice(4)))); } catch { return ''; }
    }
    return '';
  },
  pushActiveSecret: async (profileId) => {
    if (window.electronAPI?.pushActiveSecret) return window.electronAPI.pushActiveSecret(profileId);
    return { ok: false };
  },
  openExternal: (url) => {
    if (window.electronAPI?.openExternal) return window.electronAPI.openExternal(url);
    window.open(url, '_blank');
  },
  startOAuth: async (provider) => {
    if (window.electronAPI?.startOAuth) return window.electronAPI.startOAuth(provider);
    throw new Error('OAuth 登录仅在桌面应用中可用');
  },
};
