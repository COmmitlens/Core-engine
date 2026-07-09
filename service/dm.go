package service

import (
	"core/domain"
	"core/models"
	"core/ws"
	"errors"
	"fmt"
)

// DMService orchestrates DM business logic and real-time delivery via the hub.
type DMService struct {
	DMDomain              domain.DMDomain
	UserDomain            domain.UserDomain
	ManageWorkspaceDomain domain.ManageWorkspaceDomain
	Hub                   *ws.Hub
}

// ─── Conversation ─────────────────────────────────────────────────────────────

// StartConversation finds or creates a DM conversation between the caller and
// the recipient, verifying both are members of the workspace.
func (s *DMService) StartConversation(req models.StartDMReq, callerID int64) (models.DMConversationResp, error) {
	if callerID == req.RecipientID {
		return models.DMConversationResp{}, errors.New("cannot start a DM with yourself")
	}

	// Verify caller is in workspace.
	if _, err := s.ManageWorkspaceDomain.GetByWorkspaceIdAndUser(models.ManageWorkspace{
		WorkspaceID:  req.WorkspaceID,
		JoinedUserID: callerID,
	}); err != nil {
		return models.DMConversationResp{}, fmt.Errorf("caller is not a workspace member")
	}

	// Verify recipient is in workspace.
	if _, err := s.ManageWorkspaceDomain.GetByWorkspaceIdAndUser(models.ManageWorkspace{
		WorkspaceID:  req.WorkspaceID,
		JoinedUserID: req.RecipientID,
	}); err != nil {
		return models.DMConversationResp{}, fmt.Errorf("recipient is not a workspace member")
	}

	conv, err := s.DMDomain.GetOrCreateConversation(req.WorkspaceID, callerID, req.RecipientID)
	if err != nil {
		return models.DMConversationResp{}, err
	}

	return s.enrichConversation(conv, callerID)
}

// ListConversations returns all DM conversations for the caller in a workspace.
func (s *DMService) ListConversations(workspaceID, callerID int64) ([]models.DMConversationResp, error) {
	convs, err := s.DMDomain.ListConversationsForUser(workspaceID, callerID)
	if err != nil {
		return nil, err
	}

	var result []models.DMConversationResp
	for _, c := range convs {
		resp, err := s.enrichConversation(c, callerID)
		if err != nil {
			continue // skip broken rows rather than failing the whole list
		}
		result = append(result, resp)
	}
	return result, nil
}

// enrichConversation joins user data + unread count + last message preview.
func (s *DMService) enrichConversation(conv models.DMConversation, callerID int64) (models.DMConversationResp, error) {
	otherID := conv.Participant2ID
	if otherID == callerID {
		otherID = conv.Participant1ID
	}

	other, err := s.UserDomain.Get(models.GetUserParam{ID: otherID})
	if err != nil {
		return models.DMConversationResp{}, err
	}

	unread, _ := s.DMDomain.UnreadCount(conv.ID, callerID)

	var lastMsg string
	var lastMsgAt = conv.CreatedAt
	if lm, err := s.DMDomain.LastMessage(conv.ID); err == nil {
		lastMsg = lm.Content
		lastMsgAt = lm.CreatedAt
	}

	return models.DMConversationResp{
		ID:             conv.ID,
		WorkspaceID:    conv.WorkspaceID,
		Participant1ID: conv.Participant1ID,
		Participant2ID: conv.Participant2ID,
		OtherUserID:    other.ID,
		OtherUserName:  other.Name,
		OtherUsername:  other.Username,
		UnreadCount:    unread,
		LastMessage:    lastMsg,
		LastMessageAt:  lastMsgAt,
		CreatedAt:      conv.CreatedAt,
	}, nil
}

// ─── Messages ─────────────────────────────────────────────────────────────────

// SendMessage persists a message then broadcasts it via WebSocket to the
// conversation room so the other participant receives it in real-time.
func (s *DMService) SendMessage(req models.SendDMMessageReq, senderID int64) (models.DMMessageResp, error) {
	// Verify the sender belongs to this conversation.
	conv, err := s.DMDomain.GetConversationByID(req.ConversationID)
	if err != nil {
		return models.DMMessageResp{}, errors.New("conversation not found")
	}
	if conv.Participant1ID != senderID && conv.Participant2ID != senderID {
		return models.DMMessageResp{}, errors.New("forbidden: you are not a participant")
	}

	msg, err := s.DMDomain.CreateMessage(models.DMMessage{
		ConversationID: req.ConversationID,
		SenderID:       senderID,
		Content:        req.Content,
	})
	if err != nil {
		return models.DMMessageResp{}, err
	}

	sender, _ := s.UserDomain.Get(models.GetUserParam{ID: senderID})

	resp := models.DMMessageResp{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		SenderName:     sender.Name,
		Content:        msg.Content,
		IsRead:         msg.IsRead,
		CreatedAt:      msg.CreatedAt,
	}

	// Push to connected WebSocket clients in this conversation room.
	if s.Hub != nil {
		s.Hub.Broadcast(req.ConversationID, models.WSOutgoingMessage{
			Event:   "new_message",
			Message: resp,
		})
	}

	return resp, nil
}

// ListMessages returns paginated messages for a conversation.
func (s *DMService) ListMessages(req models.ListDMMessagesReq, callerID int64) ([]models.DMMessageResp, error) {
	conv, err := s.DMDomain.GetConversationByID(req.ConversationID)
	if err != nil {
		return nil, errors.New("conversation not found")
	}
	if conv.Participant1ID != callerID && conv.Participant2ID != callerID {
		return nil, errors.New("forbidden: you are not a participant")
	}

	msgs, err := s.DMDomain.ListMessages(req.ConversationID, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}

	var result []models.DMMessageResp
	for _, m := range msgs {
		sender, _ := s.UserDomain.Get(models.GetUserParam{ID: m.SenderID})
		result = append(result, models.DMMessageResp{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			SenderName:     sender.Name,
			Content:        m.Content,
			IsRead:         m.IsRead,
			CreatedAt:      m.CreatedAt,
		})
	}
	return result, nil
}

// MarkRead marks all unread messages in a conversation as read for the caller
// and broadcasts a "mark_read" event so the sender's UI can update.
func (s *DMService) MarkRead(req models.MarkDMReadReq, callerID int64) error {
	conv, err := s.DMDomain.GetConversationByID(req.ConversationID)
	if err != nil {
		return errors.New("conversation not found")
	}
	if conv.Participant1ID != callerID && conv.Participant2ID != callerID {
		return errors.New("forbidden: you are not a participant")
	}

	if err := s.DMDomain.MarkMessagesRead(req.ConversationID, callerID); err != nil {
		return err
	}

	// Notify room so the other client's unread badge clears.
	if s.Hub != nil {
		s.Hub.Broadcast(req.ConversationID, models.WSOutgoingMessage{
			Event: "mark_read",
			Message: models.DMMessageResp{
				ConversationID: req.ConversationID,
				SenderID:       callerID,
			},
		})
	}
	return nil
}
