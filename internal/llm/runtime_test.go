package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestRuntimeGeneratesTextAndRecordsUsage(t *testing.T) {
	resolver := &fakeModelResolver{resolved: validResolvedModel()}
	client := &fakeResolvedTextClient{response: TextResponse{Text: "hello back", InputTokens: 12, OutputTokens: 34}}
	usage := &fakeUsageRecorder{}
	runtime := Runtime{
		Resolver:     resolver,
		Client:       client,
		Usage:        usage,
		NewRequestID: func() string { return "request-id" },
	}

	got, err := runtime.GenerateText(context.Background(), validGenerateTextRequest())
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
	if got.Text != "hello back" || got.InputTokens != 12 || got.OutputTokens != 34 || got.RequestID != "request-id" || got.ProviderID != "openai" || got.ModelID != "model-id" {
		t.Fatalf("response = %+v, want generated text", got)
	}
	if resolver.req.Owner.GuildID != "guild-id" || resolver.req.Purpose != llmprovider.PurposeChat || resolver.req.ActorID != "actor-id" {
		t.Fatalf("resolve req = %+v, want active guild chat model", resolver.req)
	}
	if client.req.Resolved.ProviderID != llmprovider.ProviderOpenAI || client.req.Input != "hello" || client.req.Instructions != "be kind" {
		t.Fatalf("client req = %+v, want resolved text request", client.req)
	}
	want := llmprovider.UsageEvent{
		RequestID:        "request-id",
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		ActorUserID:      "actor-id",
		BillingOwnerType: llmprovider.OwnerGuild,
		BillingOwnerID:   "guild-id",
		ProviderID:       llmprovider.ProviderOpenAI,
		ModelID:          "model-id",
		Purpose:          llmprovider.PurposeChat,
		InputTokens:      12,
		OutputTokens:     34,
		Status:           llmprovider.UsageStatusSucceeded,
	}
	if usage.event != want {
		t.Fatalf("usage event = %+v, want %+v", usage.event, want)
	}
}

func TestRuntimeRecordsProviderFailure(t *testing.T) {
	providerErr := ProviderHTTPError{ProviderID: llmprovider.ProviderOpenAI, StatusCode: 429}
	resolver := &fakeModelResolver{resolved: validResolvedModel()}
	client := &fakeResolvedTextClient{err: providerErr}
	usage := &fakeUsageRecorder{}
	runtime := Runtime{
		Resolver:     resolver,
		Client:       client,
		Usage:        usage,
		NewRequestID: func() string { return "request-id" },
	}

	_, err := runtime.GenerateText(context.Background(), validGenerateTextRequest())
	if err == nil {
		t.Fatal("expected provider error")
	}
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %v, want provider error", err)
	}
	if usage.event.Status != llmprovider.UsageStatusFailed || usage.event.ErrorClass != "provider_rate_limited" {
		t.Fatalf("usage event = %+v, want failed rate-limited usage", usage.event)
	}
	if usage.event.InputTokens != 0 || usage.event.OutputTokens != 0 {
		t.Fatalf("usage event = %+v, want zero tokens on failure", usage.event)
	}
}

func TestRuntimeReturnsUsageFailureAfterSuccessfulProviderCall(t *testing.T) {
	runtime := Runtime{
		Resolver:     &fakeModelResolver{resolved: validResolvedModel()},
		Client:       &fakeResolvedTextClient{response: TextResponse{Text: "hello back", InputTokens: 1, OutputTokens: 2}},
		Usage:        &fakeUsageRecorder{err: errors.New("usage db down")},
		NewRequestID: func() string { return "request-id" },
	}

	_, err := runtime.GenerateText(context.Background(), validGenerateTextRequest())
	if err == nil || !strings.Contains(err.Error(), "usage db down") {
		t.Fatalf("error = %v, want usage failure", err)
	}
}

