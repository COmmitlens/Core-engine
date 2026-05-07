package service

import (
	"bytes"
	"context"
	"core/config"
	"core/domain"
	"core/models"
	"core/queue"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pgvector/pgvector-go"
)

type ConnectOrgService struct {
	ConnectOrgDomain          domain.ConnectOrgDomain
	GitHubCommitsDomain       domain.GitHubCommitsDomain
	GitHubRepositoryDomain    domain.GitHubRepositoryDomain
	GitHubCommitFilesDomain   domain.GitHubCommitFilesDomain
	CommitFileEmbeddingDomain domain.CommitFileEmbeddingDomain
	WorkspaceDomain           domain.WorkspaceDomain
	QueueClient               *queue.Client
}

func (c *ConnectOrgService) UpdateInstallationByUser(param models.GitHubInstallationByUserReq) (string, error) {

	params := models.GitHubInstallation{
		UserID:         param.UserID,
		IsClaimed:      param.IsClaimed,
		InstallationID: param.InstallationID,
		WorkspaceID:    param.WorkspaceID,
	}
	existingInstallation, err := c.ConnectOrgDomain.FindInstallationByInstallationID(param.InstallationID)
	if err != nil {
		return "", fmt.Errorf("installation with id %d not found", param.InstallationID)
	}

	var result string
	if existingInstallation == nil {
		result, err = c.ConnectOrgDomain.StoreInstallation(params)
	} else {
		result, err = c.ConnectOrgDomain.UpdateInstallationByUser(params)
	}

	if err != nil {
		return "", err
	}

	// Enqueue background task to fetch and store repository data
	if err := c.QueueClient.EnqueueFetchAndStoreRepos(queue.FetchAndStoreReposPayload{
		InstallationID: params.InstallationID,
		UserID:         params.UserID,
		WorkspaceID:    params.WorkspaceID,
	}); err != nil {
		fmt.Printf("Error enqueuing FetchAndStoreRepos task: %v\n", err)
	}

	return result, nil
}

func (c *ConnectOrgService) FetchAndStoreRepositoryData(installationID int64, userID int64, workspaceID int64) {
	token, err := c.GenerateInstallationToken(models.GenerateInstallationTokenReq{
		ID:     installationID,
		UserID: userID,
	})
	if err != nil {
		fmt.Printf("Error in generating token: %v\n", err)
		return
	}

	// Fetch all repos with commits
	reposData, err := c.ConnectOrgDomain.FetchAllRepositoriesWithCommits(token)
	if err != nil {
		fmt.Printf("Error fetching repositories: %v", err)
		return
	}

	// Here, you would typically store the fetched data into your database.
	// For demonstration, we are just printing the data.
	// Store the data in database
	if err := c.StoreRepositoriesAndCommits(installationID, userID, reposData); err != nil {
		fmt.Printf("Error storing repository data: %v\n", err)
		return
	}
	fmt.Printf("Storing repository data for installation %d, user %d, workspace %d\n", installationID, userID, workspaceID)

}

