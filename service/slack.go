package service

import (
	"bytes"
	"core/domain"
	"core/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type SlackService struct {
	SlackDomain             domain.SlackDomain
	AiDomain                domain.AiDomain
	GitHubRepositoryService *GitHubRepositoryService
}

// CompleteOAuth exchanges the OAuth code for a bot token and stores it
// linked to the given CommitLens workspaceID.
func (s *SlackService) CompleteOAuth(code string, workspaceID int64) error {
	oauthResp, err := s.SlackDomain.ExchangeOAuthCode(code)
	if err != nil {
		return fmt.Errorf("oauth exchange failed: %w", err)
	}

	install := models.SlackInstallation{
		WorkspaceID:   workspaceID, // ← links to CommitLens workspace
		SlackTeamID:   oauthResp.Team.ID,
		SlackTeamName: oauthResp.Team.Name,
		BotToken:      oauthResp.AccessToken,
	}

	if err := s.SlackDomain.SaveInstallation(install); err != nil {
		return fmt.Errorf("failed to save slack installation: %w", err)
	}

	log.Printf("[slack] Installed for team %s (%s) → workspace %d",
		oauthResp.Team.Name, oauthResp.Team.ID, workspaceID)
	return nil
}

// GetStatusByWorkspace returns the Slack installation for a CommitLens workspace.
func (s *SlackService) GetStatusByWorkspace(workspaceID int64) (models.SlackInstallation, error) {
	return s.SlackDomain.GetInstallationByWorkspaceID(workspaceID)
}

// HandleMessage processes an incoming Slack message event.
// Called in a goroutine — do not write to the echo context here.
func (s *SlackService) HandleMessage(event models.SlackMessageEvent) {
	// log.Printf("[slack] Message from user=%s channel=%s text=%q", event.UserID, event.ChannelID, event.Text)

	// // Step 1: Only process questions
	// if !isQuestion(event.Text) {
	// 	return
	// }

	// // Step 2: Find the CommitLens workspace linked to this Slack team
	// install, err := s.SlackDomain.GetInstallationByTeamID(event.TeamID)
	// if err != nil {
	// 	log.Printf("[slack] No installation found for team %s: %v", event.TeamID, err)
	// 	return
	// }

	// // Step 3: Query commit embeddings using existing AI pipeline
	// if s.GitHubRepositoryService == nil {
	// 	return
	// }
	// result, err := s.GitHubRepositoryService.QueryWorkspace(
	// 	models.WorkspaceQueryRequest{Query: event.Text},
	// 	install.WorkspaceID,
	// )
	// if err != nil || result.Answer == "" {
	// 	log.Printf("[slack] No answer found for query: %s", event.Text)
	// 	return
	// }

	// // Step 4: Reply in the Slack thread
	// if err := s.replyInThread(install.BotToken, event.ChannelID, event.Timestamp, result.Answer); err != nil {
	// 	log.Printf("[slack] Failed to reply in thread: %v", err)
	// }

	log.Printf("[slack] Message from user=%s channel=%s text=%q", event.UserID, event.ChannelID, event.Text)

	// Find the bot token for this Slack team
	install, err := s.SlackDomain.GetInstallationByTeamID(event.TeamID)
	if err != nil {
		log.Printf("[slack] No installation found for team %s: %v", event.TeamID, err)
		return
	}

	// Simple test reply
	if err := s.replyInThread(install.BotToken, event.ChannelID, event.Timestamp, "CommitLens reply 👋"); err != nil {
		log.Printf("[slack] Failed to reply: %v", err)
	}
}

// replyInThread posts a message back to Slack in the same thread as the original message.
func (s *SlackService) replyInThread(botToken, channelID, threadTS, text string) error {
	payload := map[string]string{
		"channel":   channelID,
		"text":      text,
		"thread_ts": threadTS, // makes it a thread reply, not a new message
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack API call failed: %w", err)
	}
	defer resp.Body.Close()

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&slackResp)
	if !slackResp.OK {
		return fmt.Errorf("slack API error: %s", slackResp.Error)
	}

	log.Printf("[slack] Replied in thread %s on channel %s", threadTS, channelID)
	return nil
}

// isQuestion uses simple heuristics to detect if a message is a question.
// The AI intent classifier will handle deeper classification during QueryWorkspace.
func isQuestion(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || len(text) < 10 {
		return false
	}

	// Ends with question mark
	if strings.HasSuffix(text, "?") {
		return true
	}

	// Starts with common question words
	questionPrefixes := []string{
		"who ", "what ", "when ", "where ", "why ", "how ",
		"which ", "did ", "does ", "do ", "is ", "are ", "was ",
		"were ", "can ", "could ", "should ", "has ", "have ",
	}
	for _, prefix := range questionPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}

	return false
}