func TestRuntimeDoesNotRecordUsageWhenResolutionFails(t *testing.T) {
	usage := &fakeUsageRecorder{}
	req := validGenerateTextRequest()
	req.RequestID = "request-id"
	runtime := Runtime{
		Resolver: &fakeModelResolver{err: errors.New("no model")},
		Client:   &fakeResolvedTextClient{},
		Usage:    usage,
	}

	_, err := runtime.GenerateText(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "no model") {
		t.Fatalf("error = %v, want resolution failure", err)
	}
	if usage.calls != 0 {
		t.Fatalf("usage calls = %d, want none before resolution", usage.calls)
	}
}

func TestRuntimeValidatesGenerateTextRequest(t *testing.T) {
	tests := []struct {
		name string
		req  GenerateTextRequest
		want string
	}{
		{name: "missing actor", req: generateTextRequestWithActor(" "), want: "actor user ID is required"},
		{name: "bad purpose", req: generateTextRequestWithPurpose("bad"), want: "unknown purpose"},
		{name: "missing input", req: generateTextRequestWithInput(" "), want: "text input is required"},
		{name: "negative max", req: generateTextRequestWithMax(-1), want: "max output tokens must be nonnegative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := Runtime{Resolver: &fakeModelResolver{}, Client: &fakeResolvedTextClient{}}
			_, err := runtime.GenerateText(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRuntimeRequiresRequestIDWhenRecordingUsage(t *testing.T) {
	runtime := Runtime{
		Resolver: &fakeModelResolver{resolved: validResolvedModel()},
		Client:   &fakeResolvedTextClient{},
		Usage:    &fakeUsageRecorder{},
	}

	_, err := runtime.GenerateText(context.Background(), validGenerateTextRequest())
	if err == nil || !strings.Contains(err.Error(), "request ID is required") {
		t.Fatalf("error = %v, want request ID requirement", err)
	}
}

func validGenerateTextRequest() GenerateTextRequest {
	return GenerateTextRequest{
		Owner:        llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: "guild-id"},
		Purpose:      llmprovider.PurposeChat,
		ActorUserID:  "actor-id",
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		Instructions: "be kind",
		Input:        "hello",
	}
}

func generateTextRequestWithActor(actorID string) GenerateTextRequest {
	req := validGenerateTextRequest()
	req.ActorUserID = actorID
	return req
}

func generateTextRequestWithPurpose(purpose llmprovider.Purpose) GenerateTextRequest {
	req := validGenerateTextRequest()
	req.Purpose = purpose
	return req
}

func generateTextRequestWithInput(input string) GenerateTextRequest {
	req := validGenerateTextRequest()
	req.Input = input
	return req
}

func generateTextRequestWithMax(max int) GenerateTextRequest {
	req := validGenerateTextRequest()
	req.MaxOutputTokens = max
	return req
}

func validResolvedModel() llmprovider.ResolvedModel {
	return llmprovider.ResolvedModel{
		Owner:            llmprovider.Scope{OwnerType: llmprovider.OwnerGuild, GuildID: "guild-id"},
		Purpose:          llmprovider.PurposeChat,
		ProviderID:       llmprovider.ProviderOpenAI,
		ModelID:          "model-id",
		CredentialID:     "credential-id",
		CredentialLabel:  "main",
		APIKey:           "sk-test",
		BillingOwnerType: llmprovider.OwnerGuild,
		BillingOwnerID:   "guild-id",
	}
}

type fakeModelResolver struct {
	req      llmprovider.ResolveModelRequest
	resolved llmprovider.ResolvedModel
	err      error
}

func (r *fakeModelResolver) ResolveActiveModel(_ context.Context, req llmprovider.ResolveModelRequest) (llmprovider.ResolvedModel, error) {
	r.req = req
	return r.resolved, r.err
}

type fakeResolvedTextClient struct {
	req      ResolvedTextRequest
	response TextResponse
	err      error
}

func (c *fakeResolvedTextClient) CreateResolvedText(_ context.Context, req ResolvedTextRequest) (TextResponse, error) {
	c.req = req
	return c.response, c.err
}

type fakeUsageRecorder struct {
	calls int
	event llmprovider.UsageEvent
	err   error
}

func (r *fakeUsageRecorder) RecordUsage(_ context.Context, event llmprovider.UsageEvent) error {
	r.calls++
	r.event = event
	return r.err
}
