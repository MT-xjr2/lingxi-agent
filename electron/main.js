const { app, BrowserWindow, ipcMain, shell, safeStorage, Menu, desktopCapturer, globalShortcut, nativeImage, dialog } = require('electron');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');
const http = require('http');
const { autoUpdater } = require('electron-updater');

// ─── 配置 ────────────────────────────────────────────────────────
const BACKEND_PORT = 3001;
const BACKEND_STARTUP_TIMEOUT = 20000;

let mainWindow = null;
let backendProcess = null;

// ─── 路径工具 ────────────────────────────────────────────────────

function getFrontendDistPath() {
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'frontend-dist');
  }
  return path.join(__dirname, '..', 'frontend-desktop', 'dist');
}

// Go 二进制路径（打包后在 resources/smart-agent[.exe]，开发时在 backend-desktop/smart-agent[.exe]）
function getGoBinPath() {
  const ext = process.platform === 'win32' ? '.exe' : '';
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'smart-agent' + ext);
  }
  return path.join(__dirname, '..', 'backend-desktop', 'smart-agent' + ext);
}

// 内置 AI 引擎路径
// macOS 打包后：resources/ai-engine/lingxi（bash 包装脚本）
// Windows 打包后：resources/ai-engine/lingxi.cmd（cmd 包装脚本）
// 开发时：对应脚本或系统 claude
function getClaudeBin() {
  const resourcesDir = app.isPackaged
    ? process.resourcesPath
    : path.join(__dirname, 'resources');
  const isWin = process.platform === 'win32';
  const bundled = path.join(resourcesDir, 'ai-engine', isWin ? 'lingxi.cmd' : 'lingxi');
  if (fs.existsSync(bundled)) {
    if (!isWin) {
      try { fs.chmodSync(bundled, 0o755); } catch (e) {}
    }
    return bundled;
  }
  // 回退到系统 claude
  if (isWin) {
    try {
      return require('child_process').execSync('where claude', { encoding: 'utf-8' }).trim().split('\n')[0];
    } catch {
      return 'claude';
    }
  }
  try {
    return require('child_process').execSync('which claude').toString().trim();
  } catch {
    return 'claude';
  }
}

// ─── Bridge 二进制 + 隔离目录 ────────────────────────────────────
// 用于非 Anthropic 协议的供应商：bridge 在本地 127.0.0.1:<port> 启一个
// Anthropic 协议端点，把 Claude Code 的请求转发到用户配置的 OpenAI 兼容供应商，
// 并把 OpenAI 流式响应实时翻译回 Anthropic SSE 返回给 Claude Code。
//
// 优先使用 litellm-bridge（Python，工具调用协议兼容性更稳定），
// 不存在时回退到 Node llm-bridge。
function getBridgeBin() {
  const resourcesDir = app.isPackaged
    ? process.resourcesPath
    : path.join(__dirname, 'resources');
  const isWin = process.platform === 'win32';

  // 优先：LiteLLM Bridge（Python）
  const litellmName = isWin ? 'bridge.cmd' : 'bridge';
  const litellmBridge = path.join(resourcesDir, 'litellm-bridge', litellmName);
  if (fs.existsSync(litellmBridge)) {
    if (!isWin) { try { fs.chmodSync(litellmBridge, 0o755); } catch (e) {} }
    console.log('[electron] bridge: using litellm-bridge (Python)');
    return litellmBridge;
  }

  // 回退：Node llm-bridge
  const bridgeName = isWin ? 'bridge.cmd' : 'bridge';
  const nodeBridge = path.join(resourcesDir, 'bridge', bridgeName);
  if (fs.existsSync(nodeBridge)) {
    if (!isWin) { try { fs.chmodSync(nodeBridge, 0o755); } catch (e) {} }
    console.log('[electron] bridge: using node llm-bridge (fallback)');
    return nodeBridge;
  }

  return '';
}

function getBridgeHome() {
  return path.join(app.getPath('userData'), 'bridge-home');
}

