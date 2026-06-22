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
	Name                       string
	Description                string
	RequiredCapability         capability.Capability
	RequiredCapabilityFor      func(Interaction) capability.Capability
	RequiredCapabilityForModal func(ModalInteraction) capability.Capability
	ModalCustomIDPrefixes      []string
	Options                    []*discordgo.ApplicationCommandOption
	Handle                     CommandHandler
	HandleModal                CommandModalHandler
}

type CommandHandler func(context.Context, Interaction) (CommandResponse, error)
type CommandModalHandler func(context.Context, ModalInteraction) (CommandResponse, error)

type CommandResponse struct {
	Content      string
	Ephemeral    bool
	Embeds       []*discordgo.MessageEmbed
	Modal        *ModalResponse
	Deferred     bool
	AfterRespond func(context.Context, InteractionResponseEditor)
}

type ModalResponse struct {
	CustomID   string
	Title      string
	Components []discordgo.MessageComponent
}

type ModalInteraction struct {
	GuildID          string
	ChannelID        string
	UserID           string
	RoleIDs          []string
	HasAdministrator bool
	Name             string
	CustomID         string
	Values           map[string]string
}

type CommandRouter struct {
	commands   map[string]Command
	authorizer CommandAuthorizer
}

type interactionResponder interface {
	InteractionRespond(*discordgo.Interaction, *discordgo.InteractionResponse, ...discordgo.RequestOption) error
}

type interactionResponseEditor interface {
	InteractionResponseEdit(*discordgo.Interaction, *discordgo.WebhookEdit, ...discordgo.RequestOption) (*discordgo.Message, error)
}

type InteractionResponseEditor interface {
	EditInteractionResponse(context.Context, *discordgo.WebhookEdit) error
}

type discordInteractionEditor struct {
	responder   interactionResponseEditor
	interaction *discordgo.Interaction
}

func (e discordInteractionEditor) EditInteractionResponse(ctx context.Context, edit *discordgo.WebhookEdit) error {
	if e.responder == nil || e.interaction == nil {
		return nil
	}
	_, err := e.responder.InteractionResponseEdit(e.interaction, edit)
	return err
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
	if event.Type == discordgo.InteractionModalSubmit {
		return r.handleModalInteraction(ctx, responder, event)
	}
	if event.Type != discordgo.InteractionApplicationCommand {
		return nil
	}

	data := event.ApplicationCommandData()
	command, ok := r.commands[data.Name]
	if !ok {
		return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Command not supported yet."})
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
		Attachments:      interactionAttachments(data.Resolved),
		RoleService:      interactionRoleService(responder),
	}
	requiredCapability := command.RequiredCapability
	if command.RequiredCapabilityFor != nil {
		dynamicCapability := command.RequiredCapabilityFor(interaction)
		if dynamicCapability != "" {
			requiredCapability = dynamicCapability
		} else if requiredCapability == "" {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
	}
	if requiredCapability != "" {
		if r.authorizer == nil {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
		decision, err := r.authorizer.Check(ctx, interaction, requiredCapability)
		if err != nil {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission check failed.", Ephemeral: true})
		}
		if !decision.Allowed {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
	}

	response, err := command.Handle(ctx, interaction)
	if err != nil {
		return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Command failed.", Ephemeral: requiredCapability != ""})
	}
	if strings.TrimSpace(response.Content) == "" && len(response.Embeds) == 0 && !response.Deferred {
		response.Content = "ok"
	}
	return respond(ctx, responder, event.Interaction, response)
}

func (r *CommandRouter) handleModalInteraction(ctx context.Context, responder interactionResponder, event *discordgo.InteractionCreate) error {
	data := event.ModalSubmitData()
	command, ok := r.commandForModal(data.CustomID)
	if !ok {
		return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Interaction not supported yet.", Ephemeral: true})
	}

	interaction := ModalInteraction{
		GuildID:          event.GuildID,
		ChannelID:        event.ChannelID,
		UserID:           interactionUserID(event.Interaction),
		RoleIDs:          interactionRoleIDs(event.Interaction),
		HasAdministrator: interactionHasAdministrator(event.Interaction),
		Name:             command.Name,
		CustomID:         data.CustomID,
		Values:           modalValues(data.Components),
	}
	requiredCapability := command.RequiredCapability
	if command.RequiredCapabilityForModal != nil {
		dynamicCapability := command.RequiredCapabilityForModal(interaction)
		if dynamicCapability != "" {
			requiredCapability = dynamicCapability
		} else if requiredCapability == "" {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
	}
	if requiredCapability != "" {
		if r.authorizer == nil {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
		decision, err := r.authorizer.Check(ctx, Interaction{
			GuildID:          interaction.GuildID,
			ChannelID:        interaction.ChannelID,
			UserID:           interaction.UserID,
			RoleIDs:          interaction.RoleIDs,
			HasAdministrator: interaction.HasAdministrator,
			Name:             interaction.Name,
		}, requiredCapability)
		if err != nil {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission check failed.", Ephemeral: true})
		}
		if !decision.Allowed {
			return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Permission denied.", Ephemeral: true})
		}
	}

	response, err := command.HandleModal(ctx, interaction)
	if err != nil {
		return respond(ctx, responder, event.Interaction, CommandResponse{Content: "Command failed.", Ephemeral: requiredCapability != ""})
	}
	if strings.TrimSpace(response.Content) == "" && len(response.Embeds) == 0 && !response.Deferred {
		response.Content = "ok"
	}
	return respond(ctx, responder, event.Interaction, response)
}

func (r *CommandRouter) commandForModal(customID string) (Command, bool) {
	for _, command := range r.commands {
		if command.HandleModal == nil {
			continue
		}
		for _, prefix := range command.ModalCustomIDPrefixes {
			if strings.HasPrefix(customID, prefix) {
				return command, true
			}
		}
	}
	return Command{}, false
}

func respond(ctx context.Context, responder interactionResponder, interaction *discordgo.Interaction, response CommandResponse) error {
	if response.Modal != nil {
		return responder.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID:   response.Modal.CustomID,
				Title:      response.Modal.Title,
				Components: response.Modal.Components,
			},
		})
	}
	data := &discordgo.InteractionResponseData{Content: response.Content, Embeds: response.Embeds}
	if response.Ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	responseType := discordgo.InteractionResponseChannelMessageWithSource
	if response.Deferred {
		responseType = discordgo.InteractionResponseDeferredChannelMessageWithSource
	}
	if err := responder.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: data,
	}); err != nil {
		return err
	}
	if response.AfterRespond != nil {
		editor, _ := responder.(interactionResponseEditor)
		go response.AfterRespond(ctx, discordInteractionEditor{responder: editor, interaction: interaction})
	}
	return nil
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

func interactionAttachments(resolved *discordgo.ApplicationCommandInteractionDataResolved) map[string]InteractionAttachment {
	if resolved == nil || len(resolved.Attachments) == 0 {
		return nil
	}
	attachments := make(map[string]InteractionAttachment, len(resolved.Attachments))
	for id, attachment := range resolved.Attachments {
		if attachment == nil {
			continue
		}
		attachments[id] = InteractionAttachment{
			ID:          attachment.ID,
			URL:         attachment.URL,
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			Size:        attachment.Size,
		}
	}
	return attachments
}

func modalValues(components []discordgo.MessageComponent) map[string]string {
	values := make(map[string]string)
	for _, component := range components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, nested := range row.Components {
			input, ok := nested.(*discordgo.TextInput)
			if !ok {
				continue
			}
			values[input.CustomID] = strings.TrimSpace(input.Value)
		}
	}
	return values
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
