package domain

import (
	"bytes"
	"core/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AiDomain interface {
	ClassifyQueryIntent(query string) (string, error)
	CallAzureChatCompletion(systemPrompt, userPrompt string) (string, error)
}

type AiDomainCtx struct{}

func (a *AiDomainCtx) ClassifyQueryIntent(query string) (string, error) {
	// Placeholder implementation. Replace with actual AI model inference.
	// For example, you could integrate with OpenAI's API here.
	return "get_commits_by_author_and_date", nil
	// return `The intent of the query is to seek an explanation for a specific code change. The user likely wants to understand the rationale behind a code modification, the context in which it was made, and its implications on the overall codebase. This intent suggests that the user is looking for insights into why a particular change was implemented, what problem it addresses, and how it affects the functionality or performance of the software.
}

func (g *AiDomainCtx) CallAzureChatCompletion(systemPrompt, userPrompt string) (string, error) {
	aiServiceURL := config.GetConfig().AiBackendUrl + "/explain-commit-file-change"

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