// ─── 获取应用隔离 HOME 目录 ──────────────────────────────────────
// AI 引擎通过 HOME 环境变量定位配置目录
// 给它一个独立目录，完全不碰用户真实的 ~/.claude/
function getAppHome() {
  return path.join(app.getPath('userData'), 'ai-home');
}

// 知识库目录：userData/knowledge
function getKbPath() {
  return path.join(app.getPath('userData'), 'knowledge');
}

// 用户上传图片持久化目录：userData/uploads
function getUploadsPath() {
  return path.join(app.getPath('userData'), 'uploads');
}

// skills 目录：隔离 HOME/.claude/skills（与 initClaudeConfig 同步的位置一致）
function getSkillsPath() {
  return path.join(getAppHome(), '.claude', 'skills');
}

// whisper.cpp 离线语音识别二进制 + 模型
function getWhisperBin() {
  const resourcesDir = app.isPackaged
    ? process.resourcesPath
    : path.join(__dirname, 'resources');
  const bin = path.join(resourcesDir, 'whisper', 'whisper-cli');
  if (fs.existsSync(bin)) {
    try { fs.chmodSync(bin, 0o755); } catch (e) {}
    return bin;
  }
  return '';
}

function getWhisperModel() {
  const resourcesDir = app.isPackaged
    ? process.resourcesPath
    : path.join(__dirname, 'resources');
  const model = path.join(resourcesDir, 'whisper', 'ggml-base.bin');
  if (fs.existsSync(model)) return model;
  return '';
}