func (c *ConnectOrgService) StoreRepositoriesAndCommits(installationID, userID int64, data models.AllReposWithCommitsResponse) error {
	// ctx := context.Background()
	for _, repo := range data.Repositories {
		// Store repository
		repoParams := models.GitHubRepository{
			InstallationID: installationID,
			UserID:         userID,
			GithubRepoID:   repo.RepoID,
			Name:           repo.RepoName,
			FullName:       repo.RepoFullName,
			Private:        repo.Private,
		}

		repoID, err := c.GitHubRepositoryDomain.StoreRepository(repoParams)
		if err != nil {
			fmt.Printf("Error storing repository %s: %v\n", repo.RepoFullName, err)
			continue
		}
		// Update repoParams with the ID from database
		repoParams.ID = repoID
		// Store commits for this repository
		for _, commit := range repo.Commits {
			committedAt, err := time.Parse(time.RFC3339, commit.Date)
			if err != nil {
				fmt.Printf("Error parsing date for commit %s: %v\n", commit.SHA, err)
				// Use current time as fallback
				committedAt = time.Now()
			}
			commitParams := models.GitHubCommits{
				GithubRepositoryID: repoID,
				CommitSHA:          commit.SHA,
				CommitMessage:      commit.Message,
				GithubAuthorName:   commit.Author,
				AuthorEmail:        commit.AuthorEmail,
				CommittedAt:        committedAt,
			}

			commitID, err := c.GitHubCommitsDomain.StoreCommit(commitParams)
			if err != nil {
				fmt.Printf("Error storing commit %s: %v\n", commit.SHA, err)
				continue
			}

			// Update commitParams with the ID from database
			commitParams.ID = commitID

			// Store files for this commit
			for _, file := range commit.Files {
				fileParams := models.GitHubCommitFiles{
					GithubCommitID: commitID,
					Filename:       file.Filename,
					Status:         file.Status,
					Additions:      file.Additions,
					Deletions:      file.Deletions,
					Patch:          file.Patch,
				}

				fileId, err := c.GitHubCommitFilesDomain.StoreCommitFile(fileParams)
				if err != nil {
					fmt.Printf("Error storing file %s: %v\n", file.Filename, err)
					continue
				}
				// Update fileParams with the ID from database
				fileParams.ID = fileId

				// Enqueue embedding task for this file
				if ok, reason := shouldEmbedWithReason(fileParams); ok {
					fmt.Printf("[DEBUG-EMBED] Enqueuing embed for file_id=%d filename=%s\n", fileParams.ID, fileParams.Filename)
					if err := c.QueueClient.EnqueueEmbedCommitFile(queue.EmbedCommitFilePayload{
						RepoFullName:  repoParams.FullName,
						RepoGithubID:  repoParams.GithubRepoID,
						RepoInstallID: repoParams.InstallationID,
						CommitID:      commitParams.ID,
						CommitSHA:     commitParams.CommitSHA,
						CommitMessage: commitParams.CommitMessage,
						AuthorName:    commitParams.GithubAuthorName,
						FileID:        fileParams.ID,
						Filename:      fileParams.Filename,
						Patch:         fileParams.Patch,
					}); err != nil {
						fmt.Printf("[DEBUG-EMBED] Error enqueuing embed for file %s: %v\n", fileParams.Filename, err)
					}
				} else {
					fmt.Printf("[DEBUG-EMBED] SKIP embed file_id=%d filename=%s reason=%s\n", fileParams.ID, fileParams.Filename, reason)
				}
			}
		}
	}
	return nil
}

func shouldEmbed(file models.GitHubCommitFiles) bool {
	ok, _ := shouldEmbedWithReason(file)
	return ok
}

func shouldEmbedWithReason(file models.GitHubCommitFiles) (bool, string) {
	// 1. Must have a diff
	if strings.TrimSpace(file.Patch) == "" {
		return false, "empty patch"
	}

	// 2. Skip removed files
	if file.Status == "removed" {
		return false, "status=removed"
	}

	// 3. File extension filter
	if !isEmbeddableFile(file.Filename) {
		return false, fmt.Sprintf("non-embeddable extension (%s)", path.Ext(file.Filename))
	}

	// 4. Size guard (important)
	const maxPatchSize = 6000 // characters
	if len(file.Patch) > maxPatchSize {
		return false, fmt.Sprintf("patch too large (%d chars > %d)", len(file.Patch), maxPatchSize)
	}

	return true, ""
}

// isEmbeddableFile checks if file type should be embedded
func isEmbeddableFile(filename string) bool {
	ext := strings.ToLower(path.Ext(filename))

	switch ext {
	case
		".go",
		".ts",
		".tsx",
		".js",
		".jsx",
		".java",
		".py",
		".rb",
		".rs",
		".c",
		".cpp",
		".h",
		".sql",
		".md",
		".yaml",
		".yml",
		".json",
		".toml":
		return true
	default:
		return false
	}
}

