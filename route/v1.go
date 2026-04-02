package route

import (
	"core/middleware"

	"github.com/labstack/echo"
)

func v1Routes(g *echo.Group, h AppModel) {
	g.GET("/health", h.Health.Check)

	auth := g.Group("/auth")
	auth.POST("/register", h.Auth.RegisterUser)
	auth.POST("/resend-otp", h.Auth.ResendOTP)
	auth.POST("/verify-otp", h.Auth.VerifyOTP)
	auth.POST("/login", h.Auth.LoginUser)
	auth.GET("/validate", h.Auth.ValidateSession, middleware.JWTVerify())
	auth.GET("/logout", h.Auth.UserLogOut, middleware.JWTVerify())
	auth.GET("/github/callback", h.Auth.GithubOAuthCallback, middleware.JWTVerify())
	auth.GET("/google", h.Auth.GoogleAuthURL)
	auth.GET("/google/callback", h.Auth.GoogleOAuthCallback)

	user := g.Group("/user", middleware.JWTVerify())
	user.GET("/get-user", h.User.GetUserName)

	workspace := g.Group("/workspace", middleware.JWTVerify())
	workspace.POST("/create", h.Workspace.CreateWorkspace)
	workspace.POST("/get", h.Workspace.GetWorkspaceById)
	workspace.POST("/add_user", h.Workspace.AddUserInWorkspace)
	workspace.POST("/getall_workspace", h.Workspace.GetAllWorkspace)
	workspace.GET("/get_repo", h.Workspace.GetAllRepository)
	workspace.GET("/get_org_details", h.Workspace.GetOrgDetails)
	workspace.GET("/get_repo_commits/:repo_id", h.Workspace.GetRepoCommits)
	workspace.GET("/get_commit_details/:github_commit_id", h.Workspace.GetCommitFilesDetails)
	workspace.POST("/get_members", h.Workspace.GetWorkSpaceMembers)
	workspace.POST("/:workspace_id/query", h.GitHubRepository.QueryWorkspace)

	g.POST("/workspace/accept-invite", h.Workspace.AcceptInvite)
	g.POST("/workspace/details", h.Workspace.GetWorkspaceDetails, middleware.JWTVerify())

	channel := g.Group("/channel", middleware.JWTVerify())
	channel.POST("/create", h.Channel.CreateChannel)
	channel.POST("/add-user", h.Channel.AddUserInChannel)

	connectorg := g.Group("/connect-org")
	connectorg.POST("/create", h.ConnectOrg.CreateConnectOrg, middleware.JWTVerify())
	connectorg.GET("/get", h.ConnectOrg.RedirectToOrgAuth, middleware.JWTVerify())
	connectorg.GET("/github/setup", h.ConnectOrg.HandleOrgCallback)
	connectorg.POST("/github/webhook", h.ConnectOrg.HandleWebhook)
	connectorg.POST("/generate_installation_token", h.ConnectOrg.GenerateInstallationToken, middleware.JWTVerify())
	// connectorg.GET("/get_repo", h.ConnectOrg.FetchInstallationRepositories, middleware.JWTVerify())

	githubRepo := g.Group("/github-repository", middleware.JWTVerify())
	githubRepo.GET("/repos/:repo_id/activity", h.GitHubRepository.GetRepositoryActivity)
	githubRepo.GET("/repos/:repo_id/commits/:commit_sha", h.GitHubRepository.GetCommitDetails)
	githubRepo.GET("/commit-files/:commit_file_id/related", h.GitHubRepository.GetRelatedCommitFiles)
	githubRepo.POST("/commit-files/:commit_file_id/explain", h.GitHubRepository.ExplainCommitFileChange)
	githubRepo.POST("/backfill-embeddings", h.GitHubRepository.BackfillEmbeddings)

}
