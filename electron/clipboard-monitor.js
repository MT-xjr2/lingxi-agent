const { clipboard, BrowserWindow } = require('electron');

let lastText = '';
let monitorInterval = null;
let enabled = true;
let mainWindowRef = null;

const POLL_INTERVAL = 2000;

// 内容分类正则
const patterns = {
  stackTrace: /(?:Error:|Exception:|Traceback|at\s+\w+\.\w+|File\s+".*",\s+line\s+\d+|panic:|goroutine\s+\d+)/m,
  url: /^https?:\/\/[^\s]+$/m,
  code: /(?:function\s+\w+|const\s+\w+\s*=|import\s+{|from\s+['"]|def\s+\w+|class\s+\w+|pub\s+fn|func\s+\w+|package\s+\w+)/m,
  json: /^\s*[{[]/,
  command: /^\s*(?:\$|>|#)\s*.+/m,
  longEnglish: /^[a-zA-Z\s,.'";:!?()-]{100,}$/,
};

function classifyContent(text) {
  if (!text || text.length < 10) return null;
  if (text.length > 5000) return null;

  if (patterns.stackTrace.test(text)) {
    return {
      type: 'error',
      label: '检测到错误日志',
      action: '需要分析原因吗？',
      icon: '🔍',
    };
  }

  if (patterns.command.test(text) && text.split('\n').length <= 5) {
    return {
      type: 'command',
      label: '检测到命令',
      action: '需要解释这条命令吗？',
      icon: '💡',
    };
  }

  if (patterns.code.test(text)) {
    return {
      type: 'code',
      label: '检测到代码片段',
      action: '需要解释或优化吗？',
      icon: '💡',
    };
  }

  if (patterns.url.test(text.trim())) {
    return {
      type: 'url',
      label: '检测到链接',
      action: '需要提取页面内容吗？',
      icon: '🔗',
    };
  }

  if (patterns.longEnglish.test(text) && text.length > 200) {
    return {
      type: 'english',
      label: '检测到英文长文',
      action: '需要翻译或总结吗？',
      icon: '🌐',
    };
  }

  return null;
}

function start(mainWindow) {
  mainWindowRef = mainWindow;
  lastText = clipboard.readText() || '';

  monitorInterval = setInterval(() => {
    if (!enabled) return;
    if (!mainWindowRef || mainWindowRef.isDestroyed()) return;

    const currentText = clipboard.readText();
    if (!currentText || currentText === lastText) return;

    lastText = currentText;

    const classification = classifyContent(currentText);
    if (!classification) return;

    const preview = currentText.length > 100
      ? currentText.substring(0, 100) + '...'
      : currentText;

    mainWindowRef.webContents.send('clipboard-suggestion', {
      ...classification,
      preview,
      fullText: currentText,
      timestamp: Date.now(),
    });
  }, POLL_INTERVAL);
}

function stop() {
  if (monitorInterval) {
    clearInterval(monitorInterval);
    monitorInterval = null;
  }
}

function setEnabled(val) {
  enabled = !!val;
}

function isEnabled() {
  return enabled;
}

module.exports = { start, stop, setEnabled, isEnabled };
