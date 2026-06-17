package llm

import "context"

type TextRequest struct {
	Model        string
	Instructions string
	Input        string
}

type TextResponse struct {
	Text         string
	InputTokens  int
	OutputTokens int
	RequestID    string
	ProviderID   string
	ModelID      string
}

type Client interface {
	CreateText(ctx context.Context, request TextRequest) (TextResponse, error)
	CreateEmbedding(ctx context.Context, model string, input string) ([]float32, error)
}
