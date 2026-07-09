package domain

import (
	"bytes"
	"core/config"
	"core/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AiDomain interface {
	ClassifyQueryIntent(query string) (ClassifiedIntent, error)
	CallAzureChatCompletion(systemPrompt, userPrompt string) (string, error)
	QueryToWorkspace(param models.WorkspaceQueryRequest, workspaceID int64) (models.WorkspaceQueryResponse, error)
}

type AiDomainCtx struct{}

// ClassifiedIntent holds both the routing intent and the extracted parameters
// so the classifier does both jobs in a single LLM call.
type ClassifiedIntent struct {
	Intent   string `json:"intent"`
	Keyword  string `json:"keyword"`  // extracted for intent:search_by_keyword
	Author   string `json:"author"`   // extracted for intent:get_commits_by_author_and_date
	Filename string `json:"filename"` // extracted for intent:get_file_history
}

func (a *AiDomainCtx) ClassifyQueryIntent(query string) (ClassifiedIntent, error) {
	aiServiceURL := config.GetConfig().AiBackendUrl + "/classify-query-intent"

	requestBody := map[string]string{
		"userQuery": query,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return ClassifiedIntent{}, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := http.Post(aiServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return ClassifiedIntent{}, fmt.Errorf("failed to call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ClassifiedIntent{}, fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ClassifiedIntent{}, fmt.Errorf("failed to read AI service response: %w", err)
	}

	var result ClassifiedIntent
	if err := json.Unmarshal(body, &result); err != nil {
		return ClassifiedIntent{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	result.Intent = "intent:" + result.Intent
	return result, nil
}

func (g *AiDomainCtx) CallAzureChatCompletion(systemPrompt, userPrompt string) (string, error) {
	aiServiceURL := config.GetConfig().AiBackendUrl + "/ai/explain-commit-file-change"

	requestBody := map[string]string{
		"systemPrompt": systemPrompt,
		"userPrompt":   userPrompt,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(aiServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read AI service response: %w", err)
	}

	return string(body), nil
}

func (g *AiDomainCtx) QueryToWorkspace(param models.WorkspaceQueryRequest, workspaceID int64) (models.WorkspaceQueryResponse, error) {
	aiServiceURL := config.GetConfig().AiBackendUrl + "/ai/react-agent"

	requestBody := map[string]interface{}{
		"query":        param.Query,
		"workspace_id": fmt.Sprintf("%d", workspaceID),
		"author":       param.Author,
		"filename":     param.Filename,
		"repo_id":      param.RepoID,
		"limit":        param.Limit,
		"date_range":   param.DateRange,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return models.WorkspaceQueryResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(aiServiceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return models.WorkspaceQueryResponse{}, fmt.Errorf("failed to call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return models.WorkspaceQueryResponse{}, fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.WorkspaceQueryResponse{}, fmt.Errorf("failed to read AI service response: %w", err)
	}

	var raw struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return models.WorkspaceQueryResponse{}, fmt.Errorf("failed to parse AI service response: %w", err)
	}

	return models.WorkspaceQueryResponse{
		Answer: raw.Response,
	}, nil
}
