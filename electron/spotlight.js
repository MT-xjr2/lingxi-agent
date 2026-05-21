const { BrowserWindow, globalShortcut, ipcMain, screen } = require('electron');
const path = require('path');
const { getFullContext } = require('./context-sensor');

let spotlightWindow = null;
let backendPort = 3001;

function setBackendPort(port) {
  backendPort = port;
}

function createSpotlightWindow() {
  if (spotlightWindow && !spotlightWindow.isDestroyed()) {
    spotlightWindow.focus();
    return spotlightWindow;
  }

  const { width: screenWidth, height: screenHeight } = screen.getPrimaryDisplay().workAreaSize;

  spotlightWindow = new BrowserWindow({
    width: 640,
    height: 80,
    x: Math.round((screenWidth - 640) / 2),
    y: Math.round(screenHeight * 0.25),
    frame: false,
    transparent: true,
    alwaysOnTop: true,
    skipTaskbar: true,
    resizable: false,
    movable: true,
    hasShadow: true,
    vibrancy: 'under-window',
    visualEffectState: 'active',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
    show: false,
  });

  spotlightWindow.loadFile(path.join(__dirname, 'spotlight.html'));

  spotlightWindow.once('ready-to-show', () => {
    spotlightWindow.show();
    spotlightWindow.focus();
  });

  spotlightWindow.on('blur', () => {
    hideSpotlight();
  });

  spotlightWindow.on('closed', () => {
    spotlightWindow = null;
  });

  return spotlightWindow;
}

function showSpotlight() {
  if (spotlightWindow && !spotlightWindow.isDestroyed()) {
    spotlightWindow.show();
    spotlightWindow.focus();
    const ctx = getFullContext();
    spotlightWindow.webContents.send('spotlight-context', ctx);
  } else {
    createSpotlightWindow();
    setTimeout(() => {
      if (spotlightWindow && !spotlightWindow.isDestroyed()) {
        const ctx = getFullContext();
        spotlightWindow.webContents.send('spotlight-context', ctx);
      }
    }, 200);
  }
}

function hideSpotlight() {
  if (spotlightWindow && !spotlightWindow.isDestroyed()) {
    spotlightWindow.hide();
  }
}

function resizeSpotlight(height) {
  if (spotlightWindow && !spotlightWindow.isDestroyed()) {
    const bounds = spotlightWindow.getBounds();
    spotlightWindow.setBounds({ ...bounds, height: Math.min(height, 500) });
  }
}

function registerShortcut() {
  try {
    globalShortcut.register('CommandOrControl+Shift+Space', () => {
      if (spotlightWindow && spotlightWindow.isVisible()) {
        hideSpotlight();
      } else {
        showSpotlight();
      }
    });
    console.log('[spotlight] shortcut registered: Cmd+Shift+Space');
  } catch (err) {
    console.error('[spotlight] failed to register shortcut:', err.message);
  }
}

function getPort() {
  return backendPort;
}

// IPC handlers
function registerIPC() {
  ipcMain.handle('show-spotlight', () => showSpotlight());
  ipcMain.handle('hide-spotlight', () => hideSpotlight());
  ipcMain.handle('resize-spotlight', (_e, height) => resizeSpotlight(height));
  ipcMain.handle('get-active-context', () => getFullContext());
  ipcMain.handle('spotlight-get-port', () => getPort());
}

module.exports = {
  createSpotlightWindow,
  showSpotlight,
  hideSpotlight,
  registerShortcut,
  registerIPC,
  setBackendPort,
};
