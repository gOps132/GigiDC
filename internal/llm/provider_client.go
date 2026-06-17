package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

const defaultMaxOutputTokens = 1024

var ErrUnsupportedProvider = errors.New("unsupported llm provider")

type ResolvedTextRequest struct {
	Resolved        llmprovider.ResolvedModel
	Instructions    string
	Input           string
	MaxOutputTokens int
}

type ResolvedTextClient interface {
	CreateResolvedText(ctx context.Context, request ResolvedTextRequest) (TextResponse, error)
}

type HTTPProviderClient struct {
	Client           *http.Client
	BaseURLs         map[llmprovider.ProviderID]string
	AnthropicVersion string
	DefaultMaxTokens int
}

func NewHTTPProviderClient(client *http.Client) HTTPProviderClient {
	return HTTPProviderClient{Client: client}
}

func (c HTTPProviderClient) CreateResolvedText(ctx context.Context, request ResolvedTextRequest) (TextResponse, error) {
	normalized, err := normalizeResolvedTextRequest(request)
	if err != nil {
		return TextResponse{}, err
	}
	switch normalized.Resolved.ProviderID {
	case llmprovider.ProviderOpenAI:
		return c.createOpenAIText(ctx, normalized)
	case llmprovider.ProviderAnthropic:
		return c.createAnthropicText(ctx, normalized)
	case llmprovider.ProviderGemini:
		return c.createGeminiText(ctx, normalized)
	default:
		return TextResponse{}, ErrUnsupportedProvider
	}
}

func normalizeResolvedTextRequest(request ResolvedTextRequest) (ResolvedTextRequest, error) {
	request.Instructions = strings.TrimSpace(request.Instructions)
	request.Input = strings.TrimSpace(request.Input)
	request.Resolved.APIKey = strings.TrimSpace(request.Resolved.APIKey)
	request.Resolved.ModelID = strings.TrimSpace(request.Resolved.ModelID)
	if request.Resolved.APIKey == "" {
		return ResolvedTextRequest{}, fmt.Errorf("provider API key is required")
	}
	if request.Resolved.ModelID == "" {
		return ResolvedTextRequest{}, fmt.Errorf("model ID is required")
	}
	if request.Input == "" {
		return ResolvedTextRequest{}, fmt.Errorf("text input is required")
	}
	if request.MaxOutputTokens < 0 {
		return ResolvedTextRequest{}, fmt.Errorf("max output tokens must be nonnegative")
	}
	return request, nil
}

func (c HTTPProviderClient) createOpenAIText(ctx context.Context, request ResolvedTextRequest) (TextResponse, error) {
	body := map[string]any{
		"model":             request.Resolved.ModelID,
		"input":             request.Input,
		"max_output_tokens": c.maxOutputTokens(request.MaxOutputTokens),
		"store":             false,
	}
	if request.Instructions != "" {
		body["instructions"] = request.Instructions
	}
	httpReq, err := c.newJSONRequest(ctx, http.MethodPost, c.baseURL(llmprovider.ProviderOpenAI, "https://api.openai.com")+"/v1/responses", body)
	if err != nil {
		return TextResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+request.Resolved.APIKey)

	var payload openAIResponse
	if err := c.doJSON(httpReq, llmprovider.ProviderOpenAI, &payload); err != nil {
		return TextResponse{}, err
	}
	text := strings.TrimSpace(payload.OutputText)
	if text == "" {
		text = strings.TrimSpace(payload.textFromOutput())
	}
	if text == "" {
		return TextResponse{}, fmt.Errorf("provider response contained no text")
	}
	return TextResponse{
		Text:         text,
		InputTokens:  payload.Usage.InputTokens,
		OutputTokens: payload.Usage.OutputTokens,
	}, nil
}

