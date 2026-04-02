package handler

import (
	"core/config"
	"core/models"
	"core/service"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo"
)

type ConnectOrgHandler struct {
	ConnectOrgService service.ConnectOrgService
}

func (connectOrgHandler *ConnectOrgHandler) CreateConnectOrg(c echo.Context) error {
	param := models.CreateChannelReqs{}
	if err := c.Bind(&param); err != nil {
		return c.JSON(400, models.BasicResp{Message: err.Error()})
	}

	resp := models.BasicResp{
		Message: "success",
		Data:    nil}
	return c.JSON(200, resp)
}

func (connectOrgHandler *ConnectOrgHandler) RedirectToOrgAuth(c echo.Context) error {
	userId := c.Get("id").(int64)
	email := c.Get("email").(string)
	name := c.Get("name").(string)
	role := c.Get("role").(string)
	language := c.Get("language").(string)
	now := time.Now()
	workspace_id := c.QueryParam("workspace_id")
	if workspace_id == "" {
		return c.JSON(400, models.BasicResp{Message: "workspace_id parameter is required"})
	}
	workspaceIDInt, err := strconv.ParseInt(workspace_id, 10, 64)
	if err != nil {
		return c.JSON(400, models.BasicResp{Message: "Invalid workspace_id"})
	}

	payload := models.JWTPayload{
		ID:          userId,
		Email:       email,
		Name:        name,
		Role:        role,
		Language:    language,
		WorkspaceID: workspaceIDInt,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(time.Hour * 72).Unix(),
		},
	}
	redirectURL, err := connectOrgHandler.ConnectOrgService.RedirectToOrgAuth(payload)
	if err != nil {
		return c.JSON(500, models.BasicResp{Message: err.Error()})
	}
	// return c.Redirect(302, redirectURL)
	return c.JSON(200, map[string]string{
		"url": redirectURL,
	})
}

func (connectOrgHandler *ConnectOrgHandler) HandleOrgCallback(c echo.Context) error {
	installationID := c.QueryParam("installation_id")
	state := c.QueryParam("state")
	claims, err := DecodeJwt(state)
	if err != nil {
		return c.JSON(400, models.BasicResp{Message: "Invalid state parameter"})
	}
	userIDFloat, ok := claims["id"].(float64)
	if !ok {
		return c.JSON(400, models.BasicResp{Message: "Invalid user ID in state parameter"})
	}
	workspaceIDFloat, ok := claims["workspace_id"].(float64)
	if !ok {
		return c.JSON(400, models.BasicResp{Message: "Invalid workspace ID in state parameter"})
	}
	workspaceID := int64(workspaceIDFloat)
	userID := int64(userIDFloat)

	installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return c.JSON(400, models.BasicResp{Message: "Invalid installation ID"})
	}
	param := models.GitHubInstallationByUserReq{
		UserID:         userID,
		IsClaimed:      true,
		InstallationID: installationIDInt,
		WorkspaceID:    workspaceID,
	}

	_, err = connectOrgHandler.ConnectOrgService.UpdateInstallationByUser(param)
	if err != nil {
		return c.JSON(500, models.BasicResp{Message: err.Error()})
	}

	return c.Redirect(302, config.GetConfig().FrontendUrl+"/dashboard")
}

func DecodeJwt(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GetConfig().JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func verifyGitHubSignature(body []byte, sigHeader, secret string) bool {
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

func (h *ConnectOrgHandler) HandleWebhook(c echo.Context) error {

	event := c.Request().Header.Get("X-GitHub-Event")

	// ✅ Read body ONCE
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(400, map[string]string{"error": "cannot read body"})
	}

	// ✅ Verify GitHub webhook signature
	sig := c.Request().Header.Get("X-Hub-Signature-256")
	secret := config.GetConfig().GithubWebhookSecret
	if secret != "" && !verifyGitHubSignature(bodyBytes, sig, secret) {
		log.Println("❌ Invalid webhook signature")
		return c.JSON(401, map[string]string{"error": "invalid signature"})
	}

	switch event {

	case "ping":
		return c.JSON(200, map[string]string{"message": "pong"})

	case "installation":
		var payload models.GitHubInstallationEvent
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid installation payload"})
		}

		if payload.Action != "created" {
			return c.JSON(200, map[string]string{"status": "ignored"})
		}

		existing, err :=
			h.ConnectOrgService.ConnectOrgDomain.
				FindInstallationByInstallationID(payload.Installation.ID)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		if existing == nil {
			log.Println("existing")
			params := models.GitHubInstallation{
				InstallationID: payload.Installation.ID,
				AccountLogin:   payload.Installation.Account.Login,
				AccountType:    payload.Installation.Account.Type,
				UserID:         0,
				IsClaimed:      false,
				WorkspaceID:    0,
			}

			if _, err := h.ConnectOrgService.ConnectOrgDomain.StoreInstallation(params); err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}

			return c.JSON(200, map[string]string{"status": "created"})
		}

		return c.JSON(200, map[string]string{"status": "already_exists"})

	case "push":
		var payload models.GitHubPushEvent
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid push payload"})
		}

		log.Println("✅ PUSH EVENT RECEIVED")
		log.Println("📦 Installation ID:", payload.Installation.ID)
		log.Println("📦 Repo ID:", payload.Repository.ID)
		log.Println("📦 Commits:", len(payload.Commits))

		// Enqueue push event for background processing via asynq
		if err := h.ConnectOrgService.QueueClient.EnqueueHandlePushEvent(bodyBytes); err != nil {
			log.Printf("❌ Error enqueuing push event: %v\n", err)
			return c.JSON(500, map[string]string{"error": "failed to enqueue push event"})
		}

		return c.JSON(200, map[string]string{"status": "push_received"})

	default:
		return c.JSON(200, map[string]string{"status": "ignored"})
	}
}

func (h *ConnectOrgHandler) GenerateInstallationToken(c echo.Context) error {
	userId := c.Get("id").(int64)
	param := models.GenerateInstallationTokenReq{}
	param.UserID = userId
	if err := c.Bind(&param); err != nil {
		return c.JSON(400, models.BasicResp{Message: err.Error()})
	}
	token, err := h.ConnectOrgService.GenerateInstallationToken(param)
	if err != nil {
		return c.JSON(500, models.BasicResp{Message: err.Error()})
	}
	resp := models.BasicResp{
		Message: "success",
		Data:    map[string]string{"token": token},
	}
	return c.JSON(200, resp)
}
