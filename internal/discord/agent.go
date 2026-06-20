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
			ContextScope:     defaultAgentMessageContext(message),
			Text:             message.Text,
			RawText:          message.RawContent,
		})
		if err != nil {
			return MessageResponse{}, err
		}
		if strings.TrimSpace(response.Text) == "" {
			return fallback.HandleMessage(ctx, message)
		}
		return MessageResponse{Content: appendAgentRunHint(response.Text, response)}, nil
	})
}

func defaultAgentMessageContext(message Message) string {
	if message.Surface == MessageSurfaceGuildMention {
		return "channel-auto"
	}
	return ""
}

func AgentHandlerAdapter(handler agent.Handler) AgentRuntime {
	if handler == nil {
		return nil
	}
	return agent.Runtime{Handlers: []agent.Handler{handler}}
}

func appendAgentRunHint(content string, response agent.Response) string {
	content = strings.TrimSpace(content)
	if strings.TrimSpace(response.RunID) == "" {
		return content
	}
	switch response.RunStatus {
	case agent.RunStatusConfirmationRequired:
		if strings.TrimSpace(response.ConfirmationID) != "" {
			return content + "\n\nPending action: `/agent pending run:" + safeInline(response.RunID) + "` then `/agent confirm run:" + safeInline(response.RunID) + "` or `/agent reject run:" + safeInline(response.RunID) + "`."
		}
		return content + "\n\nRun: `" + safeInline(response.RunID) + "`."
	case agent.RunStatusCanceled:
		return content + "\n\nRun: `" + safeInline(response.RunID) + "` canceled."
	default:
		if strings.TrimSpace(response.RunID) != "" && response.RunStatus == agent.RunStatusRunning {
			return content + "\n\nRun: `" + safeInline(response.RunID) + "`. Cancel with `/agent cancel run:" + safeInline(response.RunID) + "`."
		}
		return content
	}
}