// ─── 初始化 claude-code 隔离配置 ────────────────────────────────
function initClaudeConfig() {
  const appHome = getAppHome();
  const claudeDir = path.join(appHome, '.claude');
  const claudeJson = path.join(appHome, '.claude.json');

  const configSrc = app.isPackaged
    ? path.join(process.resourcesPath, 'ai-config')
    : path.join(__dirname, '..', 'ai-config');

  fs.mkdirSync(claudeDir, { recursive: true });

  // 写入 settings.json（每次启动都用内嵌版本）
  const settingsSrc = path.join(configSrc, 'settings.json');
  if (fs.existsSync(settingsSrc)) {
    let settings;
    try {
      settings = JSON.parse(fs.readFileSync(settingsSrc, 'utf8'));
    } catch (e) {
      settings = {};
    }
    delete settings.mcpServers;
    delete settings.env; // 密钥不写入 AI 可读的文件，仅通过进程环境变量注入
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), JSON.stringify(settings, null, 2), 'utf8');
    console.log('[electron] wrote isolated engine settings.json');
  }

  // 注册 Playwright MCP（如果 Chrome 存在）
  const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  if (fs.existsSync(chromePath)) {
    // Playwright MCP 使用内置 node-bin 驱动
    // 路径：resources/node-bin/node 和 resources/node-bin/node_modules/@playwright/mcp/cli.js
    const resourcesDir = app.isPackaged
      ? process.resourcesPath
      : path.join(__dirname, 'resources');

    const nodeBin = path.join(resourcesDir, 'node-bin', 'node');
    const mcpCliPath = path.join(resourcesDir, 'node-bin', 'node_modules', '@playwright', 'mcp', 'cli.js');

    if (fs.existsSync(mcpCliPath) && fs.existsSync(nodeBin)) {
      let claudeJsonObj = {};
      try { claudeJsonObj = JSON.parse(fs.readFileSync(claudeJson, 'utf8')); } catch (e) {}
      if (!claudeJsonObj.mcpServers) claudeJsonObj.mcpServers = {};

      const screenshotDir = path.join(app.getPath('userData'), 'playwright-screenshots');
      fs.mkdirSync(screenshotDir, { recursive: true });

      claudeJsonObj.mcpServers.playwright = {
        command: nodeBin,
        args: [
          mcpCliPath,
          '--browser', 'chrome',
          '--executable-path', chromePath,
          '--headless',
          '--no-sandbox',
          '--viewport-size', '1280x900',
          '--timeout-action', '10000',
          '--timeout-navigation', '30000',
          '--output-dir', screenshotDir,
        ],
      };
      fs.writeFileSync(claudeJson, JSON.stringify(claudeJsonObj, null, 2), 'utf8');
      console.log('[electron] registered playwright MCP, cli:', mcpCliPath);
    } else {
      console.log('[electron] playwright MCP cli not found:', mcpCliPath);
    }
  } else {
    console.log('[electron] Chrome not found, playwright MCP skipped');
  }

  // 写入 .claude.json（跳过 onboarding，仅首次）
  const claudeJsonSrc = path.join(configSrc, 'claude.json');
  if (!fs.existsSync(claudeJson) && fs.existsSync(claudeJsonSrc)) {
    fs.copyFileSync(claudeJsonSrc, claudeJson);
    console.log('[electron] wrote isolated engine config');
  }

  // 同步内置 skills
  const skillsSrc = path.join(configSrc, 'skills');
  const skillsDst = path.join(claudeDir, 'skills');
  if (fs.existsSync(skillsSrc)) {
    fs.mkdirSync(skillsDst, { recursive: true });
    for (const skillName of fs.readdirSync(skillsSrc)) {
      const srcSkill = path.join(skillsSrc, skillName);
      if (!fs.statSync(srcSkill).isDirectory()) continue;
      const dstSkill = path.join(skillsDst, skillName);
      if (!fs.existsSync(dstSkill)) {
        fs.mkdirSync(dstSkill, { recursive: true });
        for (const f of fs.readdirSync(srcSkill)) {
          fs.copyFileSync(path.join(srcSkill, f), path.join(dstSkill, f));
        }
        console.log('[electron] installed built-in skill:', skillName);
      }
    }
  }

  // 同步系统提示（每次启动都覆盖）
  const claudeMdSrc = path.join(configSrc, 'CLAUDE.md');
  if (fs.existsSync(claudeMdSrc)) {
    fs.copyFileSync(claudeMdSrc, path.join(claudeDir, 'CLAUDE.md'));
    console.log('[electron] wrote engine system prompt');
  }

  // 同步内置 subagents
  const agentsSrc = path.join(configSrc, '.claude', 'agents');
  const agentsDst = path.join(claudeDir, 'agents');
  if (fs.existsSync(agentsSrc)) {
    fs.mkdirSync(agentsDst, { recursive: true });
    for (const agentFile of fs.readdirSync(agentsSrc)) {
      if (!agentFile.endsWith('.md')) continue;
      fs.copyFileSync(path.join(agentsSrc, agentFile), path.join(agentsDst, agentFile));
      console.log('[electron] installed built-in agent:', agentFile);
    }
  }

  console.log('[electron] engine isolated HOME:', appHome);
}

// ─── 从 auth.json 读取认证环境变量 ──────────────────────────────
// auth.json 仅供 Electron 读取后注入为进程环境变量，不会被写入 AI 隔离 HOME
function getClaudeAuthEnv() {
  const configSrc = app.isPackaged
    ? path.join(process.resourcesPath, 'ai-config')
    : path.join(__dirname, '..', 'ai-config');

  const authPath = path.join(configSrc, 'auth.json');
  try {
    const auth = JSON.parse(fs.readFileSync(authPath, 'utf8'));
    return {
      ANTHROPIC_AUTH_TOKEN: auth.ANTHROPIC_AUTH_TOKEN || '',
      ANTHROPIC_BASE_URL: auth.ANTHROPIC_BASE_URL || '',
      ANTHROPIC_MODEL: auth.ANTHROPIC_MODEL || '',
    };
  } catch (e) {
    console.error('[electron] failed to read auth.json:', e.message);
    return {};
  }
}

