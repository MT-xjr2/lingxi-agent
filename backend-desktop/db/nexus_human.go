package db

// NexusHumanNickname 本端用户在群聊/A2A 中的显示昵称
func NexusHumanNickname() string {
	settings, _ := GetNexusSettings()
	if settings.Nickname != "" {
		return settings.Nickname
	}
	return "我"
}
