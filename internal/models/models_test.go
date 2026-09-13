package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchModels_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := ModelList{
			Models: []Model{
				{ID: "claude-3-opus", Name: "Claude 3 Opus", DisplayName: "Claude 3 Opus (Preview)"},
				{ID: "claude-3-sonnet", Name: "Claude 3 Sonnet", DisplayName: "Claude 3 Sonnet"},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	modelList, err := FetchModels(server.URL, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(modelList.Models) != 2 {
		t.Errorf("Expected 2 models, got: %d", len(modelList.Models))
	}

	if modelList.Models[0].ID != "claude-3-opus" {
		t.Errorf("Expected first model 'claude-3-opus', got: %s", modelList.Models[0].ID)
	}
}

func TestFetchModels_InvalidURL(t *testing.T) {
	_, err := FetchModels("http://invalid-url-that-does-not-exist.example.com/models", nil)
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

func TestFetchModels_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal Server Error"}`))
	}))
	defer server.Close()

	_, err := FetchModels(server.URL, nil)
	if err == nil {
		t.Fatal("Expected error for server error")
	}
}

func TestFetchModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	_, err := FetchModels(server.URL, nil)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}
