package nexus

import "lingxi-agent/db"

// HumanGroupNickname 本端用户在群里的显示昵称（用于 @ 与成员列表）
func HumanGroupNickname() string {
	return db.NexusHumanNickname()
}