// ─── 启动 Go 后端子进程 ──────────────────────────────────────────
function startBackend() {
  const goBin = getGoBinPath();
  const frontendDist = getFrontendDistPath();
  const appHome = getAppHome();
  const dbPath = path.join(app.getPath('userData'), 'smart-agent.db');
  const claudeBin = getClaudeBin();
  const kbPath = getKbPath();
  const skillsPath = getSkillsPath();
  const uploadsPath = getUploadsPath();

  // 确保知识库目录 + 上传目录存在
  fs.mkdirSync(kbPath, { recursive: true });
  fs.mkdirSync(uploadsPath, { recursive: true });

  console.log('[electron] starting Go backend:', goBin);
  console.log('[electron] engine bin:', claudeBin);
  console.log('[electron] frontend dist:', frontendDist);
  console.log('[electron] db path:', dbPath);
  console.log('[electron] engine HOME (isolated):', appHome);
  console.log('[electron] knowledge base path:', kbPath);
  console.log('[electron] skills path:', skillsPath);

  if (!fs.existsSync(goBin)) {
    console.error('[electron] Go binary not found:', goBin);
    console.error('[electron] Please run: cd backend-desktop && go build -o smart-agent .');
    return;
  }

  // 确保 Go 二进制有执行权限
  try { fs.chmodSync(goBin, 0o755); } catch (e) {}

  const authEnv = getClaudeAuthEnv();
  console.log('[electron] engine auth token present:', !!authEnv.ANTHROPIC_AUTH_TOKEN);
  console.log('[electron] engine base url:', authEnv.ANTHROPIC_BASE_URL || '(default)');

  // 补全 PATH：Electron 从 macOS 启动时不加载 .zshrc，需手动补充常用工具路径
  const userHome = require('os').homedir();
  const extraPaths = [
    '/usr/local/bin',
    '/opt/homebrew/bin',
    '/opt/homebrew/sbin',
    `${userHome}/.brew/Homebrew/bin`,
    `${userHome}/.nvm/versions/node/v22.22.1/bin`,
    `${userHome}/bin`,
    '/usr/bin',
    '/bin',
    '/usr/sbin',
    '/sbin',
  ];
  const currentPath = process.env.PATH || '';
  const mergedPath = [...new Set([...extraPaths, ...currentPath.split(':')])].join(':');

  backendProcess = spawn(goBin, [], {
    env: {
      ...process.env,
      PATH: mergedPath,
      PORT: String(BACKEND_PORT),
      FRONTEND_DIST: frontendDist,
      DB_PATH: dbPath,
      // 覆盖 HOME，让 AI 引擎使用隔离目录，不碰用户真实的配置
      HOME: appHome,
      // 内置 AI 引擎路径
      CLAUDE_BIN: claudeBin,
      // Bridge (OpenAI ↔ Anthropic 路由层) 二进制 + 隔离数据目录
      BRIDGE_BIN: getBridgeBin(),
      BRIDGE_HOME: getBridgeHome(),
      // 显式传入知识库和技能路径，避免 Go 自己拼路径时受 HOME 空格影响
      KB_PATH: kbPath,
      SKILLS_PATH: skillsPath,
      UPLOADS_PATH: uploadsPath,
      // whisper.cpp 离线语音识别
      WHISPER_BIN: getWhisperBin(),
      WHISPER_MODEL: getWhisperModel(),
      // 认证信息（从 settings.json 读取，注入给 Go 后端，再透传给 AI 引擎）
      ...authEnv,
    },
    cwd: app.getPath('userData'),
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  backendProcess.stdout.on('data', (data) => {
    console.log('[backend]', data.toString().trim());
  });

  backendProcess.stderr.on('data', (data) => {
    console.error('[backend:err]', data.toString().trim());
  });

  backendProcess.on('exit', (code, signal) => {
    console.log(`[electron] backend exited: code=${code} signal=${signal}`);
    backendProcess = null;
  });

  backendProcess.on('error', (err) => {
    console.error('[electron] backend spawn error:', err);
  });
}

// ─── 等待后端 HTTP 服务就绪 ──────────────────────────────────────
function waitForBackend(timeout = BACKEND_STARTUP_TIMEOUT) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      const req = http.get(`http://localhost:${BACKEND_PORT}/api/ping`, (res) => {
        res.resume();
        resolve();
      });
      req.on('error', () => {
        if (Date.now() - start > timeout) {
          reject(new Error('backend startup timeout'));
          return;
        }
        setTimeout(check, 300);
      });
      req.setTimeout(1000, () => {
        req.destroy();
        setTimeout(check, 300);
      });
    };
    check();
  });
}

