package service

import (
	"bytes"
	"core/domain"
	"core/models"
	"core/queue"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type GitHubRepositoryService struct {
	GitHubRepositoryDomain    domain.GitHubRepositoryDomain
	GitHubCommitsDomain       domain.GitHubCommitsDomain
	GitHubCommitFilesDomain   domain.GitHubCommitFilesDomain
	CommitFileEmbeddingDomain domain.CommitFileEmbeddingDomain
	QueueClient               *queue.Client
}

// BackfillEmbeddings queues embedding tasks for every commit file that has no
// embedding yet.  Returns the number of tasks enqueued.
func (g *GitHubRepositoryService) BackfillEmbeddings() (int, error) {
	if g.QueueClient == nil {
		return 0, fmt.Errorf("queue client not configured")
	}

	files, err := g.GitHubCommitFilesDomain.GetUnembeddedCommitFiles()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch unembedded files: %w", err)
	}

	enqueued := 0
	for _, f := range files {
		// Apply the same extension filter used during normal ingestion
		if !isEmbeddableFile(f.Filename) {
			continue
		}
		if err := g.QueueClient.EnqueueEmbedCommitFile(queue.EmbedCommitFilePayload{
			RepoFullName:  f.FullName,
			RepoGithubID:  f.GithubRepoID,
			RepoInstallID: f.InstallationID,
			CommitID:      f.GithubCommitID,
			CommitSHA:     "",
			CommitMessage: f.Message,
			AuthorName:    f.Author,
			FileID:        f.CommitFileID,
			Filename:      f.Filename,
			Patch:         f.Patch,
		}); err != nil {
			fmt.Printf("[backfill] failed to enqueue file_id=%d: %v\n", f.CommitFileID, err)
			continue
		}
		enqueued++
	}

	fmt.Printf("[backfill] enqueued %d / %d unembedded files\n", enqueued, len(files))
	return enqueued, nil
}

