package handler

import (
	"core/models"
	"core/service"
	"core/ws"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
)

type DMHandler struct {
	DMService service.DMService
	Hub       *ws.Hub
}

// ─── REST endpoints ───────────────────────────────────────────────────────────

// POST /dm/start
// Body: { "workspace_id": 1, "recipient_id": 2 }
// Opens (or fetches) a DM conversation between the caller and the recipient.
func (h *DMHandler) StartConversation(c echo.Context) error {
	var req models.StartDMReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	if req.WorkspaceID == 0 || req.RecipientID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "workspace_id and recipient_id are required"})
	}

	callerID := c.Get("id").(int64)

	resp, err := h.DMService.StartConversation(req, callerID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: "success", Data: resp})
}

// GET /dm/conversations?workspace_id=1
// Lists all DM conversations for the caller in a workspace.
func (h *DMHandler) ListConversations(c echo.Context) error {
	callerID := c.Get("id").(int64)
	workspaceID, err := strconv.ParseInt(c.QueryParam("workspace_id"), 10, 64)
	if err != nil || workspaceID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "workspace_id is required"})
	}

	resp, err := h.DMService.ListConversations(workspaceID, callerID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: "success", Data: resp})
}

// POST /dm/send
// Body: { "conversation_id": 1, "content": "hello" }
// Sends a message (persists + real-time broadcast).
func (h *DMHandler) SendMessage(c echo.Context) error {
	var req models.SendDMMessageReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	callerID := c.Get("id").(int64)

	resp, err := h.DMService.SendMessage(req, callerID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: "success", Data: resp})
}

// GET /dm/messages?conversation_id=1&limit=50&offset=0
// Paginated message history for a conversation.
func (h *DMHandler) ListMessages(c echo.Context) error {
	callerID := c.Get("id").(int64)

	convID, _ := strconv.ParseInt(c.QueryParam("conversation_id"), 10, 64)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	if convID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "conversation_id is required"})
	}

	req := models.ListDMMessagesReq{
		ConversationID: convID,
		Limit:          limit,
		Offset:         offset,
	}

	msgs, err := h.DMService.ListMessages(req, callerID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: "success", Data: msgs})
}

// POST /dm/read
// Body: { "conversation_id": 1 }
// Marks all unread messages in a conversation as read.
func (h *DMHandler) MarkRead(c echo.Context) error {
	var req models.MarkDMReadReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	callerID := c.Get("id").(int64)

	if err := h.DMService.MarkRead(req, callerID); err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, models.BasicResp{Message: "success"})
}

// ─── WebSocket endpoint ───────────────────────────────────────────────────────

// GET /dm/ws?conversation_id=1
// Upgrades the connection to WebSocket for real-time messaging.
// The caller must be a participant in the conversation (verified via JWT middleware).
//
// Protocol:
//
//	Client → Server  (send a message):
//	  { "conversation_id": 1, "content": "hello" }
//
//	Server → Client  (new message pushed):
//	  { "event": "new_message", "message": { ... } }
//
//	Server → Client  (mark read pushed):
//	  { "event": "mark_read", "message": { "conversation_id": 1, "sender_id": 99 } }
func (h *DMHandler) ServeWS(c echo.Context) error {
	callerID := c.Get("id").(int64)
	convID, err := strconv.ParseInt(c.QueryParam("conversation_id"), 10, 64)
	if err != nil || convID == 0 {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "conversation_id is required"})
	}

	conn, err := ws.Upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	client := &ws.Client{
		Hub:            h.Hub,
		Conn:           conn,
		Send:           make(chan []byte, 256),
		UserID:         callerID,
		ConversationID: convID,
	}
	h.Hub.Register <- client

	go client.WritePump()
	// ReadPump blocks; onMessage persists and broadcasts the message.
	client.ReadPump(func(cl *ws.Client, raw []byte) {
		var incoming models.WSIncomingMessage
		if err := json.Unmarshal(raw, &incoming); err != nil {
			return
		}
		// Force conversation_id from the query-param so clients can't spoof it.
		incoming.ConversationID = cl.ConversationID

		h.DMService.SendMessage(models.SendDMMessageReq{
			ConversationID: incoming.ConversationID,
			Content:        incoming.Content,
		}, cl.UserID)
	})

	return nil
}