// GenerateEmbedding generates vector embedding using OpenAI
// func GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
// 	endpoint := config.GetConfig().AzureEmbeddingEndpoint
// 	apiKey := config.GetConfig().AzureOpenAIKey
// 	deployment := config.GetConfig().AzureOpenAIModel

// 	client, err := azopenai.NewClient(
// 		endpoint,
// 		azopenai.KeyCredential{
// 			Key: apiKey,
// 		},
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := client.GetEmbeddings(
// 		ctx,
// 		azopenai.EmbeddingsOptions{
// 			DeploymentName: deployment,
// 			Input:          []string{text},
// 		},
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(resp.Data) == 0 {
// 		return nil, fmt.Errorf("no embeddings returned")
// 	}

// 	return resp.Data[0].Embedding, nil
// }

func GenerateEmbeddingRaw(ctx context.Context, text string) ([]float32, error) {
	endpoint := os.Getenv("AZURE_EMBEDDING_ENDPOINT")
	deployment := os.Getenv("AZURE_EMBEDDING_DEPLOYMENT")
	apiKey := os.Getenv("AZURE_EMBEDDING_API_KEY")
	apiVersion := os.Getenv("AZURE_EMBEDDING_API_VERSION")

	url := fmt.Sprintf(
		"%sopenai/deployments/%s/embeddings?api-version=%s",
		endpoint,
		deployment,
		apiVersion,
	)

	body := map[string]any{
		"input": text,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure openai error %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

// func GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
// 	vec := make([]float32, 1536)
// 	for i := range vec {
// 		vec[i] = float32((i%97)-48) / 100.0
// 	}
// 	return vec, nil
// }

// EmbedCommitFile generates and stores embedding for a commit file
func (c *ConnectOrgService) EmbedCommitFile(
	ctx context.Context,
	repo models.GitHubRepository,
	commit models.GitHubCommits,
	file models.GitHubCommitFiles,
) error {
	// Build embedding text
	text := fmt.Sprintf(
		"Repository: %s\nFile: %s\nAuthor: %s\nCommit message: %s\n\nDiff:\n%s",
		repo.FullName,
		file.Filename,
		commit.GithubAuthorName,
		commit.CommitMessage,
		file.Patch,
	)

	// Generate embedding
	embedding, err := GenerateEmbeddingRaw(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store in pgvector table
	embeddingRow := models.CommitFileEmbedding{
		Embedding:        pgvector.NewVector(embedding),
		CommitFileID:     file.ID,
		GithubCommitID:   commit.ID,
		GithubRepoID:     repo.GithubRepoID,
		InstallationID:   repo.InstallationID,
		GithubAuthorName: &commit.GithubAuthorName,
		PlatformUserID:   nil, // filled later if GitHub OAuth linked
	}
	if err := c.CommitFileEmbeddingDomain.StoreEmbedding(embeddingRow); err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	fmt.Printf("✓ Embedded file: %s (commit: %s)\n", file.Filename, commit.CommitSHA[:7])
	return nil
}

func (c *ConnectOrgService) EmbedCommitFile2(param models.EmbedCommitFile2) error {
	ctx := context.Background()
	text := fmt.Sprintf(
		"Repository: %s\nFile: %s\nAuthor: %s\nCommit message: %s\n\nDiff:\n%s",
		param.FullName,
		param.Filename,
		param.Author,
		param.Message,
		param.Patch,
	)

	// Generate embedding
	embedding, err := GenerateEmbeddingRaw(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store in pgvector table
	embeddingRow := models.CommitFileEmbedding{
		Embedding:        pgvector.NewVector(embedding),
		CommitFileID:     param.CommitFileId,
		GithubCommitID:   param.GithubCommitID,
		GithubRepoID:     param.GitHubRepoId,
		InstallationID:   param.InstallationID,
		GithubAuthorName: &param.Author,
		PlatformUserID:   nil, // filled later if GitHub OAuth linked
	}
	if err := c.CommitFileEmbeddingDomain.StoreEmbedding(embeddingRow); err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	fmt.Printf("✓ Embedded file: %s (repo: %s)\n", param.Filename, param.FullName)
	return nil
}

func (c *ConnectOrgService) RedirectToOrgAuth(payload models.JWTPayload) (string, error) {
	// Security: verify the requesting user is the owner of the workspace they are connecting
	workspace, err := c.WorkspaceDomain.GetById(models.Workspace{ID: payload.WorkspaceID})
	if err != nil {
		return "", fmt.Errorf("workspace not found")
	}
	if workspace.OwnerID != payload.ID {
		return "", fmt.Errorf("only the workspace owner can connect a GitHub organization")
	}

	token, err := GenerateJWT(payload)
	if err != nil {
		return "", err
	}

	redirectURL := fmt.Sprintf("https://github.com/apps/office-aiii/installations/new?state=%s", token)

	return redirectURL, nil
}

// VerifyInstallationBelongsToApp uses the GitHub App JWT to confirm the given
// installation_id is a real installation of OUR app. This prevents an attacker
// from submitting a stolen or fabricated installation_id.
func (c *ConnectOrgService) VerifyInstallationBelongsToApp(installationID int64) error {
	appJWT, err := GenerateGitHubAppJWT()
	if err != nil {
		return fmt.Errorf("failed to generate app JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("installation_id %d not found — does not belong to this app", installationID)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AppID int64 `json:"app_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	expectedAppID := config.GetConfig().GitHubAppID
	if result.AppID != expectedAppID {
		return fmt.Errorf("installation app_id mismatch: got %d, expected %d", result.AppID, expectedAppID)
	}

	return nil
}

func (c *ConnectOrgService) StoreInstallation(params models.GitHubInstallation) (string, error) {
	ok, err := c.ConnectOrgDomain.StoreInstallation(params)
	if err != nil {
		return "", err
	}

	return ok, nil
}

func (c *ConnectOrgService) GenerateInstallationToken(param models.GenerateInstallationTokenReq) (string, error) {

	// installation, err := c.ConnectOrgDomain.FindInstallationByID(param.ID)
	// if err != nil || installation == nil {
	// 	return "", fmt.Errorf("installation with id %d not found", param.ID)
	// }
	installation := param.ID
	appJWT, err := GenerateGitHubAppJWT()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://api.github.com/app/installations/%d/access_tokens",
		installation,
	)

	token, err := c.ConnectOrgDomain.GenerateInstallationToken(appJWT, url)
	if err != nil {
		return "", err
	}
	return token, nil
}

func GenerateGitHubAppJWT() (string, error) {
	keyData := strings.ReplaceAll(config.GetConfig().GitHubPrivateKey, `\n`, "\n")

	block, _ := pem.Decode([]byte(keyData))
	if block == nil {
		return "", errors.New("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	now := time.Now()

	claims := jwt.MapClaims{
		"iat": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": config.GetConfig().GitHubAppID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func (c *ConnectOrgService) HandlePushEvent(params models.GitHubPushEvent) error {
	repoParam := models.GitHubRepository{
		InstallationID: params.Installation.ID,
		GithubRepoID:   params.Repository.ID,
	}

	// Try to find existing repository

	userId, err := c.GitHubRepositoryDomain.FindUserIdByInstallationID(repoParam)
	if err != nil {
		fmt.Printf("Error finding user ID for installation %d: %v\n", params.Installation.ID, err)
		// Proceed with userId as 0, will update later when installation is claimed
		userId = 0
	}

	data, err := c.GitHubRepositoryDomain.FindRepositoryByInstallationID(repoParam)
	if err != nil {
		// Repository not found, create it
		fmt.Printf("Repository not found for installation %d, creating new entry\n", params.Installation.ID)

		newRepoParams := models.GitHubRepository{
			InstallationID: params.Installation.ID,
			GithubRepoID:   params.Repository.ID,
			Name:           params.Repository.Name,
			FullName:       params.Repository.FullName,
			Private:        params.Repository.Private,
			UserID:         userId, // Will be updated when installation is claimed
		}

		repoID, err := c.GitHubRepositoryDomain.StoreRepository(newRepoParams)
		if err != nil {
			return fmt.Errorf("error creating repository for installation %d: %w", params.Installation.ID, err)
		}

		// Update the data pointer with the newly created repository
		newRepoParams.ID = repoID
		data = newRepoParams

		fmt.Printf("✓ Created repository: %s (ID: %d)\n", params.Repository.FullName, repoID)
	}

	// Prepare commit parameters array for bulk insert
	commitParams := make([]models.GitHubCommits, 0, len(params.Commits))

	for _, commit := range params.Commits {
		committedAt, err := time.Parse(time.RFC3339, commit.Timestamp)
		if err != nil {
			fmt.Printf("Error parsing date for commit %s: %v\n", commit.ID, err)
			// Use current time as fallback
			committedAt = time.Now()
		}

		commitParam := models.GitHubCommits{
			GithubRepositoryID: data.ID,
			CommitSHA:          commit.ID,
			CommitMessage:      commit.Message,
			GithubAuthorName:   commit.Author.Name,
			AuthorEmail:        commit.Author.Email,
			CommittedAt:        committedAt,
		}

		commitParams = append(commitParams, commitParam)
	}
	commitData, err := c.GitHubCommitsDomain.StoreCommitsBulk(commitParams)
	if err != nil {
		return fmt.Errorf("error storing commits for repository %d: %w", data.ID, err)
	}

	token, err := c.GenerateInstallationToken(models.GenerateInstallationTokenReq{
		ID:     params.Installation.ID,
		UserID: data.UserID,
	})
	if err != nil {
		return fmt.Errorf("error generating installation token: %w", err)
	}

	// ctx := context.Background()

	for _, commits := range commitData {
		commitFilesData, err := c.ConnectOrgDomain.FetchCommitDetail(token, params.Repository.FullName, commits.CommitSHA)
		if err != nil {
			fmt.Printf("Error fetching commit details for %s: %v\n", commits.CommitSHA, err)
			continue
		}

		// Store files for this commit
		for _, file := range commitFilesData.Files {
			fileParams := models.GitHubCommitFiles{
				GithubCommitID: commits.ID,
				Filename:       file.Filename,
				Status:         file.Status,
				Additions:      file.Additions,
				Deletions:      file.Deletions,
				Patch:          file.Patch,
			}

			fileId, err := c.GitHubCommitFilesDomain.StoreCommitFile(fileParams)
			if err != nil {
				fmt.Printf("Error storing file %s: %v\n", file.Filename, err)
				continue
			}
			// Update fileParams with the ID from database
			fileParams.ID = fileId

			// Enqueue embedding task for this file
			if ok, reason := shouldEmbedWithReason(fileParams); ok {
				fmt.Printf("[DEBUG-EMBED] Enqueuing embed for file_id=%d filename=%s\n", fileId, file.Filename)
				if err := c.QueueClient.EnqueueEmbedCommitFileV2(queue.EmbedCommitFileV2Payload{
					CommitFileID:   fileId,
					GitHubRepoID:   data.GithubRepoID,
					InstallationID: data.InstallationID,
					GithubCommitID: commits.ID,
					FullName:       params.Repository.FullName,
					Filename:       file.Filename,
					Author:         commits.GithubAuthorName,
					Message:        commits.CommitMessage,
					Patch:          file.Patch,
				}); err != nil {
					fmt.Printf("[DEBUG-EMBED] Error enqueuing embed for file %s: %v\n", file.Filename, err)
				}
			} else {
				fmt.Printf("[DEBUG-EMBED] SKIP embed file_id=%d filename=%s reason=%s\n", fileId, file.Filename, reason)
			}
		}
	}

	return nil
}
