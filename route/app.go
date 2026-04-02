package route

import (
	"core/config"
	"core/domain"
	"core/handler"
	"core/queue"
	"core/service"
	"log"

	"github.com/hibiken/asynq"
)

type AppModel struct {
	Health           handler.HealthHandler
	User             handler.UserHandler
	Auth             handler.AuthHandler
	Role             handler.RoleHandler
	Workspace        handler.WorkspaceHandler
	Channel          handler.ChannelHandler
	ManageWorkspace  handler.ManageWorkspaceHandler
	ManageChannels   handler.ManageChannelsHandler
	Credentials      handler.CredentialsHandler
	ConnectOrg       handler.ConnectOrgHandler
	GitHubRepository handler.GitHubRepositoryHandler
}

func App() AppModel {
	// Initialize queue client
	cfg := config.GetConfig()
	redisAddr := cfg.RedisAddr
	if redisAddr == "" {
		redisAddr = "localhost:6379" // default
	}
	redisPassword := cfg.RedisPassword
	queueClient := queue.NewClient(redisAddr, redisPassword)
	log.Printf("[queue] Client connected to Redis at %s", redisAddr)

	//domain
	healthDomain := &domain.HealthDomainCtx{}
	authDomain := &domain.AuthDomainCtx{}
	userDomain := &domain.UserDomainCtx{}
	roleDomain := &domain.RoleDomainCtx{}
	workspaceDomain := &domain.WorkspaceDomainCtx{}
	channelDomain := &domain.ChannelDomainCtx{}
	manageWorkspaceDomain := &domain.ManageWorkspaceDomainCtx{}
	manageChannelsDomain := &domain.ManageChannelsDomainCtx{}
	credentialsDomain := &domain.CredentialsDomainCtx{}
	connectOrgDomain := &domain.ConnectOrgDomainCtx{}
	gitHubCommitsDomain := &domain.GitHubCommitsDomainCtx{}
	gitHubRepositoryDomain := &domain.GitHubRepositoryDomainCtx{}
	gitHubCommitFilesDomain := &domain.GitHubCommitFilesDomainCtx{}
	commitFileEmbeddingDomain := &domain.CommitFileEmbeddingDomainCtx{}
	gitHubInstallationsDomain := &domain.GitHubInstallationsDomainCtx{}
	aiDomain := &domain.AiDomainCtx{}

	//service
	healthService := service.HealthService{
		HealthDomain: healthDomain,
	}
	userService := service.UserService{
		UserDomain: userDomain,
	}
	authService := service.AuthService{
		AuthDomain: authDomain,
		UserDomain: userDomain,
	}
	roleService := service.RoleService{
		RoleDomain: roleDomain,
	}
	workspaceService := service.WorkspaceService{
		WorkspaceDomain:           workspaceDomain,
		ManageWorkspaceDomain:     manageWorkspaceDomain,
		ChannelDomain:             channelDomain,
		CredentialsDomain:         credentialsDomain,
		ManageChannelsDomain:      manageChannelsDomain,
		UserDomain:                userDomain,
		GitHubRepositoryDomain:    gitHubRepositoryDomain,
		GitHubInstallationsDomain: gitHubInstallationsDomain,
		GitHubCommitsDomain:       gitHubCommitsDomain,
		GitHubCommitFilesDomain:   gitHubCommitFilesDomain,
	}
	channelService := service.ChannelService{
		ChannelDomain:         channelDomain,
		ManageChannelsDomain:  manageChannelsDomain,
		UserDomain:            userDomain,
		ManageWorkspaceDomain: manageWorkspaceDomain,
	}

	manageWorkspaceService := service.ManageWorkspaceService{
		ManageWorkspaceDomain: manageWorkspaceDomain,
	}
	manageChannelsService := service.ManageChannelsService{
		ManageChannelsDomain: manageChannelsDomain,
	}
	credentialsService := service.CredentialsService{
		CredentialsDomain: credentialsDomain,
	}
	connectOrgService := service.ConnectOrgService{
		ConnectOrgDomain:          connectOrgDomain,
		GitHubCommitsDomain:       gitHubCommitsDomain,
		GitHubRepositoryDomain:    gitHubRepositoryDomain,
		GitHubCommitFilesDomain:   gitHubCommitFilesDomain,
		CommitFileEmbeddingDomain: commitFileEmbeddingDomain,
		QueueClient:               queueClient,
	}

	// Start the asynq worker server (processes enqueued tasks in background)
	mux := asynq.NewServeMux()
	RegisterTaskHandlers(mux, &connectOrgService)
	queue.StartWorker(redisAddr, redisPassword, mux)

	gitHubRepositoryService := service.GitHubRepositoryService{
		GitHubRepositoryDomain:    gitHubRepositoryDomain,
		GitHubCommitsDomain:       gitHubCommitsDomain,
		GitHubCommitFilesDomain:   gitHubCommitFilesDomain,
		CommitFileEmbeddingDomain: commitFileEmbeddingDomain,
		QueueClient:               queueClient,
		AiDomain:                  aiDomain,
	}

	//handler
	healthHandler := handler.HealthHandler{
		HealthService: healthService,
	}
	userHandler := handler.UserHandler{
		UserService: userService,
	}
	authHandler := handler.AuthHandler{
		AuthService: authService,
	}
	roleHandler := handler.RoleHandler{
		RoleService: roleService,
	}

	workspaceHandler := handler.WorkspaceHandler{
		WorkspaceService: workspaceService,
	}
	channelHandler := handler.ChannelHandler{
		ChannelService: channelService,
	}

	manageWorkspaceHandler := handler.ManageWorkspaceHandler{
		ManageWorkspaceService: manageWorkspaceService,
	}
	manageChannelsHandler := handler.ManageChannelsHandler{
		ManageChannelsService: manageChannelsService,
	}
	credentialsHandler := handler.CredentialsHandler{
		CredentialsService: credentialsService,
	}
	connectOrgHandler := handler.ConnectOrgHandler{
		ConnectOrgService: connectOrgService,
	}
	gitHubRepositoryHandler := handler.GitHubRepositoryHandler{
		GitHubRepositoryService: gitHubRepositoryService,
	}

	return AppModel{
		Health:           healthHandler,
		User:             userHandler,
		Auth:             authHandler,
		Role:             roleHandler,
		Workspace:        workspaceHandler,
		Channel:          channelHandler,
		ManageWorkspace:  manageWorkspaceHandler,
		ManageChannels:   manageChannelsHandler,
		Credentials:      credentialsHandler,
		ConnectOrg:       connectOrgHandler,
		GitHubRepository: gitHubRepositoryHandler,
	}
}
