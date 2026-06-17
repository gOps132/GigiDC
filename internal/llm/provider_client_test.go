package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestHTTPProviderClientCallsOpenAIResponsesAPI(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			t.Fatalf("request = %s %s, want POST /v1/responses", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"output_text": "hello from openai",
			"usage": {"input_tokens": 12, "output_tokens": 34}
		}`))
	}))
	defer server.Close()

	client := HTTPProviderClient{Client: server.Client(), BaseURLs: map[llmprovider.ProviderID]string{llmprovider.ProviderOpenAI: server.URL}}
	got, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderOpenAI))
	if err != nil {
		t.Fatalf("CreateResolvedText returned error: %v", err)
	}
	if gotAuth != "Bearer sk-test-secret" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotBody["model"] != "model-id" || gotBody["input"] != "hello" || gotBody["instructions"] != "be kind" || gotBody["store"] != false || gotBody["max_output_tokens"] != float64(defaultMaxOutputTokens) {
		t.Fatalf("body = %+v, want OpenAI response body", gotBody)
	}
	if got.Text != "hello from openai" || got.InputTokens != 12 || got.OutputTokens != 34 {
		t.Fatalf("response = %+v, want parsed text and usage", got)
	}
}

func TestHTTPProviderClientParsesOpenAIOutputArrayFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"output": [{
				"type": "message",
				"content": [
					{"type": "output_text", "text": "one"},
					{"type": "output_text", "text": "two"}
				]
			}],
			"usage": {"input_tokens": 1, "output_tokens": 2}
		}`))
	}))
	defer server.Close()

	client := HTTPProviderClient{Client: server.Client(), BaseURLs: map[llmprovider.ProviderID]string{llmprovider.ProviderOpenAI: server.URL}}
	got, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderOpenAI))
	if err != nil {
		t.Fatalf("CreateResolvedText returned error: %v", err)
	}
	if got.Text != "one\ntwo" || got.InputTokens != 1 || got.OutputTokens != 2 {
		t.Fatalf("response = %+v, want output array text", got)
	}
}

func TestHTTPProviderClientCallsAnthropicMessagesAPI(t *testing.T) {
	var gotKey, gotVersion string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Fatalf("request = %s %s, want POST /v1/messages", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"content": [{"type": "text", "text": "hello from claude"}],
			"usage": {"input_tokens": 7, "output_tokens": 8}
		}`))
	}))
	defer server.Close()

	client := HTTPProviderClient{Client: server.Client(), BaseURLs: map[llmprovider.ProviderID]string{llmprovider.ProviderAnthropic: server.URL}}
	got, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderAnthropic))
	if err != nil {
		t.Fatalf("CreateResolvedText returned error: %v", err)
	}
	if gotKey != "sk-test-secret" || gotVersion != "2023-06-01" {
		t.Fatalf("key/version = %q/%q, want Anthropic auth headers", gotKey, gotVersion)
	}
	messages, ok := gotBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %+v, want one user message", gotBody["messages"])
	}
	if gotBody["model"] != "model-id" || gotBody["system"] != "be kind" || gotBody["max_tokens"] != float64(defaultMaxOutputTokens) {
		t.Fatalf("body = %+v, want Anthropic body", gotBody)
	}
	if got.Text != "hello from claude" || got.InputTokens != 7 || got.OutputTokens != 8 {
		t.Fatalf("response = %+v, want parsed text and usage", got)
	}
}

func TestHTTPProviderClientCallsGeminiGenerateContentAPI(t *testing.T) {
	var gotKey string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/model-id:generateContent" {
			t.Fatalf("request = %s %s, want POST /v1beta/models/model-id:generateContent", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"content": {"parts": [{"text": "hello from gemini"}]}
			}],
			"usageMetadata": {"promptTokenCount": 3, "candidatesTokenCount": 4, "totalTokenCount": 7}
		}`))
	}))
	defer server.Close()

	client := HTTPProviderClient{Client: server.Client(), BaseURLs: map[llmprovider.ProviderID]string{llmprovider.ProviderGemini: server.URL}}
	got, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderGemini))
	if err != nil {
		t.Fatalf("CreateResolvedText returned error: %v", err)
	}
	if gotKey != "sk-test-secret" {
		t.Fatalf("key = %q, want Gemini key query", gotKey)
	}
	generationConfig, ok := gotBody["generationConfig"].(map[string]any)
	if !ok || generationConfig["maxOutputTokens"] != float64(defaultMaxOutputTokens) || gotBody["systemInstruction"] == nil || gotBody["contents"] == nil {
		t.Fatalf("body = %+v, want Gemini content and system instruction", gotBody)
	}
	if got.Text != "hello from gemini" || got.InputTokens != 3 || got.OutputTokens != 4 {
		t.Fatalf("response = %+v, want parsed text and usage", got)
	}
}

func TestHTTPProviderClientSanitizesProviderHTTPFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key sk-test-secret and prompt hello", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := HTTPProviderClient{Client: server.Client(), BaseURLs: map[llmprovider.ProviderID]string{llmprovider.ProviderOpenAI: server.URL}}
	_, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderOpenAI))
	if err == nil {
		t.Fatal("expected provider HTTP error")
	}
	if !strings.Contains(err.Error(), "openai provider request failed with status 401") {
		t.Fatalf("error = %q, want sanitized provider status", err.Error())
	}
	if strings.Contains(err.Error(), "sk-test-secret") || strings.Contains(err.Error(), "hello") {
		t.Fatalf("error leaked secret or prompt: %q", err.Error())
	}
}

func TestHTTPProviderClientRejectsUnsupportedProvider(t *testing.T) {
	client := HTTPProviderClient{}
	_, err := client.CreateResolvedText(context.Background(), validResolvedTextRequest(llmprovider.ProviderCustom))
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("error = %v, want ErrUnsupportedProvider", err)
	}
}

func TestHTTPProviderClientValidatesResolvedTextRequest(t *testing.T) {
	tests := []struct {
		name string
		req  ResolvedTextRequest
		want string
	}{
		{name: "missing api key", req: resolvedTextRequestWith("", "model-id", "hello"), want: "provider API key is required"},
		{name: "missing model", req: resolvedTextRequestWith("sk-test", "", "hello"), want: "model ID is required"},
		{name: "missing input", req: resolvedTextRequestWith("sk-test", "model-id", " "), want: "text input is required"},
		{name: "negative max", req: ResolvedTextRequest{Resolved: llmprovider.ResolvedModel{ProviderID: llmprovider.ProviderOpenAI, APIKey: "sk-test", ModelID: "model-id"}, Input: "hello", MaxOutputTokens: -1}, want: "max output tokens must be nonnegative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := HTTPProviderClient{}
			_, err := client.CreateResolvedText(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func validResolvedTextRequest(providerID llmprovider.ProviderID) ResolvedTextRequest {
	return resolvedTextRequestWithProvider(providerID, "sk-test-secret", "model-id", "hello")
}

func resolvedTextRequestWith(apiKey string, modelID string, input string) ResolvedTextRequest {
	return resolvedTextRequestWithProvider(llmprovider.ProviderOpenAI, apiKey, modelID, input)
}

func resolvedTextRequestWithProvider(providerID llmprovider.ProviderID, apiKey string, modelID string, input string) ResolvedTextRequest {
	return ResolvedTextRequest{
		Resolved: llmprovider.ResolvedModel{
			ProviderID: providerID,
			ModelID:    modelID,
			APIKey:     apiKey,
		},
		Instructions: "be kind",
		Input:        input,
	}
}
