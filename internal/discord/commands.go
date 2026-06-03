package discord

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/capability"
)

type Command struct {
	Name               string
	Description        string
	RequiredCapability capability.Capability
	Options            []*discordgo.ApplicationCommandOption
	Handle             CommandHandler
}

type CommandHandler func(context.Context, Interaction) (CommandResponse, error)

type CommandResponse struct {
	Content   string
	Ephemeral bool
}

type CommandRouter struct {
	commands   map[string]Command
	authorizer CommandAuthorizer
}

type interactionResponder interface {
	InteractionRespond(*discordgo.Interaction, *discordgo.InteractionResponse, ...discordgo.RequestOption) error
}

type CommandAuthorizer interface {
	Check(ctx context.Context, interaction Interaction, required capability.Capability) (capability.Decision, error)
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

func (r *CommandRouter) SetAuthorizer(authorizer CommandAuthorizer) {
	r.authorizer = authorizer
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
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	commands := make([]*discordgo.ApplicationCommand, 0, len(r.commands))
	for _, name := range names {
		command := r.commands[name]
		commands = append(commands, &discordgo.ApplicationCommand{
			Name:        command.Name,
			Description: command.Description,
			Options:     cloneCommandOptions(command.Options),
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
		return respond(responder, event.Interaction, CommandResponse{Content: "Command not supported yet."})
	}

	interaction := Interaction{
		GuildID:          event.GuildID,
		ChannelID:        event.ChannelID,
		UserID:           interactionUserID(event.Interaction),
		RoleIDs:          interactionRoleIDs(event.Interaction),
		HasAdministrator: interactionHasAdministrator(event.Interaction),
		Name:             data.Name,
		Text:             "",
		Options:          interactionOptions(data.Options),
		RoleService:      interactionRoleService(responder),
	}
	if command.RequiredCapability != "" {
		if r.authorizer == nil {
			return respond(responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
		decision, err := r.authorizer.Check(ctx, interaction, command.RequiredCapability)
		if err != nil {
			return respond(responder, event.Interaction, CommandResponse{Content: "Permission check failed.", Ephemeral: true})
		}
		if !decision.Allowed {
			return respond(responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
	}

	response, err := command.Handle(ctx, Interaction{
		GuildID:          interaction.GuildID,
		ChannelID:        interaction.ChannelID,
		UserID:           interaction.UserID,
		RoleIDs:          interaction.RoleIDs,
		HasAdministrator: interaction.HasAdministrator,
		Name:             interaction.Name,
		Text:             interaction.Text,
		Options:          interaction.Options,
		RoleService:      interaction.RoleService,
	})
	if err != nil {
		return respond(responder, event.Interaction, CommandResponse{Content: "Command failed.", Ephemeral: command.RequiredCapability != ""})
	}
	if strings.TrimSpace(response.Content) == "" {
		response.Content = "ok"
	}
	return respond(responder, event.Interaction, response)
}

func respond(responder interactionResponder, interaction *discordgo.Interaction, response CommandResponse) error {
	data := &discordgo.InteractionResponseData{Content: response.Content}
	if response.Ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	return responder.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
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

func interactionRoleIDs(interaction *discordgo.Interaction) []string {
	if interaction.Member == nil {
		return nil
	}
	return append([]string(nil), interaction.Member.Roles...)
}

func interactionHasAdministrator(interaction *discordgo.Interaction) bool {
	if interaction.Member == nil {
		return false
	}
	return interaction.Member.Permissions&discordgo.PermissionAdministrator != 0
}

func interactionRoleService(responder interactionResponder) GuildRoleService {
	service, ok := responder.(GuildRoleService)
	if !ok {
		return nil
	}
	return service
}

func interactionOptions(options []*discordgo.ApplicationCommandInteractionDataOption) []InteractionOption {
	out := make([]InteractionOption, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}
		out = append(out, InteractionOption{
			Name:    option.Name,
			Type:    option.Type,
			Value:   optionValue(option),
			Options: interactionOptions(option.Options),
		})
	}
	return out
}

func optionValue(option *discordgo.ApplicationCommandInteractionDataOption) string {
	if option.Value == nil {
		return ""
	}
	value, ok := option.Value.(string)
	if ok {
		return value
	}
	return fmt.Sprint(option.Value)
}

func cloneCommandOptions(options []*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]*discordgo.ApplicationCommandOption, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}
		copy := *option
		copy.Options = cloneCommandOptions(option.Options)
		cloned = append(cloned, &copy)
	}
	return cloned
}
