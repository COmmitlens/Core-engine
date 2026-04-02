package handler

import (
	"context"
	"core/config"
	"core/handler/validation"
	"core/models"
	"core/service"
	"core/utils"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type AuthHandler struct {
	AuthService service.AuthService
}

func (authHandler *AuthHandler) RegisterUser(c echo.Context) error {
	var err error
	param := new(models.RegisterUserRequest)
	param.Role = "customer"
	param.IsActive = false

	err = c.Bind(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	err = validation.RegisterUser(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}

	data, err := authHandler.AuthService.RegisterUser(param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})

	}
	resp := models.BasicResp{
		Message: utils.Success,
		Data:    data,
	}

	return c.JSON(http.StatusOK, resp)
}

func (authHandler *AuthHandler) ResendOTP(c echo.Context) error {
	var err error
	param := new(models.ResendOTPRequest)

	err = c.Bind(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	err = validation.ResendOTP(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}

	err = authHandler.AuthService.ResendOTP(param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})

	}
	resp := models.BasicResp{
		Message: utils.Success,
		Data:    "OTP sent successfully",
	}

	return c.JSON(http.StatusOK, resp)
}

func (authHandler *AuthHandler) VerifyOTP(c echo.Context) error {
	var err error
	param := new(models.VerifyOTPRequest)

	err = c.Bind(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	err = validation.VerifyOTP(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}

	err = authHandler.AuthService.VerifyOTP(*param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})

	}
	resp := models.BasicResp{
		Message: utils.Success,
		Data:    "OTP verified successfully",
	}

	return c.JSON(http.StatusOK, resp)
}

func (authHandler *AuthHandler) LoginUser(c echo.Context) error {
	var err error
	param := new(models.LogInRequest)
	err = c.Bind(param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	data, err := authHandler.AuthService.LoginUser(*param)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: err.Error()})
	}
	cookie := &http.Cookie{
		Name:     "Bearer",
		Value:    data.Token,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		SameSite: http.SameSiteNoneMode,
	}
	c.SetCookie(cookie)

	// Don't send token in response body when using HTTP-only cookies
	resp := models.BasicResp{
		Message: utils.Success,
		Data: map[string]interface{}{
			"email":   data.Email,
			"name":    data.Name,
			"role":    data.Role,
			"user_id": data.ID,
		},
	}
	return c.JSON(http.StatusOK, resp)
}

func (authHandler *AuthHandler) UserLogOut(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "Bearer",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}

	c.SetCookie(cookie)
	return c.JSON(http.StatusOK, models.BasicResp{
		Message: utils.Success,
		Data:    "Logged out successfully",
	})
}

// ValidateSession - Check if user's session is valid
func (authHandler *AuthHandler) ValidateSession(c echo.Context) error {
	// If middleware passed, user is authenticated
	// Extract user data set by middleware
	userData := map[string]interface{}{
		"id":       c.Get("id"),
		"email":    c.Get("email"),
		"name":     c.Get("name"),
		"role":     c.Get("role"),
		"language": c.Get("language"),
	}

	resp := models.BasicResp{
		Message: utils.Success,
		Data:    userData,
	}
	return c.JSON(http.StatusOK, resp)
}

func googleOAuthConfig() *oauth2.Config {
	cfg := config.GetConfig()
	return &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GoogleAuthURL returns the Google OAuth consent page URL.
func (authHandler *AuthHandler) GoogleAuthURL(c echo.Context) error {
	state := generateState()
	c.SetCookie(&http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   300,
		SameSite: http.SameSiteLaxMode,
	})
	url := googleOAuthConfig().AuthCodeURL(state)
	return c.JSON(http.StatusOK, models.BasicResp{
		Message: utils.Success,
		Data:    map[string]string{"url": url},
	})
}

