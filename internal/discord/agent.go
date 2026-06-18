package discord

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/agent"
)

type AgentRuntime interface {
	Run(context.Context, agent.Request) (agent.Response, error)
}

func AgentMessageHandler(runtime AgentRuntime, fallback MessageHandler) MessageHandler {
	if fallback == nil {
		fallback = CoreMessageHandler()
	}
	return MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		if runtime == nil {
			return fallback.HandleMessage(ctx, message)
		}
		response, err := runtime.Run(ctx, agent.Request{
			Surface:          agent.Surface(message.Surface),
			GuildID:          message.GuildID,
			ChannelID:        message.ChannelID,
			ActorUserID:      message.UserID,
			RoleIDs:          message.RoleIDs,
			HasAdministrator: message.HasAdministrator,
			Text:             message.Text,
			RawText:          message.RawContent,
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

func AgentHandlerAdapter(handler agent.Handler) AgentRuntime {
	if handler == nil {
		return nil
	}
	return agent.Runtime{Handlers: []agent.Handler{handler}}
}
