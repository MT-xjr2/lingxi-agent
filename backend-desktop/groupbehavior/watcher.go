package groupbehavior

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"lingxi-agent/db"
)

// ColdStartTrigger 由调用方注入：当冷场触发时，被调用以推进群聊一个轮次（会传入 isColdStart=true）
type ColdStartTrigger func(roomID int64)

var (
	coldStartMu      sync.Mutex
	coldStartRunning bool
	coldStartTrigger ColdStartTrigger
	coldStartCancel  context.CancelFunc

	coldStartCooldown  = 90 * time.Second
	coldStartThreshold = 90 * time.Second
	lastColdStartAt    = sync.Map{} // roomID -> time.Time
)

// StartColdStartWatcher 启动冷场守望者；ctx 取消时退出
func StartColdStartWatcher(parent context.Context, trigger ColdStartTrigger) {
	coldStartMu.Lock()
	if coldStartRunning {
		coldStartMu.Unlock()
		return
	}
	coldStartRunning = true
	coldStartTrigger = trigger
	ctx, cancel := context.WithCancel(parent)
	coldStartCancel = cancel
	coldStartMu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		slog.Info("groupbehavior cold-start watcher started")
		for {
			select {
			case <-ctx.Done():
				slog.Info("groupbehavior cold-start watcher stopped")
				return
			case <-ticker.C:
				scanColdRooms()
			}
		}
	}()
}

// StopColdStartWatcher 停止
func StopColdStartWatcher() {
	coldStartMu.Lock()
	defer coldStartMu.Unlock()
	if coldStartCancel != nil {
		coldStartCancel()
	}
	coldStartRunning = false
}

func scanColdRooms() {
	rooms, err := db.ListGroupChats()
	if err != nil {
		return
	}
	now := time.Now()
	for _, r := range rooms {
		if r.Status != "active" {
			continue
		}
		// 仅 host (本端创建者) 负责调度
		if !r.CreatedByLocal {
			continue
		}
		last, _ := db.GetLastGroupMessageTime(r.ID)
		if last.IsZero() {
			continue
		}
		if now.Sub(last) < coldStartThreshold {
			continue
		}
		// 冷却检查
		if v, ok := lastColdStartAt.Load(r.ID); ok {
			if t, _ := v.(time.Time); now.Sub(t) < coldStartCooldown {
				continue
			}
		}
		// 检查是否有可发言的本地 Agent（避免空跑）
		members, _ := db.ListGroupMembers(r.ID)
		if len(localJoinedAgents(members)) == 0 {
			continue
		}
		lastColdStartAt.Store(r.ID, now)
		slog.Info("cold-start triggered", "room", r.ID, "idleSec", int(now.Sub(last).Seconds()))
		if coldStartTrigger != nil {
			go coldStartTrigger(r.ID)
		}
	}
}
