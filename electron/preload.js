const { contextBridge, ipcRenderer } = require('electron');

// 向渲染进程暴露安全的 API
contextBridge.exposeInMainWorld('electronAPI', {
  openExternal: (url) => ipcRenderer.invoke('open-external', url),
  getVersion: () => ipcRenderer.invoke('get-version'),

  // ─── AKSK 加密 / 解密（safeStorage，macOS 走 Keychain）──────
  encryptSecret: (plain) => ipcRenderer.invoke('encrypt-secret', plain),
  decryptSecret: (cipher) => ipcRenderer.invoke('decrypt-secret', cipher),
  isEncryptionAvailable: () => ipcRenderer.invoke('is-encryption-available'),

  // 让前端在切换激活档案后请求主进程把新档案明文同步给后端进程
  pushActiveSecret: (profileId) => ipcRenderer.invoke('push-active-secret', profileId),

  // ─── OAuth 登录 ─────────────────────────────────────────────
  startOAuth: (provider) => ipcRenderer.invoke('start-oauth', provider),

  // ─── 屏幕截图 ────────────────────────────────────────────────
  captureScreen: () => ipcRenderer.invoke('capture-screen'),
  onScreenshotCaptured: (callback) => {
    ipcRenderer.on('screenshot-captured', (_e, data) => callback(data));
    return () => ipcRenderer.removeAllListeners('screenshot-captured');
  },

  // ─── 桌面通知 ──────────────────────────────────────────────
  showNotification: (title, body) => ipcRenderer.invoke('show-notification', title, body),

});
