package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lingxi-agent/vectordb"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher 监控文件夹变化并触发索引更新
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	debounce map[string]*time.Timer
	stop     chan struct{}
}

var Global *FileWatcher

// Start 启动文件监控
func Start() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create file watcher", "err", err)
		return
	}

	Global = &FileWatcher{
		watcher:  w,
		debounce: make(map[string]*time.Timer),
		stop:     make(chan struct{}),
	}

	// 加载已配置的监控目录
	dirs, err := vectordb.ListWatchedDirs()
	if err != nil {
		slog.Error("failed to load watched dirs", "err", err)
	}
	for _, d := range dirs {
		if d.Enabled {
			Global.AddDir(d.DirPath)
		}
	}

	go Global.run()
	slog.Info("file watcher started", "dirs", len(dirs))
}

// Stop 停止文件监控
func Stop() {
	if Global == nil {
		return
	}
	close(Global.stop)
	Global.watcher.Close()
}

// AddDir 添加监控目录（递归添加子目录）
func (fw *FileWatcher) AddDir(dir string) {
	if _, err := os.Stat(dir); err != nil {
		slog.Warn("watched dir does not exist", "dir", dir)
		return
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// 跳过隐藏目录和 node_modules 等
			name := info.Name()
			if len(name) > 0 && name[0] == '.' {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "__pycache__" || name == ".git" {
				return filepath.SkipDir
			}
			fw.watcher.Add(path)
		}
		return nil
	})
}

// RemoveDir 移除监控目录
func (fw *FileWatcher) RemoveDir(dir string) {
	fw.watcher.Remove(dir)
}

func (fw *FileWatcher) run() {
	for {
		select {
		case <-fw.stop:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("file watcher error", "err", err)
		}
	}
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// 只处理支持的文件类型
	if !vectordb.IsSupportedFile(path) {
		return
	}

	// 跳过隐藏文件
	base := filepath.Base(path)
	if len(base) > 0 && base[0] == '.' {
		return
	}

	// 防抖：同一文件 2 秒内的多次事件合并为一次处理
	fw.mu.Lock()
	if timer, exists := fw.debounce[path]; exists {
		timer.Stop()
	}

	fw.debounce[path] = time.AfterFunc(2*time.Second, func() {
		fw.mu.Lock()
		delete(fw.debounce, path)
		fw.mu.Unlock()

		fw.processFileChange(path, event.Op)
	})
	fw.mu.Unlock()
}

func (fw *FileWatcher) processFileChange(path string, op fsnotify.Op) {
	switch {
	case op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename):
		slog.Debug("watched file removed", "path", path)
		vectordb.RemoveWatchedFile(path)

	case op.Has(fsnotify.Create) || op.Has(fsnotify.Write):
		// 等待文件写入完成
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(path); err != nil {
			return
		}
		slog.Debug("watched file changed, reindexing", "path", path)
		vectordb.IndexWatchedFile(path)
	}
}
