package domain

import (
	"core/config"
	"core/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SlackDomain interface {
	// OAuth: exchange code for bot token
	ExchangeOAuthCode(code string) (models.SlackOAuthResponse, error)
	// Store the installation (bot token) for a Slack team
	SaveInstallation(install models.SlackInstallation) error
	// Retrieve installation by Slack team ID
	GetInstallationByTeamID(teamID string) (models.SlackInstallation, error)
	// Retrieve installation by CommitLens workspace ID
	GetInstallationByWorkspaceID(workspaceID int64) (models.SlackInstallation, error)
}

type SlackDomainCtx struct{}

// ExchangeOAuthCode calls Slack's oauth.v2.access endpoint.
func (s *SlackDomainCtx) ExchangeOAuthCode(code string) (models.SlackOAuthResponse, error) {
	cfg := config.GetConfig()

	resp, err := http.PostForm("https://slack.com/api/oauth.v2.access",
		url.Values{
			"client_id":     {cfg.SlackClientID},
			"client_secret": {cfg.SlackClientSecret},
			"code":          {code},
			"redirect_uri":  {cfg.SlackRedirectURI},
		},
	)
	if err != nil {
		return models.SlackOAuthResponse{}, fmt.Errorf("slack oauth request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var oauthResp models.SlackOAuthResponse
	if err := json.Unmarshal(body, &oauthResp); err != nil {
		return models.SlackOAuthResponse{}, fmt.Errorf("failed to parse slack oauth response: %w", err)
	}
	if !oauthResp.OK {
		return models.SlackOAuthResponse{}, fmt.Errorf("slack oauth error: %s", oauthResp.Error)
	}
	return oauthResp, nil
}

// SaveInstallation persists the bot token and team info to the DB.
func (s *SlackDomainCtx) SaveInstallation(install models.SlackInstallation) error {
	db := config.DbManager()
	return db.Save(&install).Error
}

// GetInstallationByTeamID fetches the stored installation for a Slack team.
func (s *SlackDomainCtx) GetInstallationByTeamID(teamID string) (models.SlackInstallation, error) {
	db := config.DbManager()
	var install models.SlackInstallation
	err := db.Where("slack_team_id = ?", teamID).First(&install).Error
	return install, err
}

// GetInstallationByWorkspaceID fetches the Slack installation for a CommitLens workspace.
func (s *SlackDomainCtx) GetInstallationByWorkspaceID(workspaceID int64) (models.SlackInstallation, error) {
	db := config.DbManager()
	var install models.SlackInstallation
	err := db.Where("workspace_id = ?", workspaceID).First(&install).Error
	return install, err
}