func (g *GitHubRepositoryService) GetRepositoryActivity(repoID, days int64) ([]models.CommitActivity, error) {
	data, err := g.GitHubRepositoryDomain.GetRepositoryActivity(repoID, days)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (g *GitHubRepositoryService) GetCommitDetails(repoID int64, commitSHA string) (models.CommitDetailsResponse, error) {
	data, err := g.GitHubCommitsDomain.GetCommitDetails(repoID, commitSHA)
	if err != nil {
		return models.CommitDetailsResponse{}, err
	}
	return data, nil
}

func (g *GitHubRepositoryService) GetRelatedCommitFiles(commitFileID string) ([]models.RelatedCommitFileResponse, error) {
	data, err := g.CommitFileEmbeddingDomain.GetRelatedCommitFiles(commitFileID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (g *GitHubRepositoryService) ExplainCommitFileChange(param models.ExplainCommitFileChangeRequest) (models.ExplainCommitFileChangeResponse, error) {
	data, err := g.CommitFileEmbeddingDomain.GetRelatedCommitFiles(param.CommitFileID)
	if err != nil {
		return models.ExplainCommitFileChangeResponse{}, err
	}

	commitFileInt, err := strconv.ParseInt(param.CommitFileID, 10, 64)
	if err != nil {
		return models.ExplainCommitFileChangeResponse{}, fmt.Errorf("invalid commit file id: %w", err)
	}

	// Get the main commit file details
	mainCommitFile, err := g.GitHubCommitFilesDomain.GetGitHubCommitFilesByID(commitFileInt)
	if err != nil {
		return models.ExplainCommitFileChangeResponse{}, err
	}

	// Limit the number of related files to top 3 most similar
	maxRelatedFiles := 3
	if len(data) > maxRelatedFiles {
		data = data[:maxRelatedFiles]
	}

	// Build related files context
	relatedFilesContext, err := g.buildRelatedFilesContext(data)
	if err != nil {
		return models.ExplainCommitFileChangeResponse{}, err
	}

	// Build prompts
	systemPrompt := g.buildSystemPrompt()
	userPrompt := g.buildUserPrompt(mainCommitFile, relatedFilesContext, len(data), param.Question)

	// Log token estimation (rough: 1 token ≈ 4 chars)
	estimatedTokens := (len(systemPrompt) + len(userPrompt)) / 4
	fmt.Printf("Estimated tokens: %d\n", estimatedTokens)

	// TODO: Send systemPrompt and userPrompt to your LLM service

	aiResponse, err := g.callAIService(systemPrompt, userPrompt)
	if err != nil {
		return models.ExplainCommitFileChangeResponse{}, fmt.Errorf("AI service error: %w", err)
	}
	response := models.ExplainCommitFileChangeResponse{
		Summary: aiResponse,
		Reasoning: []string{
			fmt.Sprintf("Main file: %s (%s)", mainCommitFile.Filename, mainCommitFile.Status),
			fmt.Sprintf("Analyzed %d related historical changes", len(data)),
		},
		ConfidenceScore: 0.9, // Placeholder confidence score
	}

	return response, nil
}

func (g *GitHubRepositoryService) callAIService(systemPrompt, userPrompt string) (string, error) {
	const aiServiceURL = g.Config.AiBackendUrl + "/explain-commit-file-change"

	// Prepare request payload
	requestBody := map[string]string{
		"systemPrompt": systemPrompt,
		"userPrompt":   userPrompt,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP POST request
	resp, err := http.Post(aiServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call AI service: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Return the response as string (assuming the AI service returns plain text)
	return string(body), nil
}

func (g *GitHubRepositoryService) buildRelatedFilesContext(relatedFiles []models.RelatedCommitFileResponse) (string, error) {
	const (
		maxPatchChars        = 500  // Limit patch size per file
		maxTotalContextChars = 2000 // Limit total related files context
	)

	var relatedFilesContext string
	totalContextLength := 0

	for _, file := range relatedFiles {
		githubCommitFiles, err := g.GitHubCommitFilesDomain.GetGitHubCommitFilesByID(file.CommitFileID)
		if err != nil {
			continue // Skip files with errors
		}

		// Truncate patch if too long
		patch := githubCommitFiles.Patch
		if len(patch) > maxPatchChars {
			patch = patch[:maxPatchChars] + "\n... (truncated)"
		}

		fileContext := fmt.Sprintf(`
Related File %d:
- Filename: %s
- Status: %s
- Additions: %d, Deletions: %d
- Commit SHA: %s
- Author: %s
- Similarity Score: %.2f
- Patch:
%s
---
`, file.CommitFileID, githubCommitFiles.Filename, githubCommitFiles.Status,
			githubCommitFiles.Additions, githubCommitFiles.Deletions,
			file.CommitSHA, file.Author, file.Similarity, patch)

		// Check if adding this would exceed the limit
		if totalContextLength+len(fileContext) > maxTotalContextChars {
			relatedFilesContext += "\n... (additional related files omitted)"
			break
		}

		relatedFilesContext += fileContext
		totalContextLength += len(fileContext)
	}

	return relatedFilesContext, nil
}

func (g *GitHubRepositoryService) buildSystemPrompt() string {
	return `You are an expert code reviewer. Analyze commit changes and explain:
1. What changed and why
2. How it relates to historical changes
3. Potential impact and patterns

Be concise and technical.`
}

func (g *GitHubRepositoryService) buildUserPrompt(mainCommitFile models.GitHubCommitFiles, relatedFilesContext string, relatedCount int, question string) string {
	const maxMainPatchChars = 1000

	// Truncate main file patch if too long
	mainPatch := mainCommitFile.Patch
	if len(mainPatch) > maxMainPatchChars {
		mainPatch = mainPatch[:maxMainPatchChars] + "\n... (truncated)"
	}

	return fmt.Sprintf(`Analyze this commit file change:

MAIN CHANGE:
File: %s (%s)
+%d/-%d lines
Patch:
%s

RELATED CHANGES (%d similar commits):
%s

Provide a brief explanation of what changed, why, and any patterns from historical changes.

QUESTION:
%s`, mainCommitFile.Filename, mainCommitFile.Status,
		mainCommitFile.Additions, mainCommitFile.Deletions, mainPatch,
		relatedCount, relatedFilesContext, question)
}
