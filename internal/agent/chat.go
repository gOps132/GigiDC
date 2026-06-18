package agent

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/assistant"
)

type ChatResponder interface {
	Reply(ctx context.Context, message assistant.Message) (assistant.Response, error)
}

type ChatHandler struct {
	Responder ChatResponder
}

func (h ChatHandler) HandleAgentRequest(ctx context.Context, request Request) (Response, bool, error) {
	if h.Responder == nil {
		return Response{}, false, nil
	}
	reply, err := h.Responder.Reply(ctx, assistant.Message{
		Surface:     assistant.Surface(request.Surface),
		GuildID:     request.GuildID,
		ChannelID:   request.ChannelID,
		ActorUserID: request.ActorUserID,
		Text:        request.Text,
	})
	if err != nil {
		return Response{}, false, err
	}
	if strings.TrimSpace(reply.Text) == "" {
		return Response{}, false, nil
	}
	return Response{Text: reply.Text, Visibility: VisibilityPublic}, true, nil
}
