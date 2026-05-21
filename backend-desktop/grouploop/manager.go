package grouploop

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"lingxi-agent/db"
)

// SpeakRequest 请求某 Agent 在群内发言
type SpeakRequest struct {
	RoomID    int64
	AgentID   int64
	AgentName string
	Trigger   string
	Forced    bool
}

// SpeakerFunc 由 handler 注入：执行一轮群聊 LLM
type SpeakerFunc func(req SpeakRequest)

// WakeMode 唤醒强度：避免 Agent 消息引发全员顺序接龙
type WakeMode int

const (
	WakeFull WakeMode = iota // 用户发言：多人并发抢话
	WakeLight                // Agent 发言：随机少量接话
)

var (
	speakFn      SpeakerFunc
	globalCtx    context.Context
	globalCancel context.CancelFunc
	rooms        sync.Map // roomID -> *roomRuntime

	wakeDebounceMu sync.Mutex
	wakeDebounce   = map[int64]*time.Timer{} // roomID -> debounced light wake
)

type roomRuntime struct {
	roomID int64
	loops  []*agentLoop
	cancel context.CancelFunc
}

// Init 注入发言回调
func Init(fn SpeakerFunc) {
	speakFn = fn
}

// BootAll 应用启动时恢复所有 active 群的本地 Agent 协程
func BootAll() {
	if globalCancel != nil {
		globalCancel()
	}
	globalCtx, globalCancel = context.WithCancel(context.Background())

	roomsList, err := db.ListGroupChats()
	if err != nil {
		return
	}
	for _, r := range roomsList {
		if r.Status == "active" {
			BootRoom(r.ID, false)
		}
	}
	slog.Info("grouploop BootAll done", "rooms", len(roomsList))
}

// BootRoom 为群内所有本端 joined Agent 启动常驻协程
func BootRoom(roomID int64, icebreaker bool) {
	room, err := db.GetGroupChat(roomID)
	if err != nil || room == nil || room.Status != "active" {
		return
	}

	members, err := db.ListGroupMembers(roomID)
	if err != nil {
		return
	}
	local := localJoinedAgents(members)
	if len(local) == 0 {
		return
	}

	StopRoom(roomID)

	ctx := globalCtx
	if ctx == nil {
		ctx = context.Background()
	}
	roomCtx, cancel := context.WithCancel(ctx)
	rt := &roomRuntime{roomID: roomID, cancel: cancel}

	for _, m := range local {
		al := &agentLoop{
			roomID: roomID,
			member: m,
			wakeCh: make(chan string, 16),
		}
		rt.loops = append(rt.loops, al)
		p, _ := db.GetPersonality(m.AgentID)
		go al.run(roomCtx, p)
		if icebreaker {
			al.icebreaker(p)
		}
	}

	rooms.Store(roomID, rt)
	slog.Info("grouploop BootRoom", "room", roomID, "agents", len(local), "icebreaker", icebreaker)
}

// WakeRoom 新消息后唤醒 Agent（按模式控制并发，避免 A→B→A 顺序接龙）
func WakeRoom(roomID int64, mode WakeMode) {
	v, ok := rooms.Load(roomID)
	if !ok {
		BootRoom(roomID, false)
		v, ok = rooms.Load(roomID)
		if !ok {
			return
		}
	}
	rt := v.(*roomRuntime)
	switch mode {
	case WakeFull:
		rt.wakeUserSubset()
	default:
		rt.wakeLightSubset()
	}
}

// ScheduleLightWake 延迟轻量唤醒（Agent 发消息后防抖，允许多人重叠发言）
func ScheduleLightWake(roomID int64, delay time.Duration) {
	wakeDebounceMu.Lock()
	defer wakeDebounceMu.Unlock()
	if t, ok := wakeDebounce[roomID]; ok {
		t.Stop()
	}
	wakeDebounce[roomID] = time.AfterFunc(delay, func() {
		wakeDebounceMu.Lock()
		delete(wakeDebounce, roomID)
		wakeDebounceMu.Unlock()
		WakeRoom(roomID, WakeLight)
	})
}

// StopRoom 停止某群所有协程
func StopRoom(roomID int64) {
	wakeDebounceMu.Lock()
	if t, ok := wakeDebounce[roomID]; ok {
		t.Stop()
		delete(wakeDebounce, roomID)
	}
	wakeDebounceMu.Unlock()

	v, ok := rooms.LoadAndDelete(roomID)
	if !ok {
		return
	}
	rt := v.(*roomRuntime)
	if rt.cancel != nil {
		rt.cancel()
	}
}

// StopAll 应用关闭时停止全部
func StopAll() {
	if globalCancel != nil {
		globalCancel()
	}
	rooms.Range(func(key, _ interface{}) bool {
		StopRoom(key.(int64))
		return true
	})
}

func (rt *roomRuntime) wakeUserSubset() {
	n := len(rt.loops)
	if n == 0 {
		return
	}
	// 用户发言：最多 2 个 Agent 尝试接话，且可能 0 人（避免全员 @ 用户）
	maxPick := 2
	if n < maxPick {
		maxPick = n
	}
	pick := rand.Intn(maxPick + 1)
	idx := rand.Perm(n)
	for i := 0; i < pick; i++ {
		rt.loops[idx[i]].wakeFull()
	}
}

func (rt *roomRuntime) wakeLightSubset() {
	n := len(rt.loops)
	if n == 0 {
		return
	}
	// Agent 发言后：40% 概率无人接话；否则最多 1 人轻量接话
	if rand.Intn(100) < 40 {
		return
	}
	maxPick := 1
	if n < maxPick {
		maxPick = n
	}
	pick := 1
	if maxPick == 0 {
		return
	}
	idx := rand.Perm(n)
	for i := 0; i < pick; i++ {
		rt.loops[idx[i]].wakeLight()
	}
}

func localJoinedAgents(members []db.GroupMember) []db.GroupMember {
	out := make([]db.GroupMember, 0, len(members))
	for _, m := range members {
		if m.IsLocal && m.Status == "joined" && m.AgentID > 0 {
			out = append(out, m)
		}
	}
	return out
}