func (c HTTPProviderClient) createAnthropicText(ctx context.Context, request ResolvedTextRequest) (TextResponse, error) {
	body := map[string]any{
		"model":      request.Resolved.ModelID,
		"max_tokens": c.maxOutputTokens(request.MaxOutputTokens),
		"messages": []map[string]any{{
			"role":    "user",
			"content": request.Input,
		}},
	}
	if request.Instructions != "" {
		body["system"] = request.Instructions
	}
	httpReq, err := c.newJSONRequest(ctx, http.MethodPost, c.baseURL(llmprovider.ProviderAnthropic, "https://api.anthropic.com")+"/v1/messages", body)
	if err != nil {
		return TextResponse{}, err
	}
	httpReq.Header.Set("x-api-key", request.Resolved.APIKey)
	version := strings.TrimSpace(c.AnthropicVersion)
	if version == "" {
		version = "2023-06-01"
	}
	httpReq.Header.Set("anthropic-version", version)

	var payload anthropicResponse
	if err := c.doJSON(httpReq, llmprovider.ProviderAnthropic, &payload); err != nil {
		return TextResponse{}, err
	}
	text := strings.TrimSpace(payload.text())
	if text == "" {
		return TextResponse{}, fmt.Errorf("provider response contained no text")
	}
	return TextResponse{
		Text:         text,
		InputTokens:  payload.Usage.InputTokens,
		OutputTokens: payload.Usage.OutputTokens,
	}, nil
}

func (c HTTPProviderClient) createGeminiText(ctx context.Context, request ResolvedTextRequest) (TextResponse, error) {
	endpoint, err := url.Parse(c.baseURL(llmprovider.ProviderGemini, "https://generativelanguage.googleapis.com") + "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(request.Resolved.ModelID, "models/")) + ":generateContent")
	if err != nil {
		return TextResponse{}, err
	}
	query := endpoint.Query()
	query.Set("key", request.Resolved.APIKey)
	endpoint.RawQuery = query.Encode()

	body := map[string]any{
		"contents": []map[string]any{{
			"role": "user",
			"parts": []map[string]string{{
				"text": request.Input,
			}},
		}},
		"generationConfig": map[string]any{
			"maxOutputTokens": c.maxOutputTokens(request.MaxOutputTokens),
		},
	}
	if request.Instructions != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{
				"text": request.Instructions,
			}},
		}
	}
	httpReq, err := c.newJSONRequest(ctx, http.MethodPost, endpoint.String(), body)
	if err != nil {
		return TextResponse{}, err
	}

	var payload geminiResponse
	if err := c.doJSON(httpReq, llmprovider.ProviderGemini, &payload); err != nil {
		return TextResponse{}, err
	}
	text := strings.TrimSpace(payload.text())
	if text == "" {
		return TextResponse{}, fmt.Errorf("provider response contained no text")
	}
	return TextResponse{
		Text:         text,
		InputTokens:  payload.UsageMetadata.PromptTokenCount,
		OutputTokens: payload.UsageMetadata.CandidatesTokenCount,
	}, nil
}

func (c HTTPProviderClient) newJSONRequest(ctx context.Context, method string, endpoint string, body any) (*http.Request, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c HTTPProviderClient) doJSON(req *http.Request, providerID llmprovider.ProviderID, dest any) error {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s provider: %w", providerID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return ProviderHTTPError{ProviderID: providerID, StatusCode: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode %s provider response: %w", providerID, err)
	}
	return nil
}

func (c HTTPProviderClient) maxOutputTokens(requested int) int {
	if requested > 0 {
		return requested
	}
	if c.DefaultMaxTokens > 0 {
		return c.DefaultMaxTokens
	}
	return defaultMaxOutputTokens
}

func (c HTTPProviderClient) baseURL(providerID llmprovider.ProviderID, fallback string) string {
	if c.BaseURLs == nil {
		return fallback
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURLs[providerID]), "/")
	if base == "" {
		return fallback
	}
	return base
}

type ProviderHTTPError struct {
	ProviderID llmprovider.ProviderID
	StatusCode int
}

func (e ProviderHTTPError) Error() string {
	if e.ProviderID == "" {
		return fmt.Sprintf("llm provider request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("%s provider request failed with status %d", e.ProviderID, e.StatusCode)
}

type openAIResponse struct {
	OutputText string `json:"output_text"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func (r openAIResponse) textFromOutput() string {
	var parts []string
	for _, output := range r.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (r anthropicResponse) text() string {
	var parts []string
	for _, content := range r.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "\n")
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (r geminiResponse) text() string {
	var parts []string
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
