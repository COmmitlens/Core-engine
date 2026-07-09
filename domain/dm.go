package domain

import (
	"core/config"
	"core/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ─── Interface ────────────────────────────────────────────────────────────────

type DMDomain interface {
	// Conversations
	GetOrCreateConversation(workspaceID, userA, userB int64) (models.DMConversation, error)
	GetConversationByID(id int64) (models.DMConversation, error)
	ListConversationsForUser(workspaceID, userID int64) ([]models.DMConversation, error)

	// Messages
	CreateMessage(msg models.DMMessage) (models.DMMessage, error)
	ListMessages(conversationID int64, limit, offset int) ([]models.DMMessage, error)
	MarkMessagesRead(conversationID, readerID int64) error

	// Helpers
	UnreadCount(conversationID, userID int64) (int64, error)
	LastMessage(conversationID int64) (models.DMMessage, error)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type DMDomainCtx struct{}

// normalise makes participant1 always the smaller ID so there is only one row
// per pair regardless of who initiates the conversation.
func normalise(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func (d *DMDomainCtx) GetOrCreateConversation(workspaceID, userA, userB int64) (models.DMConversation, error) {
	db := config.DbManager()
	p1, p2 := normalise(userA, userB)

	var conv models.DMConversation
	err := db.Where("workspace_id = ? AND participant1_id = ? AND participant2_id = ?",
		workspaceID, p1, p2).First(&conv).Error

	if err == nil {
		return conv, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.DMConversation{}, err
	}

	conv = models.DMConversation{
		WorkspaceID:    workspaceID,
		Participant1ID: p1,
		Participant2ID: p2,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&conv).Error; err != nil {
		return models.DMConversation{}, err
	}
	return conv, nil
}

func (d *DMDomainCtx) GetConversationByID(id int64) (models.DMConversation, error) {
	db := config.DbManager()
	var conv models.DMConversation
	if err := db.First(&conv, id).Error; err != nil {
		return models.DMConversation{}, err
	}
	return conv, nil
}

func (d *DMDomainCtx) ListConversationsForUser(workspaceID, userID int64) ([]models.DMConversation, error) {
	db := config.DbManager()
	var convs []models.DMConversation
	err := db.Where("workspace_id = ? AND (participant1_id = ? OR participant2_id = ?)",
		workspaceID, userID, userID).
		Order("updated_at DESC").
		Find(&convs).Error
	return convs, err
}

func (d *DMDomainCtx) CreateMessage(msg models.DMMessage) (models.DMMessage, error) {
	db := config.DbManager()
	msg.CreatedAt = time.Now()
	msg.UpdatedAt = time.Now()
	if err := db.Create(&msg).Error; err != nil {
		return models.DMMessage{}, err
	}
	// bump conversation updated_at so inbox ordering stays fresh
	db.Model(&models.DMConversation{}).Where("id = ?", msg.ConversationID).
		Update("updated_at", time.Now())
	return msg, nil
}

func (d *DMDomainCtx) ListMessages(conversationID int64, limit, offset int) ([]models.DMMessage, error) {
	db := config.DbManager()
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var msgs []models.DMMessage
	err := db.Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&msgs).Error
	return msgs, err
}

func (d *DMDomainCtx) MarkMessagesRead(conversationID, readerID int64) error {
	db := config.DbManager()
	return db.Model(&models.DMMessage{}).
		Where("conversation_id = ? AND sender_id != ? AND is_read = false", conversationID, readerID).
		Update("is_read", true).Error
}

func (d *DMDomainCtx) UnreadCount(conversationID, userID int64) (int64, error) {
	db := config.DbManager()
	var count int64
	err := db.Model(&models.DMMessage{}).
		Where("conversation_id = ? AND sender_id != ? AND is_read = false", conversationID, userID).
		Count(&count).Error
	return count, err
}

func (d *DMDomainCtx) LastMessage(conversationID int64) (models.DMMessage, error) {
	db := config.DbManager()
	var msg models.DMMessage
	err := db.Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		First(&msg).Error
	return msg, err
}
