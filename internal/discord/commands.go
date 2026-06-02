package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	Name        string
	Description string
	Handle      CommandHandler
}

type CommandHandler func(context.Context, Interaction) (CommandResponse, error)

type CommandResponse struct {
	Content string
}

type CommandRouter struct {
	commands map[string]Command
}

type interactionResponder interface {
	InteractionRespond(*discordgo.Interaction, *discordgo.InteractionResponse, ...discordgo.RequestOption) error
}

func NewCommandRouter(commands ...Command) (*CommandRouter, error) {
	router := &CommandRouter{commands: make(map[string]Command, len(commands))}
	for _, command := range commands {
		name := strings.TrimSpace(command.Name)
		if name == "" {
			return nil, fmt.Errorf("command name is required")
		}
		if command.Description == "" {
			return nil, fmt.Errorf("command %q description is required", name)
		}
		if command.Handle == nil {
			return nil, fmt.Errorf("command %q handler is required", name)
		}
		if _, exists := router.commands[name]; exists {
			return nil, fmt.Errorf("duplicate command %q", name)
		}

		command.Name = name
		router.commands[name] = command
	}
	return router, nil
}

func CoreCommands() []Command {
	return []Command{
		{
			Name:        "ping",
			Description: "Check whether Gigi is online.",
			Handle: func(context.Context, Interaction) (CommandResponse, error) {
				return CommandResponse{Content: "pong"}, nil
			},
		},
	}
}

func (r *CommandRouter) ApplicationCommands() []*discordgo.ApplicationCommand {
	commands := make([]*discordgo.ApplicationCommand, 0, len(r.commands))
	for _, command := range r.commands {
		commands = append(commands, &discordgo.ApplicationCommand{
			Name:        command.Name,
			Description: command.Description,
		})
	}
	return commands
}

func (r *CommandRouter) HandleInteraction(ctx context.Context, responder interactionResponder, event *discordgo.InteractionCreate) error {
	if event == nil || event.Interaction == nil {
		return nil
	}
	if event.Type != discordgo.InteractionApplicationCommand {
		return nil
	}

	data := event.ApplicationCommandData()
	command, ok := r.commands[data.Name]
	if !ok {
		return respond(responder, event.Interaction, "Command not supported yet.")
	}

	response, err := command.Handle(ctx, Interaction{
		GuildID:   event.GuildID,
		ChannelID: event.ChannelID,
		UserID:    interactionUserID(event.Interaction),
		Name:      data.Name,
		Text:      "",
	})
	if err != nil {
		return respond(responder, event.Interaction, "Command failed.")
	}
	if strings.TrimSpace(response.Content) == "" {
		response.Content = "ok"
	}
	return respond(responder, event.Interaction, response.Content)
}

func respond(responder interactionResponder, interaction *discordgo.Interaction, content string) error {
	return responder.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func interactionUserID(interaction *discordgo.Interaction) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	if interaction.User != nil {
		return interaction.User.ID
	}
	return ""
}
