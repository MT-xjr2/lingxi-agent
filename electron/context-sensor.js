const { execSync } = require('child_process');

function getActiveWindow() {
  if (process.platform === 'darwin') {
    return getActiveWindowMac();
  } else if (process.platform === 'win32') {
    return getActiveWindowWin();
  }
  return { appName: '', windowTitle: '' };
}

function getActiveWindowMac() {
  try {
    const script = `
      tell application "System Events"
        set frontApp to name of first process whose frontmost is true
        set frontWindow to ""
        try
          set frontWindow to name of front window of first process whose frontmost is true
        end try
      end tell
      return frontApp & "|" & frontWindow
    `;
    const result = execSync(`osascript -e '${script}'`, {
      timeout: 3000,
      encoding: 'utf-8',
    }).trim();
    const [appName, windowTitle] = result.split('|');
    return { appName: appName || '', windowTitle: windowTitle || '' };
  } catch {
    return { appName: '', windowTitle: '' };
  }
}

function getActiveWindowWin() {
  try {
    const ps = `
      Add-Type @"
        using System;
        using System.Runtime.InteropServices;
        public class WinAPI {
          [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
          [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, System.Text.StringBuilder text, int count);
          [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);
        }
"@
      $h = [WinAPI]::GetForegroundWindow()
      $sb = New-Object System.Text.StringBuilder 256
      [WinAPI]::GetWindowText($h, $sb, 256) | Out-Null
      $pid = 0
      [WinAPI]::GetWindowThreadProcessId($h, [ref]$pid) | Out-Null
      $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
      "$($proc.ProcessName)|$($sb.ToString())"
    `;
    const result = execSync(`powershell -NoProfile -Command "${ps.replace(/"/g, '\\"')}"`, {
      timeout: 5000, encoding: 'utf-8',
    }).trim();
    const [appName, windowTitle] = result.split('|');
    return { appName: appName || '', windowTitle: windowTitle || '' };
  } catch {
    return { appName: '', windowTitle: '' };
  }
}

function getBrowserURL() {
  if (process.platform !== 'darwin') return '';
  try {
    const script = `
      tell application "System Events"
        set frontApp to name of first process whose frontmost is true
      end tell
      if frontApp is "Google Chrome" then
        tell application "Google Chrome"
          return URL of active tab of front window
        end tell
      else if frontApp is "Safari" then
        tell application "Safari"
          return URL of current tab of front window
        end tell
      else if frontApp is "Microsoft Edge" then
        tell application "Microsoft Edge"
          return URL of active tab of front window
        end tell
      else if frontApp is "Arc" then
        tell application "Arc"
          return URL of active tab of front window
        end tell
      end if
      return ""
    `;
    return execSync(`osascript -e '${script}'`, {
      timeout: 3000,
      encoding: 'utf-8',
    }).trim();
  } catch {
    return '';
  }
}

function getContextForApp(appName) {
  const name = (appName || '').toLowerCase();
  if (name.includes('code') || name.includes('cursor') || name.includes('idea') ||
      name.includes('xcode') || name.includes('sublime') || name.includes('atom') ||
      name.includes('vim') || name.includes('emacs') || name.includes('jetbrains')) {
    return 'ide';
  }
  if (name.includes('chrome') || name.includes('safari') || name.includes('firefox') ||
      name.includes('edge') || name.includes('arc') || name.includes('brave') || name.includes('opera')) {
    return 'browser';
  }
  if (name.includes('terminal') || name.includes('iterm') || name.includes('alacritty') ||
      name.includes('warp') || name.includes('powershell') || name.includes('cmd') ||
      name.includes('kitty') || name.includes('hyper')) {
    return 'terminal';
  }
  if (name.includes('word') || name.includes('pages') || name.includes('notion') ||
      name.includes('obsidian') || name.includes('typora') || name.includes('bear')) {
    return 'writing';
  }
  if (name.includes('finder') || name.includes('explorer')) {
    return 'filemanager';
  }
  return 'general';
}

function getQuickActions(contextType) {
  switch (contextType) {
    case 'ide':
      return [
        { id: 'explain', label: '解释代码', icon: '💡' },
        { id: 'test', label: '生成测试', icon: '🧪' },
        { id: 'doc', label: '查文档', icon: '📖' },
        { id: 'optimize', label: '优化代码', icon: '⚡' },
      ];
    case 'browser':
      return [
        { id: 'summarize', label: '总结页面', icon: '📋' },
        { id: 'translate', label: '翻译', icon: '🌐' },
        { id: 'extract', label: '提取要点', icon: '✨' },
      ];
    case 'terminal':
      return [
        { id: 'explain-cmd', label: '解释命令', icon: '💡' },
        { id: 'fix-error', label: '修复错误', icon: '🔧' },
        { id: 'suggest', label: '推荐命令', icon: '📝' },
      ];
    case 'writing':
      return [
        { id: 'polish', label: '润色文字', icon: '✏️' },
        { id: 'translate', label: '翻译', icon: '🌐' },
        { id: 'expand', label: '扩展内容', icon: '📝' },
      ];
    default:
      return [
        { id: 'translate', label: '翻译', icon: '🌐' },
        { id: 'continue', label: '继续上次', icon: '💬' },
        { id: 'new-task', label: '新建任务', icon: '➕' },
      ];
  }
}

function getFullContext() {
  const win = getActiveWindow();
  const url = getBrowserURL();
  const contextType = getContextForApp(win.appName);
  const quickActions = getQuickActions(contextType);

  return {
    appName: win.appName,
    windowTitle: win.windowTitle,
    url,
    contextType,
    quickActions,
  };
}

module.exports = { getActiveWindow, getBrowserURL, getContextForApp, getQuickActions, getFullContext };