// ─── 应用菜单（确保 Cmd+C/V/X/A 等快捷键在 webview 内可用）─────
function buildAppMenu() {
  const isMac = process.platform === 'darwin';
  const template = [
    ...(isMac ? [{
      label: app.name,
      submenu: [
        { role: 'about' },
        { type: 'separator' },
        { role: 'services' },
        { type: 'separator' },
        { role: 'hide' },
        { role: 'hideOthers' },
        { role: 'unhide' },
        { type: 'separator' },
        { role: 'quit' },
      ],
    }] : []),
    {
      label: 'Edit',
      submenu: [
        { role: 'undo' },
        { role: 'redo' },
        { type: 'separator' },
        { role: 'cut' },
        { role: 'copy' },
        { role: 'paste' },
        ...(isMac
          ? [
              { role: 'pasteAndMatchStyle' },
              { role: 'delete' },
              { role: 'selectAll' },
            ]
          : [{ role: 'delete' }, { type: 'separator' }, { role: 'selectAll' }]),
      ],
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' },
      ],
    },
    {
      role: 'window',
      submenu: [
        { role: 'minimize' },
        { role: 'zoom' },
        ...(isMac ? [{ type: 'separator' }, { role: 'front' }] : [{ role: 'close' }]),
      ],
    },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}


function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    titleBarStyle: 'hiddenInset',
    trafficLightPosition: { x: 16, y: 16 },
    backgroundColor: '#0f0f11',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
    icon: path.join(__dirname, 'assets', 'icon.png'),
    show: false,
  });

  // 立即加载 splash 页面，后端就绪后再切换到主应用
  mainWindow.loadFile(path.join(__dirname, 'splash.html'));

  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function switchToApp() {
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.loadURL(`http://localhost:${BACKEND_PORT}`);
  }
}

// ─── safeStorage 工具：AKSK 加解密 ───────────────────────────────
function encryptSecretBase64(plain) {
  if (!plain) return '';
  if (!safeStorage.isEncryptionAvailable()) {
    // 无加密能力时退化为简易混淆（不建议，但保证可用）
    return 'b64:' + Buffer.from(String(plain), 'utf8').toString('base64');
  }
  return 'sf:' + safeStorage.encryptString(String(plain)).toString('base64');
}

function decryptSecretBase64(cipher) {
  if (!cipher) return '';
  if (cipher.startsWith('sf:')) {
    if (!safeStorage.isEncryptionAvailable()) {
      console.warn('[electron] safeStorage 不可用，无法解密 sf: 密文');
      return '';
    }
    try {
      return safeStorage.decryptString(Buffer.from(cipher.slice(3), 'base64'));
    } catch (e) {
      console.error('[electron] decryptString error:', e.message);
      return '';
    }
  }
  if (cipher.startsWith('b64:')) {
    try { return Buffer.from(cipher.slice(4), 'base64').toString('utf8'); } catch { return ''; }
  }
  return '';
}

// ─── 与后端通信：查激活档案 / 推送明文 token ─────────────────────
function backendRequest(method, path, body) {
  return new Promise((resolve, reject) => {
    const data = body ? Buffer.from(JSON.stringify(body)) : null;
    const req = http.request({
      host: 'localhost', port: BACKEND_PORT, method, path,
      headers: data
        ? { 'Content-Type': 'application/json', 'Content-Length': data.length }
        : {},
    }, (res) => {
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => {
        const txt = Buffer.concat(chunks).toString('utf8');
        if (res.statusCode >= 400) {
          reject(new Error(`HTTP ${res.statusCode}: ${txt}`));
          return;
        }
        try { resolve(JSON.parse(txt)); } catch { resolve(txt); }
      });
    });
    req.on('error', reject);
    if (data) req.write(data);
    req.end();
  });
}

