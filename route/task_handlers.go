package route

import (
	"context"
	"core/models"
	"core/queue"
	"core/service"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
)

// RegisterTaskHandlers registers all asynq task handlers on the given mux
func RegisterTaskHandlers(mux *asynq.ServeMux, svc *service.ConnectOrgService) {
	mux.HandleFunc(queue.TypeFetchAndStoreRepos, handleFetchAndStoreRepos(svc))
	mux.HandleFunc(queue.TypeEmbedCommitFile, handleEmbedCommitFile(svc))
	mux.HandleFunc(queue.TypeEmbedCommitFileV2, handleEmbedCommitFileV2(svc))
	mux.HandleFunc(queue.TypeHandlePushEvent, handlePushEvent(svc))
	mux.HandleFunc(queue.TypeHandleInstallationRepositoriesEvent, handleInstallationRepositoriesEvent(svc))
}

// --- Task Handlers ---

func handleFetchAndStoreRepos(svc *service.ConnectOrgService) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.FetchAndStoreReposPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal FetchAndStoreRepos payload: %w", err)
		}

		log.Printf("[queue] Processing FetchAndStoreRepos: installation=%d user=%d workspace=%d",
			p.InstallationID, p.UserID, p.WorkspaceID)

		svc.FetchAndStoreRepositoryData(p.InstallationID, p.UserID, p.WorkspaceID)
		return nil
	}
}

func handleEmbedCommitFile(svc *service.ConnectOrgService) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.EmbedCommitFilePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal EmbedCommitFile payload: %w", err)
		}

		log.Printf("[queue] Processing EmbedCommitFile: file=%s commit=%s", p.Filename, p.CommitSHA)

		repo := models.GitHubRepository{
			FullName:       p.RepoFullName,
			GithubRepoID:   p.RepoGithubID,
			InstallationID: p.RepoInstallID,
		}
		commit := models.GitHubCommits{
			ID:               p.CommitID,
			CommitSHA:        p.CommitSHA,
			CommitMessage:    p.CommitMessage,
			GithubAuthorName: p.AuthorName,
		}
		file := models.GitHubCommitFiles{
			ID:       p.FileID,
			Filename: p.Filename,
			Patch:    p.Patch,
		}

		return svc.EmbedCommitFile(ctx, repo, commit, file)
	}
}

func handleEmbedCommitFileV2(svc *service.ConnectOrgService) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.EmbedCommitFileV2Payload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal EmbedCommitFileV2 payload: %w", err)
		}

		log.Printf("[queue] Processing EmbedCommitFileV2: file=%s repo=%s", p.Filename, p.FullName)

		param := models.EmbedCommitFile2{
			CommitFileId:   p.CommitFileID,
			GitHubRepoId:   p.GitHubRepoID,
			InstallationID: p.InstallationID,
			GithubCommitID: p.GithubCommitID,
			FullName:       p.FullName,
			Filename:       p.Filename,
			Author:         p.Author,
			Message:        p.Message,
			Patch:          p.Patch,
		}

		return svc.EmbedCommitFile2(param)
	}
}

func handlePushEvent(svc *service.ConnectOrgService) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.HandlePushEventPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal HandlePushEvent payload: %w", err)
		}

		var payload models.GitHubPushEvent
		if err := json.Unmarshal(p.RawJSON, &payload); err != nil {
			return fmt.Errorf("unmarshal GitHubPushEvent: %w", err)
		}

		log.Printf("[queue] Processing HandlePushEvent: repo=%s installation=%d commits=%d",
			payload.Repository.FullName, payload.Installation.ID, len(payload.Commits))

		return svc.HandlePushEvent(payload)
	}
}

func handleInstallationRepositoriesEvent(svc *service.ConnectOrgService) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.HandleInstallationRepositoriesPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal HandleInstallationRepositoriesEvent payload: %w", err)
		}

		var payload models.GitHubInstallationRepositoriesEvent
		if err := json.Unmarshal(p.RawJSON, &payload); err != nil {
			return fmt.Errorf("unmarshal GitHubInstallationRepositoriesEvent: %w", err)
		}

		log.Printf("[queue] Processing HandleInstallationRepositoriesEvent: installation=%d action=%s added=%d removed=%d",
			payload.Installation.ID, payload.Action,
			len(payload.RepositoriesAdded), len(payload.RepositoriesRemoved))

		return svc.HandleInstallationRepositoriesEvent(payload)
	}
}
