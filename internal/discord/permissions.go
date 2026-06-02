package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
)

type CapabilityGrantManager interface {
	GrantRole(ctx context.Context, guildID string, roleID string, cap capability.Capability, actorID string) error
	RevokeRole(ctx context.Context, guildID string, roleID string, cap capability.Capability) error
	GrantUser(ctx context.Context, guildID string, userID string, cap capability.Capability, actorID string) error
	RevokeUser(ctx context.Context, guildID string, userID string, cap capability.Capability) error
}

func PermissionCommands(manager CapabilityGrantManager, recorder AuditRecorder) []Command {
	return []Command{{
		Name:               "permissions",
		Description:        "Manage Gigi capability grants.",
		RequiredCapability: capability.CapabilityManage,
		Options: []*discordgo.ApplicationCommandOption{
			permissionSubcommand("grant-role", "Grant a capability to a Discord role.", "role", discordgo.ApplicationCommandOptionRole),
			permissionSubcommand("revoke-role", "Revoke a capability from a Discord role.", "role", discordgo.ApplicationCommandOptionRole),
			permissionSubcommand("grant-user", "Grant a capability to a Discord user.", "user", discordgo.ApplicationCommandOptionUser),
			permissionSubcommand("revoke-user", "Revoke a capability from a Discord user.", "user", discordgo.ApplicationCommandOptionUser),
		},
		Handle: permissionHandler(manager, recorder),
	}}
}

func permissionSubcommand(name string, description string, targetName string, targetType discordgo.ApplicationCommandOptionType) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        targetType,
				Name:        targetName,
				Description: "Target Discord " + targetName + ".",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "capability",
				Description: "Capability name, for example plugin.install.",
				Required:    true,
			},
		},
	}
}

func permissionHandler(manager CapabilityGrantManager, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if manager == nil {
			return CommandResponse{}, fmt.Errorf("capability grant manager is required")
		}
		request, err := parsePermissionRequest(interaction)
		if err != nil {
			_ = recordPermissionChange(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}

		switch request.Action {
		case "grant-role":
			err = manager.GrantRole(ctx, interaction.GuildID, request.TargetID, request.Capability, interaction.UserID)
		case "revoke-role":
			err = manager.RevokeRole(ctx, interaction.GuildID, request.TargetID, request.Capability)
		case "grant-user":
			err = manager.GrantUser(ctx, interaction.GuildID, request.TargetID, request.Capability, interaction.UserID)
		case "revoke-user":
			err = manager.RevokeUser(ctx, interaction.GuildID, request.TargetID, request.Capability)
		default:
			err = fmt.Errorf("unsupported permissions action")
		}
		if err != nil {
			_ = recordPermissionChange(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{}, err
		}
		if err := recordPermissionChange(ctx, recorder, interaction, request, audit.StatusSucceeded, nil); err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Content: permissionSuccessMessage(request), Ephemeral: true}, nil
	}
}

type permissionRequest struct {
	Action     string
	TargetKind string
	TargetID   string
	Capability capability.Capability
}

func parsePermissionRequest(interaction Interaction) (permissionRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return permissionRequest{}, fmt.Errorf("Permissions can only be changed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return permissionRequest{}, fmt.Errorf("Choose one permissions action.")
	}
	subcommand := interaction.Options[0]
	request := permissionRequest{Action: subcommand.Name}
	switch subcommand.Name {
	case "grant-role", "revoke-role":
		request.TargetKind = "role"
		request.TargetID = optionByName(subcommand.Options, "role")
	case "grant-user", "revoke-user":
		request.TargetKind = "user"
		request.TargetID = optionByName(subcommand.Options, "user")
	default:
		return request, fmt.Errorf("Unsupported permissions action.")
	}
	if strings.TrimSpace(request.TargetID) == "" {
		return request, fmt.Errorf("Target is required.")
	}
	capabilityValue := optionByName(subcommand.Options, "capability")
	capabilityName, err := capability.Normalize(capabilityValue)
	if err != nil {
		return request, err
	}
	request.Capability = capabilityName
	return request, nil
}

func optionByName(options []InteractionOption, name string) string {
	for _, option := range options {
		if option.Name == name {
			return strings.TrimSpace(option.Value)
		}
	}
	return ""
}

func permissionSuccessMessage(request permissionRequest) string {
	verb := "Granted"
	if strings.HasPrefix(request.Action, "revoke-") {
		verb = "Revoked"
	}
	return fmt.Sprintf("%s `%s` for %s `%s`.", verb, request.Capability, request.TargetKind, request.TargetID)
}

func recordPermissionChange(ctx context.Context, recorder AuditRecorder, interaction Interaction, request permissionRequest, status audit.Status, err error) error {
	if recorder == nil || strings.TrimSpace(interaction.UserID) == "" {
		return nil
	}
	reason := ""
	if err != nil {
		reason = "permission_change_failed"
	}
	metadata := map[string]string{
		"command": interaction.Name,
		"action":  request.Action,
	}
	if request.Capability != "" {
		metadata["capability"] = string(request.Capability)
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.permissions.change",
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}
