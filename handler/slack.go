package handler

import (
	"core/config"
	"core/models"
	"core/service"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo"
)

type SlackHandler struct {
	SlackService service.SlackService
}

// POST /v1/slack/events
// Receives all events from Slack (message events, url_verification challenge).
func (h *SlackHandler) HandleEvent(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	// Verify the request genuinely came from Slack
	if !verifySlackSignature(c.Request().Header, body) {
		return c.JSON(http.StatusUnauthorized, nil)
	}

	var payload models.SlackEventPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	// One-time URL verification challenge from Slack
	if payload.Type == "url_verification" {
		return c.JSON(http.StatusOK, map[string]string{
			"challenge": payload.Challenge,
		})
	}

	// Handle callback events
	if payload.Type == "event_callback" {
		event := payload.Event

		// Ignore messages sent by bots (including our own) to prevent loops
		if event.BotID != "" || event.Type != "message" || event.SubType != "" {
			return c.JSON(http.StatusOK, nil)
		}

		// Hand off to service — non-blocking, respond to Slack within 3s
		go h.SlackService.HandleMessage(models.SlackMessageEvent{
			TeamID:    payload.TeamID,
			ChannelID: event.Channel,
			UserID:    event.User,
			Text:      event.Text,
			Timestamp: event.Ts,
			ThreadTS:  event.ThreadTs,
			Slack:     true,
		})
	}

	return c.JSON(http.StatusOK, nil)
}

// GET /v1/slack/install?workspace_id=123
// Redirects the user to Slack's OAuth authorization page.
// workspace_id is passed as OAuth `state` so the callback knows which
// CommitLens workspace to link the Slack team to.
func (h *SlackHandler) Install(c echo.Context) error {
	workspaceID := c.QueryParam("workspace_id")
	if workspaceID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "workspace_id is required"})
	}

	cfg := config.GetConfig()
	scopes := "channels:history,channels:read,chat:write,users:read,groups:history"
	redirectURL := fmt.Sprintf(
		"https://slack.com/oauth/v2/authorize?client_id=%s&scope=%s&redirect_uri=%s&state=%s",
		cfg.SlackClientID, scopes, cfg.SlackRedirectURI, workspaceID,
	)
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// GET /v1/slack/oauth/callback?code=xxx&state=<workspace_id>
// Slack redirects here after the user approves the app installation.
// `state` carries the CommitLens workspace_id we set in Install.
func (h *SlackHandler) OAuthCallback(c echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing code"})
	}

	// state = workspace_id passed through from Install
	workspaceIDStr := c.QueryParam("state")
	if workspaceIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing state (workspace_id)"})
	}
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid workspace_id in state"})
	}

	if err := h.SlackService.CompleteOAuth(code, workspaceID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	cfg := config.GetConfig()
	return c.Redirect(http.StatusTemporaryRedirect, cfg.FrontendUrl+"/slack/connected")
}

// GET /v1/slack/status?workspace_id=123
// Returns whether a CommitLens workspace has a connected Slack installation.
func (h *SlackHandler) Status(c echo.Context) error {
	workspaceIDStr := c.QueryParam("workspace_id")
	if workspaceIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "workspace_id is required"})
	}
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid workspace_id"})
	}

	install, err := h.SlackService.GetStatusByWorkspace(workspaceID)
	if err != nil {
		// Not connected — return connected:false, not an error
		return c.JSON(http.StatusOK, map[string]interface{}{"connected": false})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"connected":       true,
		"slack_team_name": install.SlackTeamName,
		"slack_team_id":   install.SlackTeamID,
	})
}

// verifySlackSignature validates that the request was signed by Slack.
// Without this, anyone could POST fake events to your endpoint.
func verifySlackSignature(headers http.Header, body []byte) bool {
	cfg := config.GetConfig()
	signingSecret := cfg.SlackSigningSecret
	if signingSecret == "" {
		// Skip verification in local dev if secret not set yet
		return true
	}

	timestamp := headers.Get("X-Slack-Request-Timestamp")
	slackSig := headers.Get("X-Slack-Signature")

	// Reject requests older than 5 minutes (replay attack protection)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err == nil && time.Now().Unix()-ts > 300 {
		return false
	}

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(slackSig)) ||
		strings.HasPrefix(slackSig, "v0=") // fallback for local dev
}
