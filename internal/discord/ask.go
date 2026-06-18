package discord

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
)

func AskCommand(runtime AgentRuntime) Command {
	return Command{
		Name:        "ask",
		Description: "Ask Gigi through the agent runtime.",
		Options: []*discordgo.ApplicationCommandOption{
			stringOption("question", "Question for Gigi.", nil),
			optionalStringOption("context", "Context scope.", []*discordgo.ApplicationCommandOptionChoice{
				{Name: "none", Value: "none"},
				{Name: "channel", Value: "channel"},
			}),
			optionalStringOption("visibility", "Response visibility.", []*discordgo.ApplicationCommandOptionChoice{
				{Name: "public", Value: "public"},
				{Name: "private", Value: "private"},
			}),
		},
		Handle: askHandler(runtime),
	}
}

func askHandler(runtime AgentRuntime) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if runtime == nil {
			return CommandResponse{Content: "Ask is not configured yet.", Ephemeral: true}, nil
		}
		question := optionByName(interaction.Options, "question")
		if question == "" {
			return CommandResponse{Content: "Question is required.", Ephemeral: true}, nil
		}
		contextScope := normalizeAskContext(optionByName(interaction.Options, "context"))
		visibility := normalizeAskVisibility(optionByName(interaction.Options, "visibility"))
		response, err := runtime.Run(ctx, agent.Request{
			Surface:          agent.SurfaceGuildMention,
			GuildID:          interaction.GuildID,
			ChannelID:        interaction.ChannelID,
			ActorUserID:      interaction.UserID,
			RoleIDs:          interaction.RoleIDs,
			HasAdministrator: interaction.HasAdministrator,
			ContextScope:     contextScope,
			Text:             question,
			RawText:          question,
		})
		if err != nil {
			return CommandResponse{Content: "Ask failed.", Ephemeral: visibility == "private"}, nil
		}
		content := strings.TrimSpace(response.Text)
		if content == "" {
			content = "I could not answer that."
		}
		return CommandResponse{Content: content, Ephemeral: visibility == "private"}, nil
	}
}

func normalizeAskContext(value string) string {
	switch strings.TrimSpace(value) {
	case "", "none":
		return "none"
	case "channel":
		return "channel"
	default:
		return "none"
	}
}

func normalizeAskVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "", "public":
		return "public"
	case "private":
		return "private"
	default:
		return "public"
	}
}
