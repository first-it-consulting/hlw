package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Model represents an AI model available from an endpoint.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Type        string `json:"type,omitempty"`

	// Context window, under the two names OpenAI-compatible servers use for
	// it. vLLM and oMLX report max_model_len; others report context_length.
	MaxModelLen   int `json:"max_model_len,omitempty"`
	ContextLength int `json:"context_length,omitempty"`
}

// Find returns the model with the given id.
func Find(models []Model, id string) (Model, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// ContextWindow returns the model's context window in tokens, or 0 when the
// endpoint does not report one.
func (m Model) ContextWindow() int {
	if m.MaxModelLen > 0 {
		return m.MaxModelLen
	}
	return m.ContextLength
}

// ModelList represents a list of models from an API response.
type ModelList struct {
	Models []Model `json:"data"`
}

// ollamaTagsResponse is the response format from Ollama's /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a model from Ollama's API.
type OllamaModel struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// DefaultFetchTimeout is how long a launch waits for the model list.
const DefaultFetchTimeout = 10 * time.Second

// FetchModels retrieves the list of available models from the given URL.
func FetchModels(url string, headers map[string]string) (*ModelList, error) {
	return FetchModelsTimeout(url, headers, DefaultFetchTimeout)
}

// FetchModelsTimeout is FetchModels with an explicit timeout, for callers like
// shell completion that must give up quickly rather than stall the prompt.
func FetchModelsTimeout(url string, headers map[string]string, timeout time.Duration) (*ModelList, error) {
	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to %s: %w", url, err)
	}

	// Add custom headers (e.g., Authorization)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try OpenAI/Anthropic format first: {"data": [...]}
	var modelList ModelList
	if err := json.Unmarshal(body, &modelList); err == nil && len(modelList.Models) > 0 {
		return &modelList, nil
	}

	// Try Ollama format: {"models": [...]}
	var ollamaResp ollamaTagsResponse
	if err := json.Unmarshal(body, &ollamaResp); err == nil && len(ollamaResp.Models) > 0 {
		models := make([]Model, 0, len(ollamaResp.Models))
		for _, m := range ollamaResp.Models {
			models = append(models, Model{
				ID:   m.Name,
				Name: m.Name,
			})
		}
		return &ModelList{Models: models}, nil
	}

	return nil, fmt.Errorf("failed to parse model list from response")
}