// pushActiveSecretToBackend 拉取激活档案的密文 → 解密 → 一次性下发到后端进程内存
async function pushActiveSecretToBackend(profileIdHint) {
  try {
    const profiles = await backendRequest('GET', '/api/api-profiles?include_cipher=1', null);
    let active = profiles.find((p) => p.is_active);
    if (profileIdHint && !active) active = profiles.find((p) => p.id === profileIdHint);
    if (!active) {
      console.log('[electron] no active profile yet');
      return;
    }
    const token = decryptSecretBase64(active.auth_token_cipher);
    if (!token) {
      console.warn('[electron] active profile has empty/undecryptable token, skip push');
      return;
    }
    await backendRequest('POST', '/api/runtime/active-secret', {
      id: active.id,
      name: active.name,
      model: active.model,
      base_url: active.base_url,
      token,
      protocol: active.provider_protocol || 'anthropic',
      transformer: active.transformer || '',
    });
    console.log('[electron] pushed active secret: id=', active.id, 'proto=', active.provider_protocol, 'model=', active.model);
  } catch (e) {
    console.error('[electron] pushActiveSecretToBackend error:', e.message);
  }
}

// ─── IPC 处理 ────────────────────────────────────────────────────
ipcMain.handle('open-external', async (_event, url) => {
  await shell.openExternal(url);
});

ipcMain.handle('get-version', () => {
  return app.getVersion();
});

ipcMain.handle('check-for-update', async () => {
  if (!app.isPackaged) return { status: 'dev' };
  try {
    const result = await autoUpdater.checkForUpdates();
    return { status: 'ok', version: result?.updateInfo?.version };
  } catch (err) {
    return { status: 'error', error: err.message };
  }
});

ipcMain.handle('encrypt-secret', (_e, plain) => encryptSecretBase64(plain));
ipcMain.handle('decrypt-secret', (_e, cipher) => decryptSecretBase64(cipher));
ipcMain.handle('is-encryption-available', () => {
  try { return safeStorage.isEncryptionAvailable(); } catch { return false; }
});
ipcMain.handle('push-active-secret', async (_e, profileId) => {
  await pushActiveSecretToBackend(profileId);
  return { ok: true };
});

// ─── 屏幕截图 (desktopCapturer) ──────────────────────────────────
ipcMain.handle('capture-screen', async () => {
  const sources = await desktopCapturer.getSources({
    types: ['screen'],
    thumbnailSize: { width: 1920, height: 1080 },
  });
  if (!sources.length) throw new Error('无法获取屏幕源');
  const img = sources[0].thumbnail;
  const png = img.toPNG();
  return { data: png.toString('base64'), mediaType: 'image/png' };
});

// ─── 桌面通知 ─────────────────────────────────────────────────────
const { Notification: ElectronNotification } = require('electron');
ipcMain.handle('show-notification', (_e, title, body) => {
  if (ElectronNotification.isSupported()) {
    const n = new ElectronNotification({ title: title || '灵犀', body: body || '' });
    n.show();
  }
});

