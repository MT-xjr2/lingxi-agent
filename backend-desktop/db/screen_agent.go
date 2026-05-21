package db

import "log/slog"

type ScreenAction struct {
	ID               int64  `json:"id"`
	SessionID        int64  `json:"session_id"`
	MessageID        int64  `json:"message_id"`
	ActionType       string `json:"action_type"`
	ActionData       string `json:"action_data"`
	ScreenshotBefore string `json:"screenshot_before"`
	ScreenshotAfter  string `json:"screenshot_after"`
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg"`
	CreatedAt        string `json:"created_at"`
}

func InsertScreenAction(a *ScreenAction) (int64, error) {
	res, err := DB.Exec(`INSERT INTO screen_actions (session_id, message_id, action_type, action_data, screenshot_before, screenshot_after, status, error_msg)
		VALUES (?,?,?,?,?,?,?,?)`,
		a.SessionID, a.MessageID, a.ActionType, a.ActionData, a.ScreenshotBefore, a.ScreenshotAfter, a.Status, a.ErrorMsg)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateScreenActionStatus(id int64, status, errorMsg, screenshotAfter string) error {
	_, err := DB.Exec(`UPDATE screen_actions SET status=?, error_msg=?, screenshot_after=? WHERE id=?`,
		status, errorMsg, screenshotAfter, id)
	return err
}

func ListScreenActions(sessionID int64, limit int) []ScreenAction {
	if limit <= 0 {
		limit = 50
	}
	rows, err := DB.Query(`SELECT id, session_id, message_id, action_type, action_data,
		screenshot_before, screenshot_after, status, error_msg, created_at
		FROM screen_actions WHERE session_id=? ORDER BY id DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		slog.Warn("list screen actions error", "err", err)
		return nil
	}
	defer rows.Close()
	var list []ScreenAction
	for rows.Next() {
		var a ScreenAction
		rows.Scan(&a.ID, &a.SessionID, &a.MessageID, &a.ActionType, &a.ActionData,
			&a.ScreenshotBefore, &a.ScreenshotAfter, &a.Status, &a.ErrorMsg, &a.CreatedAt)
		list = append(list, a)
	}
	return list
}

func GetAgentScreenConfig(agentID int64) (bool, string) {
	var enabled int
	var config string
	err := DB.QueryRow(`SELECT COALESCE(screen_agent_enabled, 0), COALESCE(screen_agent_config, '{}') FROM agents WHERE id=?`, agentID).Scan(&enabled, &config)
	if err != nil {
		return false, "{}"
	}
	return enabled == 1, config
}

func SetAgentScreenConfig(agentID int64, enabled bool, config string) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := DB.Exec(`UPDATE agents SET screen_agent_enabled=?, screen_agent_config=? WHERE id=?`, e, config, agentID)
	return err
}