// GoogleOAuthCallback handles the redirect from Google, upserts the user, and sets the JWT cookie.
func (authHandler *AuthHandler) GoogleOAuthCallback(c echo.Context) error {
	// Validate CSRF state
	state := c.QueryParam("state")
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie.Value != state || state == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "invalid oauth state"})
	}
	c.SetCookie(&http.Cookie{Name: "oauth_state", Value: "", MaxAge: -1, Path: "/"})

	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "code is required"})
	}

	oauthToken, err := googleOAuthConfig().Exchange(context.Background(), code)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to exchange code: " + err.Error()})
	}

	client := googleOAuthConfig().Client(context.Background(), oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to fetch user info"})
	}
	defer resp.Body.Close()

	var googleUser struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to parse user info"})
	}

	data, err := authHandler.AuthService.GoogleLogin(googleUser.Email, googleUser.Name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}

	c.SetCookie(&http.Cookie{
		Name:     "Bearer",
		Value:    data.Token,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteNoneMode,
	})

	frontendURL := config.GetConfig().FrontendUrl
	return c.Redirect(http.StatusTemporaryRedirect, frontendURL+data.Redirect)
}

func githubAuthOAuthConfig() *oauth2.Config {
	cfg := config.GetConfig()
	return &oauth2.Config{
		ClientID:     cfg.GithubAuthClientID,
		ClientSecret: cfg.GithubAuthClientSecret,
		RedirectURL:  cfg.GithubAuthRedirectURL,
		Scopes:       []string{"user:email", "read:user"},
		Endpoint:     github.Endpoint,
	}
}

// GithubAuthURL returns the GitHub OAuth consent page URL.
func (authHandler *AuthHandler) GithubAuthURL(c echo.Context) error {
	state := generateState()
	c.SetCookie(&http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   300,
		SameSite: http.SameSiteLaxMode,
	})
	url := githubAuthOAuthConfig().AuthCodeURL(state)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

// GithubAuthCallback handles the redirect from GitHub, upserts the user, and sets the JWT cookie.
func (authHandler *AuthHandler) GithubAuthCallback(c echo.Context) error {
	state := c.QueryParam("state")
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie.Value != state || state == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "invalid oauth state"})
	}
	c.SetCookie(&http.Cookie{Name: "oauth_state", Value: "", MaxAge: -1, Path: "/"})

	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "code is required"})
	}

	oauthToken, err := githubAuthOAuthConfig().Exchange(context.Background(), code)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to exchange code: " + err.Error()})
	}

	client := githubAuthOAuthConfig().Client(context.Background(), oauthToken)

	// Fetch GitHub user profile
	userResp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to fetch user info"})
	}
	defer userResp.Body.Close()

	var githubUser struct {
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to parse user info"})
	}

	// GitHub may not expose email in user profile — fetch from emails endpoint
	if githubUser.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to fetch user email"})
		}
		defer emailResp.Body.Close()

		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		if err := json.NewDecoder(emailResp.Body).Decode(&emails); err != nil {
			return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: "failed to parse user email"})
		}
		for _, e := range emails {
			if e.Primary {
				githubUser.Email = e.Email
				break
			}
		}
	}

	if githubUser.Email == "" {
		return c.JSON(http.StatusBadRequest, models.BasicResp{Message: "no email found on GitHub account"})
	}

	// Use login (username) as display name if name is empty
	if githubUser.Name == "" {
		githubUser.Name = githubUser.Login
	}

	data, err := authHandler.AuthService.GithubLogin(githubUser.Email, githubUser.Name, githubUser.Login)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}

	c.SetCookie(&http.Cookie{
		Name:     "Bearer",
		Value:    data.Token,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteNoneMode,
	})

	frontendURL := config.GetConfig().FrontendUrl
	return c.Redirect(http.StatusTemporaryRedirect, frontendURL+data.Redirect)
}

func (authHandler *AuthHandler) GithubOAuthCallback(c echo.Context) error {
	installationID := c.QueryParam("installation_id")
	setupAction := c.QueryParam("setup_action")

	if installationID == "" {
		return c.JSON(400, models.BasicResp{Message: "installation_id is required"})
	}

	// Get user ID from JWT token (set by middleware)
	userID := c.Get("user_id") // Adjust based on your JWT middleware implementation
	fmt.Println("User ID from token:", userID)
	fmt.Println("Installation ID:", installationID)
	fmt.Println("Setup Action:", setupAction)
	// Store installation in DB via service
	// err := connectOrgHandler.ConnectOrgService.StoreInstallation(userID, installationID, setupAction)
	// if err != nil {
	// 	return c.JSON(500, models.BasicResp{Message: "Failed to store installation: " + err.Error()})
	// }

	resp := models.BasicResp{
		Message: "Installation successful",
		Data:    map[string]string{"installation_id": installationID}}
	return c.JSON(200, resp)
}
