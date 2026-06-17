package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPTesterCallsOpenAIModelsEndpoint(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	tester := HTTPTester{Client: server.Client(), BaseURLs: map[ProviderID]string{ProviderOpenAI: server.URL}}
	got, err := tester.TestCredential(context.Background(), ProviderTestRequest{ProviderID: ProviderOpenAI, APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("TestCredential returned error: %v", err)
	}
	if got.Status != TestStatusSucceeded || got.ErrorCode != TestErrorNone {
		t.Fatalf("result = %+v, want succeeded", got)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want bearer key", gotAuth)
	}
}

func TestHTTPTesterCallsAnthropicModelsEndpoint(t *testing.T) {
	var gotKey, gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	tester := HTTPTester{Client: server.Client(), BaseURLs: map[ProviderID]string{ProviderAnthropic: server.URL}}
	got, err := tester.TestCredential(context.Background(), ProviderTestRequest{ProviderID: ProviderAnthropic, APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("TestCredential returned error: %v", err)
	}
	if got.Status != TestStatusSucceeded || gotKey != "sk-test" || gotVersion != "2023-06-01" {
		t.Fatalf("result/key/version = %+v/%q/%q, want anthropic test", got, gotKey, gotVersion)
	}
}

func TestHTTPTesterCallsGeminiModelsEndpoint(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		if r.URL.Path != "/v1beta/models" {
			t.Fatalf("path = %q, want /v1beta/models", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	tester := HTTPTester{Client: server.Client(), BaseURLs: map[ProviderID]string{ProviderGemini: server.URL}}
	got, err := tester.TestCredential(context.Background(), ProviderTestRequest{ProviderID: ProviderGemini, APIKey: "gemini-key"})
	if err != nil {
		t.Fatalf("TestCredential returned error: %v", err)
	}
	if got.Status != TestStatusSucceeded || gotKey != "gemini-key" {
		t.Fatalf("result/key = %+v/%q, want gemini test", got, gotKey)
	}
}

func TestHTTPTesterMapsStatusesToSanitizedErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want TestErrorCode
	}{
		{name: "auth", code: http.StatusUnauthorized, want: TestErrorAuthFailed},
		{name: "rate limit", code: http.StatusTooManyRequests, want: TestErrorRateLimited},
		{name: "server", code: http.StatusBadGateway, want: TestErrorProviderUnavailable},
		{name: "bad response", code: http.StatusBadRequest, want: TestErrorInvalidResponse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "secret sk-test must not escape", tt.code)
			}))
			defer server.Close()
			tester := HTTPTester{Client: server.Client(), BaseURLs: map[ProviderID]string{ProviderOpenAI: server.URL}}

			got, err := tester.TestCredential(context.Background(), ProviderTestRequest{ProviderID: ProviderOpenAI, APIKey: "sk-test"})
			if err != nil {
				t.Fatalf("TestCredential returned error: %v", err)
			}
			if got.Status != TestStatusFailed || got.ErrorCode != tt.want {
				t.Fatalf("result = %+v, want failed %q", got, tt.want)
			}
			if strings.Contains(string(got.ErrorCode), "sk-test") {
				t.Fatalf("error code leaked secret: %+v", got)
			}
		})
	}
}
