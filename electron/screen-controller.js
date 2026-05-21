const { execSync } = require('child_process');
const { screen } = require('electron');
const { getFullContext } = require('./context-sensor');

// Screen Agent 操控状态
let aborted = false;
let lastActionTime = 0;
const MIN_ACTION_INTERVAL_MS = 500;
let actionCount = 0;
const MAX_ACTIONS_PER_MINUTE = 60;
let actionCountResetTimer = null;

function setAborted(val) {
  aborted = !!val;
  if (val) {
    actionCount = 0;
  }
}
function isAborted() { return aborted; }

function checkRateLimit() {
  const now = Date.now();
  if (now - lastActionTime < MIN_ACTION_INTERVAL_MS) {
    const wait = MIN_ACTION_INTERVAL_MS - (now - lastActionTime);
    const { execSync: execS } = require('child_process');
    // 阻塞等待（简单版速率限制）
    try { execS(`sleep ${wait / 1000}`, { timeout: 2000 }); } catch {}
  }
  if (actionCount >= MAX_ACTIONS_PER_MINUTE) {
    throw new Error(`速率限制：每分钟最多 ${MAX_ACTIONS_PER_MINUTE} 次操作`);
  }
  actionCount++;
  lastActionTime = Date.now();
  if (!actionCountResetTimer) {
    actionCountResetTimer = setTimeout(() => {
      actionCount = 0;
      actionCountResetTimer = null;
    }, 60000);
  }
}

// ─── 截屏（由 main.js 提供 desktopCapturer 注入）──────────────────
let _captureScreenFn = null;

function setCaptureScreenFn(fn) {
  _captureScreenFn = fn;
}

async function captureScreen(region) {
  if (!_captureScreenFn) {
    throw new Error('captureScreen 未初始化，需由 main.js 注入');
  }
  return _captureScreenFn(region);
}

// ─── 增强上下文（活跃窗口 + 鼠标位置 + 剪贴板）─────────────────
function getEnhancedContext() {
  const ctx = getFullContext();
  const display = screen.getPrimaryDisplay();
  const cursor = screen.getCursorScreenPoint();
  let clipboardText = '';
  try {
    const { clipboard } = require('electron');
    clipboardText = clipboard.readText() || '';
    if (clipboardText.length > 500) {
      clipboardText = clipboardText.substring(0, 500) + '...';
    }
  } catch { /* ignore */ }

  return {
    ...ctx,
    cursorX: cursor.x,
    cursorY: cursor.y,
    screenWidth: display.size.width,
    screenHeight: display.size.height,
    scaleFactor: display.scaleFactor,
    clipboardPreview: clipboardText,
  };
}

// ─── macOS 桌面操控（AppleScript + osascript）──────────────────
function executeClickMac(x, y, button, count) {
  const clickType = button === 'right' ? 'rc' : (count === 2 ? 'dc' : 'c');
  try {
    execSync(`osascript -e 'do shell script "cliclick ${clickType}:${Math.round(x)},${Math.round(y)}"'`, {
      timeout: 5000, encoding: 'utf-8',
    });
    return { success: true };
  } catch {
    // cliclick 不可用时使用 AppleScript CGEvent 回退
    try {
      const script = `
        use framework "Foundation"
        use framework "ApplicationServices"
        set pt to current application's CGPointMake(${Math.round(x)}, ${Math.round(y)})
        set moveEvt to current application's CGEventCreateMouseEvent(missing value, current application's kCGEventMouseMoved, pt, 0)
        current application's CGEventPost(current application's kCGHIDEventTap, moveEvt)
        delay 0.05
        set downEvt to current application's CGEventCreateMouseEvent(missing value, current application's kCGEventLeftMouseDown, pt, 0)
        current application's CGEventPost(current application's kCGHIDEventTap, downEvt)
        delay 0.05
        set upEvt to current application's CGEventCreateMouseEvent(missing value, current application's kCGEventLeftMouseUp, pt, 0)
        current application's CGEventPost(current application's kCGHIDEventTap, upEvt)
      `;
      execSync(`osascript -l AppleScript -e '${script.replace(/'/g, "'\\''")}'`, {
        timeout: 5000, encoding: 'utf-8',
      });
      return { success: true };
    } catch (e2) {
      return { success: false, error: e2.message };
    }
  }
}