// ─── 应用生命周期 ────────────────────────────────────────────────
// ─── 自动更新 ────────────────────────────────────────────────────
function setupAutoUpdater() {
  if (!app.isPackaged) {
    console.log('[updater] skipping auto-update in dev mode');
    return;
  }

  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = true;
  autoUpdater.logger = {
    info: (m) => console.log('[updater]', m),
    warn: (m) => console.warn('[updater]', m),
    error: (m) => console.error('[updater]', m),
  };

  autoUpdater.on('checking-for-update', () => {
    console.log('[updater] checking for update...');
    if (mainWindow) {
      mainWindow.webContents.send('update-status', { status: 'checking' });
    }
  });

  autoUpdater.on('update-available', (info) => {
    console.log('[updater] update available:', info.version);
    if (mainWindow) {
      mainWindow.webContents.send('update-status', {
        status: 'available',
        version: info.version,
        releaseNotes: info.releaseNotes,
        releaseDate: info.releaseDate,
      });
    }
    dialog.showMessageBox(mainWindow, {
      type: 'info',
      title: '发现新版本',
      message: `灵犀 ${info.version} 可用`,
      detail: '是否立即下载更新？',
      buttons: ['立即下载', '稍后再说'],
      defaultId: 0,
    }).then(({ response }) => {
      if (response === 0) {
        autoUpdater.downloadUpdate();
      }
    });
  });

  autoUpdater.on('update-not-available', () => {
    console.log('[updater] no update available');
    if (mainWindow) {
      mainWindow.webContents.send('update-status', { status: 'up-to-date' });
    }
  });

  autoUpdater.on('download-progress', (progress) => {
    console.log(`[updater] download: ${Math.round(progress.percent)}%`);
    if (mainWindow) {
      mainWindow.webContents.send('update-status', {
        status: 'downloading',
        percent: Math.round(progress.percent),
        bytesPerSecond: progress.bytesPerSecond,
        total: progress.total,
        transferred: progress.transferred,
      });
    }
  });

  autoUpdater.on('update-downloaded', (info) => {
    console.log('[updater] update downloaded:', info.version);
    if (mainWindow) {
      mainWindow.webContents.send('update-status', {
        status: 'downloaded',
        version: info.version,
      });
    }
    dialog.showMessageBox(mainWindow, {
      type: 'info',
      title: '更新已就绪',
      message: `灵犀 ${info.version} 已下载完成`,
      detail: '重启应用以完成更新。',
      buttons: ['立即重启', '稍后重启'],
      defaultId: 0,
    }).then(({ response }) => {
      if (response === 0) {
        autoUpdater.quitAndInstall(false, true);
      }
    });
  });

  autoUpdater.on('error', (err) => {
    console.error('[updater] error:', err.message);
    if (mainWindow) {
      mainWindow.webContents.send('update-status', {
        status: 'error',
        error: err.message,
      });
    }
  });

  // 启动后延迟 10 秒检查更新，避免影响启动速度
  setTimeout(() => {
    autoUpdater.checkForUpdates().catch((err) => {
      console.error('[updater] check failed:', err.message);
    });
  }, 10000);

  // 每 4 小时检查一次更新
  setInterval(() => {
    autoUpdater.checkForUpdates().catch(() => {});
  }, 4 * 60 * 60 * 1000);
}

app.whenReady().then(async () => {
  buildAppMenu();

  // 立即创建窗口并显示 splash 页，让用户第一时间看到内容
  createWindow();

  // 并行执行初始化（不阻塞窗口显示）
  try {
    initClaudeConfig();
  } catch (err) {
    console.error('[electron] initClaudeConfig error:', err);
  }

  startBackend();

  try {
    await waitForBackend();
    console.log('[electron] backend is ready');
    await pushActiveSecretToBackend();
    // 后端就绪后切换到主应用
    switchToApp();
    // 启动自动更新检查
    setupAutoUpdater();
  } catch (err) {
    console.error('[electron] backend failed to start:', err);
  }

  // 注册全局截屏快捷键 Cmd+Shift+S → 截屏并推送到前端
  try {
    globalShortcut.register('CommandOrControl+Shift+S', async () => {
      if (!mainWindow) return;
      try {
        const sources = await desktopCapturer.getSources({
          types: ['screen'],
          thumbnailSize: { width: 1920, height: 1080 },
        });
        if (!sources.length) return;
        const png = sources[0].thumbnail.toPNG();
        mainWindow.webContents.send('screenshot-captured', {
          data: png.toString('base64'),
          mediaType: 'image/png',
        });
      } catch (err) {
        console.error('[electron] screenshot error:', err.message);
      }
    });
  } catch (err) {
    console.error('[electron] failed to register screenshot shortcut:', err.message);
  }

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  app.quit();
});

app.on('before-quit', () => {
  globalShortcut.unregisterAll();
  if (backendProcess) {
    console.log('[electron] killing backend process...');
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }
});
