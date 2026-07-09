package models

import "time"

// DMConversation represents a 1-to-1 DM thread between two workspace members.
// Unique constraint: (workspace_id, participant1_id, participant2_id)
// where participant1_id < participant2_id (enforced in domain to avoid duplicates).
type DMConversation struct {
	ID             int64     `gorm:"column:id;primaryKey"                 json:"id"`
	WorkspaceID    int64     `gorm:"column:workspace_id;not null;index"   json:"workspace_id"`
	Participant1ID int64     `gorm:"column:participant1_id;not null"      json:"participant1_id"`
	Participant2ID int64     `gorm:"column:participant2_id;not null"      json:"participant2_id"`
	CreatedAt      time.Time `gorm:"column:created_at"                    json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"                    json:"updated_at"`
}

func (DMConversation) TableName() string { return "dm_conversations" }

// DMMessage is a single message inside a DM conversation.
type DMMessage struct {
	ID             int64     `gorm:"column:id;primaryKey"                           json:"id"`
	ConversationID int64     `gorm:"column:conversation_id;not null;index"          json:"conversation_id"`
	SenderID       int64     `gorm:"column:sender_id;not null"                      json:"sender_id"`
	Content        string    `gorm:"column:content;type:text;not null"              json:"content"`
	IsRead         bool      `gorm:"column:is_read;default:false"                   json:"is_read"`
	CreatedAt      time.Time `gorm:"column:created_at"                              json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"                              json:"updated_at"`
}

func (DMMessage) TableName() string { return "dm_messages" }

// ─── Request / Response DTOs ──────────────────────────────────────────────────

type StartDMReq struct {
	WorkspaceID int64 `json:"workspace_id"   validate:"required"`
	RecipientID int64 `json:"recipient_id"   validate:"required"`
}

type SendDMMessageReq struct {
	ConversationID int64  `json:"conversation_id" validate:"required"`
	Content        string `json:"content"         validate:"required"`
}

type MarkDMReadReq struct {
	ConversationID int64 `json:"conversation_id" validate:"required"`
}

type ListDMMessagesReq struct {
	ConversationID int64 `json:"conversation_id"`
	// pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type DMConversationResp struct {
	ID             int64 `json:"id"`
	WorkspaceID    int64 `json:"workspace_id"`
	Participant1ID int64 `json:"participant1_id"`
	Participant2ID int64 `json:"participant2_id"`
	// Enriched fields (joined from users table in service layer)
	OtherUserID   int64     `json:"other_user_id"`
	OtherUserName string    `json:"other_user_name"`
	OtherUsername string    `json:"other_username"`
	UnreadCount   int64     `json:"unread_count"`
	LastMessage   string    `json:"last_message"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type DMMessageResp struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Content        string    `json:"content"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
}

// WSIncomingMessage is the payload a client sends over the WebSocket connection
// when the user types a message.
type WSIncomingMessage struct {
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content"`
}

// WSOutgoingMessage is pushed to connected clients by the hub.
type WSOutgoingMessage struct {
	Event   string        `json:"event"` // "new_message" | "mark_read"
	Message DMMessageResp `json:"message"`
}