function executeTypeMac(text) {
  try {
    const escaped = text.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
    const script = `tell application "System Events" to keystroke "${escaped}"`;
    execSync(`osascript -e '${script}'`, { timeout: 5000, encoding: 'utf-8' });
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

function executeKeyPressMac(key, modifiers) {
  try {
    const keyMap = {
      'return': 36, 'enter': 36, 'tab': 48, 'space': 49, 'delete': 51, 'backspace': 51,
      'escape': 53, 'esc': 53, 'up': 126, 'down': 125, 'left': 123, 'right': 124,
      'home': 115, 'end': 119, 'pageup': 116, 'pagedown': 121,
      'f1': 122, 'f2': 120, 'f3': 99, 'f4': 118, 'f5': 96, 'f6': 97,
      'a': 0, 'b': 11, 'c': 8, 'd': 2, 'e': 14, 'f': 3, 'g': 5, 'h': 4,
      'i': 34, 'j': 38, 'k': 40, 'l': 37, 'm': 46, 'n': 45, 'o': 31,
      'p': 35, 'q': 12, 'r': 15, 's': 1, 't': 17, 'u': 32, 'v': 9,
      'w': 13, 'x': 7, 'y': 16, 'z': 6,
      '0': 29, '1': 18, '2': 19, '3': 20, '4': 21, '5': 23, '6': 22, '7': 26, '8': 28, '9': 25,
    };
    const keyCode = keyMap[key.toLowerCase()];
    if (keyCode === undefined) {
      return { success: false, error: `Unknown key: ${key}` };
    }
    const modList = (modifiers || []).map(m => {
      switch (m.toLowerCase()) {
        case 'cmd': case 'command': return 'command down';
        case 'shift': return 'shift down';
        case 'ctrl': case 'control': return 'control down';
        case 'alt': case 'option': return 'option down';
        default: return '';
      }
    }).filter(Boolean);

    const usingClause = modList.length > 0 ? ` using {${modList.join(', ')}}` : '';
    const script = `tell application "System Events" to key code ${keyCode}${usingClause}`;
    execSync(`osascript -e '${script}'`, { timeout: 5000, encoding: 'utf-8' });
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

function executeScrollMac(x, y, deltaX, deltaY) {
  try {
    // 先移动到目标位置再滚动
    const moveScript = `
      use framework "Foundation"
      use framework "ApplicationServices"
      set pt to current application's CGPointMake(${Math.round(x)}, ${Math.round(y)})
      set moveEvt to current application's CGEventCreateMouseEvent(missing value, current application's kCGEventMouseMoved, pt, 0)
      current application's CGEventPost(current application's kCGHIDEventTap, moveEvt)
    `;
    execSync(`osascript -l AppleScript -e '${moveScript.replace(/'/g, "'\\''")}'`, {
      timeout: 3000, encoding: 'utf-8',
    });
    const scrollScript = `
      use framework "Foundation"
      use framework "ApplicationServices"
      set scrollEvt to current application's CGEventCreateScrollWheelEvent(missing value, 0, 1, ${Math.round(deltaY || 0)})
      current application's CGEventPost(current application's kCGHIDEventTap, scrollEvt)
    `;
    execSync(`osascript -l AppleScript -e '${scrollScript.replace(/'/g, "'\\''")}'`, {
      timeout: 3000, encoding: 'utf-8',
    });
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

function executeMouseMoveMac(x, y) {
  try {
    const script = `
      use framework "Foundation"
      use framework "ApplicationServices"
      set pt to current application's CGPointMake(${Math.round(x)}, ${Math.round(y)})
      set moveEvt to current application's CGEventCreateMouseEvent(missing value, current application's kCGEventMouseMoved, pt, 0)
      current application's CGEventPost(current application's kCGHIDEventTap, moveEvt)
    `;
    execSync(`osascript -l AppleScript -e '${script.replace(/'/g, "'\\''")}'`, {
      timeout: 3000, encoding: 'utf-8',
    });
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

function executeOpenAppMac(appName) {
  try {
    execSync(`open -a "${appName.replace(/"/g, '\\"')}"`, { timeout: 5000 });
    return { success: true };
  } catch (e) {
    return { success: false, error: e.message };
  }
}

// ─── 统一操作入口 ────────────────────────────────────────────────
async function executeAction(action) {
  if (aborted) {
    return { success: false, error: 'Screen Agent 已中止' };
  }

  const isMac = process.platform === 'darwin';
  if (!isMac) {
    return { success: false, error: '当前仅支持 macOS' };
  }

  // 速率限制（截屏不受限）
  if (action.type !== 'screenshot') {
    try {
      checkRateLimit();
    } catch (e) {
      return { success: false, error: e.message };
    }
  }

  switch (action.type) {
    case 'click':
      return executeClickMac(action.x, action.y, action.button || 'left', action.count || 1);
    case 'type':
      return executeTypeMac(action.text || '');
    case 'keyPress':
      return executeKeyPressMac(action.key, action.modifiers);
    case 'scroll':
      return executeScrollMac(action.x || 0, action.y || 0, action.deltaX || 0, action.deltaY || 0);
    case 'moveMouse':
      return executeMouseMoveMac(action.x, action.y);
    case 'openApp':
      return executeOpenAppMac(action.appName || '');
    case 'screenshot':
      return captureScreen(action.region);
    default:
      return { success: false, error: `未知操作类型: ${action.type}` };
  }
}

// 批量执行（步间自动等待）
async function executeActions(actions, stepDelay) {
  const delay = stepDelay || 600;
  const results = [];
  for (const action of actions) {
    if (aborted) {
      results.push({ success: false, error: 'Screen Agent 已中止' });
      break;
    }
    const result = await executeAction(action);
    results.push(result);
    if (!result.success) break;
    await new Promise(r => setTimeout(r, delay));
  }
  return results;
}

module.exports = {
  captureScreen,
  setCaptureScreenFn,
  getEnhancedContext,
  executeAction,
  executeActions,
  setAborted,
  isAborted,
};
