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

type IntentResponse struct {
	Intent string `json:"intent"`
}

func (a *AiDomainCtx) ClassifyQueryIntent(query string) (string, error) {
	// Placeholder implementation for intent classification
	aiServiceURL := config.GetConfig().AiBackendUrl + "/classify-query-intent"

	requestBody := map[string]string{
		"userQuery": query,
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

	var result IntentResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	return "intent:" + result.Intent, nil
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
