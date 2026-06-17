package discord

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/assistant"
)

type AssistantResponder interface {
	Reply(ctx context.Context, message assistant.Message) (assistant.Response, error)
}

func AssistantFallbackHandler(responder AssistantResponder, fallback MessageHandler) MessageHandler {
	if fallback == nil {
		fallback = CoreMessageHandler()
	}
	return MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		if responder == nil {
			return fallback.HandleMessage(ctx, message)
		}
		response, err := responder.Reply(ctx, assistant.Message{
			Surface:     assistant.Surface(message.Surface),
			GuildID:     message.GuildID,
			ChannelID:   message.ChannelID,
			ActorUserID: message.UserID,
			Text:        message.Text,
		})
		if err != nil {
			return MessageResponse{}, err
		}
		if strings.TrimSpace(response.Text) == "" {
			return fallback.HandleMessage(ctx, message)
		}
		return MessageResponse{Content: response.Text}, nil
	})
}
