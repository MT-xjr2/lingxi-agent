const { contextBridge, ipcRenderer } = require('electron');

// 向渲染进程暴露安全的 API
contextBridge.exposeInMainWorld('electronAPI', {
  openExternal: (url) => ipcRenderer.invoke('open-external', url),
  getVersion: () => ipcRenderer.invoke('get-version'),

  // ─── AKSK 加密 / 解密（safeStorage，macOS 走 Keychain）──────
  // 前端在保存档案时调用 encryptSecret(plain) → cipher(base64)，再 POST 给后端。
  // decryptSecret 仅供调试或测试连接时使用；生产链路一般通过 pushActiveSecret 让 Electron 直传后端进程。
  encryptSecret: (plain) => ipcRenderer.invoke('encrypt-secret', plain),
  decryptSecret: (cipher) => ipcRenderer.invoke('decrypt-secret', cipher),
  isEncryptionAvailable: () => ipcRenderer.invoke('is-encryption-available'),

  // 让前端在切换激活档案后请求主进程把新档案明文同步给后端进程
  pushActiveSecret: (profileId) => ipcRenderer.invoke('push-active-secret', profileId),
});
