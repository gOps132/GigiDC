package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

type ModelResolver interface {
	ResolveActiveModel(ctx context.Context, req llmprovider.ResolveModelRequest) (llmprovider.ResolvedModel, error)
}

type Runtime struct {
	Resolver     ModelResolver
	Client       ResolvedTextClient
	Usage        llmprovider.UsageRecorder
	NewRequestID func() string
}

type GenerateTextRequest struct {
	Owner           llmprovider.Scope
	Purpose         llmprovider.Purpose
	ActorUserID     string
	GuildID         string
	ChannelID       string
	RequestID       string
	Instructions    string
	Input           string
	MaxOutputTokens int
}

func (r Runtime) GenerateText(ctx context.Context, req GenerateTextRequest) (TextResponse, error) {
	if r.Resolver == nil {
		return TextResponse{}, fmt.Errorf("llm model resolver is required")
	}
	if r.Client == nil {
		return TextResponse{}, fmt.Errorf("llm text client is required")
	}
	normalized, err := r.normalizeGenerateTextRequest(req)
	if err != nil {
		return TextResponse{}, err
	}
	resolved, err := r.Resolver.ResolveActiveModel(ctx, llmprovider.ResolveModelRequest{
		Owner:   normalized.Owner,
		Purpose: normalized.Purpose,
		ActorID: normalized.ActorUserID,
	})
	if err != nil {
		return TextResponse{}, err
	}

	response, callErr := r.Client.CreateResolvedText(ctx, ResolvedTextRequest{
		Resolved:        resolved,
		Instructions:    normalized.Instructions,
		Input:           normalized.Input,
		MaxOutputTokens: normalized.MaxOutputTokens,
	})
	usageErr := r.recordUsage(ctx, normalized, resolved, response, callErr)
	if callErr != nil {
		if usageErr != nil {
			return TextResponse{}, fmt.Errorf("%w; record usage: %v", callErr, usageErr)
		}
		return TextResponse{}, callErr
	}
	if usageErr != nil {
		return TextResponse{}, usageErr
	}
	return response, nil
}

func (r Runtime) normalizeGenerateTextRequest(req GenerateTextRequest) (GenerateTextRequest, error) {
	req.ActorUserID = strings.TrimSpace(req.ActorUserID)
	req.GuildID = strings.TrimSpace(req.GuildID)
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.Instructions = strings.TrimSpace(req.Instructions)
	req.Input = strings.TrimSpace(req.Input)
	if req.ActorUserID == "" {
		return GenerateTextRequest{}, fmt.Errorf("actor user ID is required")
	}
	if err := llmprovider.ValidatePurpose(req.Purpose); err != nil {
		return GenerateTextRequest{}, err
	}
	if req.Input == "" {
		return GenerateTextRequest{}, fmt.Errorf("text input is required")
	}
	if req.MaxOutputTokens < 0 {
		return GenerateTextRequest{}, fmt.Errorf("max output tokens must be nonnegative")
	}
	if req.RequestID == "" && r.NewRequestID != nil {
		req.RequestID = strings.TrimSpace(r.NewRequestID())
	}
	if req.RequestID == "" && r.Usage != nil {
		return GenerateTextRequest{}, fmt.Errorf("request ID is required")
	}
	return req, nil
}

func (r Runtime) recordUsage(ctx context.Context, req GenerateTextRequest, resolved llmprovider.ResolvedModel, response TextResponse, callErr error) error {
	if r.Usage == nil {
		return nil
	}
	status := llmprovider.UsageStatusSucceeded
	errorClass := ""
	if callErr != nil {
		status = llmprovider.UsageStatusFailed
		errorClass = classifyUsageError(callErr)
		response = TextResponse{}
	}
	return r.Usage.RecordUsage(ctx, llmprovider.UsageEvent{
		RequestID:        req.RequestID,
		GuildID:          req.GuildID,
		ChannelID:        req.ChannelID,
		ActorUserID:      req.ActorUserID,
		BillingOwnerType: resolved.BillingOwnerType,
		BillingOwnerID:   resolved.BillingOwnerID,
		ProviderID:       resolved.ProviderID,
		ModelID:          resolved.ModelID,
		Purpose:          resolved.Purpose,
		InputTokens:      response.InputTokens,
		OutputTokens:     response.OutputTokens,
		Status:           status,
		ErrorClass:       errorClass,
	})
}

func classifyUsageError(err error) string {
	if err == nil {
		return ""
	}
	var providerErr ProviderHTTPError
	if errors.As(err, &providerErr) {
		switch providerErr.StatusCode {
		case 401, 403:
			return "provider_auth"
		case 429:
			return "provider_rate_limited"
		default:
			if providerErr.StatusCode >= 500 {
				return "provider_unavailable"
			}
			return "provider_request_failed"
		}
	}
	if errors.Is(err, ErrUnsupportedProvider) {
		return "unsupported_provider"
	}
	return "provider_request_failed"
}
