package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
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

// StatusError is returned when the endpoint answered, but not with 200.
type StatusError struct {
	URL  string
	Code int
	Body string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("unexpected status code %d from %s", e.Code, e.URL)
	}
	return fmt.Sprintf("unexpected status code %d from %s: %s", e.Code, e.URL, e.Body)
}

// Unreachable reports whether err means nothing answered at all — connection
// refused, DNS failure, timeout — as opposed to an endpoint that answered with
// an error or with something hlw could not parse.
func Unreachable(err error) bool {
	if err == nil {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return false
	}
	var ue *neturl.Error
	return errors.As(err, &ue)
}

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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &StatusError{URL: url, Code: resp.StatusCode, Body: strings.TrimSpace(string(body))}
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
