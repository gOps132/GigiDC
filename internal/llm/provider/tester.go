package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TestErrorCode string

const (
	TestErrorNone                TestErrorCode = ""
	TestErrorAuthFailed          TestErrorCode = "auth_failed"
	TestErrorInvalidResponse     TestErrorCode = "invalid_response"
	TestErrorProviderUnavailable TestErrorCode = "provider_unavailable"
	TestErrorRateLimited         TestErrorCode = "rate_limited"
	TestErrorRequestFailed       TestErrorCode = "request_failed"
	TestErrorSecretOpenFailed    TestErrorCode = "secret_open_failed"
	TestErrorUnsupportedProvider TestErrorCode = "unsupported_provider"
)

type TestCredentialRequest struct {
	Owner   Scope
	Label   string
	ActorID string
}

type ProviderTestRequest struct {
	ProviderID ProviderID
	APIKey     SecretValue
}

type TestCredentialResult struct {
	ProviderID ProviderID
	Label      string
	Status     TestStatus
	ErrorCode  TestErrorCode
}

type CredentialTester interface {
	TestCredential(ctx context.Context, req ProviderTestRequest) (TestCredentialResult, error)
}

type HTTPTester struct {
	Client           *http.Client
	BaseURLs         map[ProviderID]string
	AnthropicVersion string
}

func NewHTTPTester(client *http.Client) HTTPTester {
	return HTTPTester{Client: client}
}

func (t HTTPTester) TestCredential(ctx context.Context, req ProviderTestRequest) (TestCredentialResult, error) {
	providerID := ProviderID(strings.TrimSpace(string(req.ProviderID)))
	apiKey := strings.TrimSpace(req.APIKey.Raw())
	result := TestCredentialResult{ProviderID: providerID, Status: TestStatusFailed}
	if apiKey == "" {
		result.ErrorCode = TestErrorAuthFailed
		return result, nil
	}

	httpReq, err := t.request(ctx, providerID, apiKey)
	if err != nil {
		result.ErrorCode = TestErrorUnsupportedProvider
		return result, nil
	}

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		result.ErrorCode = TestErrorRequestFailed
		return result, nil
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		result.Status = TestStatusSucceeded
		result.ErrorCode = TestErrorNone
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		result.ErrorCode = TestErrorAuthFailed
	case resp.StatusCode == http.StatusTooManyRequests:
		result.ErrorCode = TestErrorRateLimited
	case resp.StatusCode >= 500:
		result.ErrorCode = TestErrorProviderUnavailable
	default:
		result.ErrorCode = TestErrorInvalidResponse
	}
	return result, nil
}

func (t HTTPTester) request(ctx context.Context, providerID ProviderID, apiKey string) (*http.Request, error) {
	switch providerID {
	case ProviderOpenAI:
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL(providerID, "https://api.openai.com")+"/v1/models", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return req, nil
	case ProviderAnthropic:
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL(providerID, "https://api.anthropic.com")+"/v1/models", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		version := strings.TrimSpace(t.AnthropicVersion)
		if version == "" {
			version = "2023-06-01"
		}
		req.Header.Set("anthropic-version", version)
		return req, nil
	case ProviderGemini:
		base := t.baseURL(providerID, "https://generativelanguage.googleapis.com")
		u, err := url.Parse(base + "/v1beta/models")
		if err != nil {
			return nil, err
		}
		query := u.Query()
		query.Set("key", apiKey)
		u.RawQuery = query.Encode()
		return http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}

func (t HTTPTester) baseURL(providerID ProviderID, fallback string) string {
	if t.BaseURLs == nil {
		return fallback
	}
	base := strings.TrimRight(strings.TrimSpace(t.BaseURLs[providerID]), "/")
	if base == "" {
		return fallback
	}
	return base
}
