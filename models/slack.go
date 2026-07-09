package models

import "time"

// SlackInstallation stores one row per Slack workspace that installs CommitLens.
// WorkspaceID links the Slack team to the CommitLens workspace that connected it.
type SlackInstallation struct {
	ID            int64     `gorm:"primaryKey"                      json:"id"`
	WorkspaceID   int64     `gorm:"index;not null"                  json:"workspace_id"`  // FK → workspaces.id
	SlackTeamID   string    `gorm:"uniqueIndex;not null"            json:"slack_team_id"` // e.g. T012AB3CD
	SlackTeamName string    `gorm:"not null"                        json:"slack_team_name"`
	BotToken      string    `gorm:"not null"                        json:"bot_token"` // xoxb-... encrypt in prod
	CreatedAt     time.Time `gorm:"autoCreateTime"                  json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"                  json:"updated_at"`
}

func (SlackInstallation) TableName() string { return "slack_installations" }

// ─── Slack Event API payloads ─────────────────────────────────────────────────

// SlackEventPayload is the top-level envelope Slack POSTs to /v1/slack/events.
type SlackEventPayload struct {
	Type      string     `json:"type"`      // "url_verification" | "event_callback"
	Challenge string     `json:"challenge"` // only present on url_verification
	TeamID    string     `json:"team_id"`
	Event     SlackEvent `json:"event"`
}

// SlackEvent is the inner event object inside an event_callback payload.
type SlackEvent struct {
	Type     string `json:"type"`    // "message"
	SubType  string `json:"subtype"` // "bot_message", "message_changed", etc. — ignore these
	Channel  string `json:"channel"`
	User     string `json:"user"`
	Text     string `json:"text"`
	Ts       string `json:"ts"`
	ThreadTs string `json:"thread_ts"` // set if message is inside a thread
	BotID    string `json:"bot_id"`    // non-empty if sent by a bot — we skip these
}

// SlackMessageEvent is the cleaned-up struct passed to the service layer.
type SlackMessageEvent struct {
	TeamID    string
	ChannelID string
	UserID    string
	Text      string
	Timestamp string
	ThreadTS  string
}

// ─── OAuth response from Slack ────────────────────────────────────────────────

type SlackOAuthResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`        // set when ok = false
	AccessToken string `json:"access_token"` // bot token (xoxb-...)
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
}
