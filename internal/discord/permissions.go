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
	GrantRoleCapabilities(ctx context.Context, guildID string, roleID string, caps []capability.Capability, actorID string) error
	RevokeRoleCapabilities(ctx context.Context, guildID string, roleID string, caps []capability.Capability) error
	GrantUser(ctx context.Context, guildID string, userID string, cap capability.Capability, actorID string) error
	RevokeUser(ctx context.Context, guildID string, userID string, cap capability.Capability) error
}

type GuildRoleService interface {
	GuildRoleCreate(guildID string, data *discordgo.RoleParams, options ...discordgo.RequestOption) (*discordgo.Role, error)
	GuildMemberRoleAdd(guildID string, userID string, roleID string, options ...discordgo.RequestOption) error
	GuildMemberRoleRemove(guildID string, userID string, roleID string, options ...discordgo.RequestOption) error
}

func PermissionCommands(manager CapabilityGrantManager, roles GuildRoleService, recorder AuditRecorder) []Command {
	return []Command{{
		Name:               "permissions",
		Description:        "Manage Gigi capability grants.",
		RequiredCapability: capability.CapabilityManage,
		Options: []*discordgo.ApplicationCommandOption{
			permissionRoleGroup(),
			permissionUserGroup(),
		},
		Handle: permissionHandler(manager, roles, recorder),
	}}
}

func permissionRoleGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "role",
		Description: "Create, assign, and grant Gigi capabilities to Discord roles.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "create",
				Description: "Create a Discord role and grant a Gigi preset.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("name", "Role name.", nil),
					presetOption(),
				},
			},
			roleUserSubcommand("assign", "Assign a Discord role to a user."),
			roleUserSubcommand("unassign", "Remove a Discord role from a user."),
			roleCapabilitySubcommand("grant", "Grant a capability to a Discord role."),
			roleCapabilitySubcommand("revoke", "Revoke a capability from a Discord role."),
			rolePresetSubcommand("grant-preset", "Grant a preset to a Discord role."),
			rolePresetSubcommand("revoke-preset", "Revoke a preset from a Discord role."),
		},
	}
}

func permissionUserGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "user",
		Description: "Manage direct user capability exceptions.",
		Options: []*discordgo.ApplicationCommandOption{
			userCapabilitySubcommand("grant", "Grant a direct capability exception to a user."),
			userCapabilitySubcommand("revoke", "Revoke a direct capability exception from a user."),
		},
	}
}

func roleUserSubcommand(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Discord role.", Required: true},
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Discord user.", Required: true},
		},
	}
}

func roleCapabilitySubcommand(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Discord role.", Required: true},
			capabilityOption(),
		},
	}
}

func rolePresetSubcommand(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Discord role.", Required: true},
			presetOption(),
		},
	}
}

func userCapabilitySubcommand(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Discord user.", Required: true},
			capabilityOption(),
		},
	}
}

func stringOption(name string, description string, choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        name,
		Description: description,
		Required:    true,
		Choices:     choices,
	}
}

func capabilityOption() *discordgo.ApplicationCommandOption {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(capability.KnownCapabilities()))
	for _, cap := range capability.KnownCapabilities() {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: string(cap), Value: string(cap)})
	}
	return stringOption("capability", "Gigi capability.", choices)
}

func presetOption() *discordgo.ApplicationCommandOption {
	presets := capability.KnownPresets()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(presets))
	for _, preset := range presets {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: preset.Name, Value: preset.Name})
	}
	return stringOption("preset", "Gigi capability preset.", choices)
}

func permissionHandler(manager CapabilityGrantManager, roles GuildRoleService, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if manager == nil {
			return CommandResponse{}, fmt.Errorf("capability grant manager is required")
		}
		request, err := parsePermissionRequest(interaction)
		if err != nil {
			_ = recordPermissionChange(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}

		roleService := interaction.RoleService
		if roleService == nil {
			roleService = roles
		}
		response, err := executePermissionRequest(ctx, manager, roleService, interaction, request)
		if err != nil {
			_ = recordPermissionChange(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: cleanPermissionError(request, err), Ephemeral: true}, nil
		}
		if err := recordPermissionChange(ctx, recorder, interaction, request, audit.StatusSucceeded, nil); err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Content: response, Ephemeral: true}, nil
	}
}

func executePermissionRequest(ctx context.Context, manager CapabilityGrantManager, roles GuildRoleService, interaction Interaction, request permissionRequest) (string, error) {
	switch {
	case request.Group == "role" && request.Action == "create":
		if roles == nil {
			return "", fmt.Errorf("discord role service is required")
		}
		role, err := roles.GuildRoleCreate(interaction.GuildID, &discordgo.RoleParams{Name: request.RoleName})
		if err != nil {
			return "", fmt.Errorf("discord role operation: %w", err)
		}
		if err := manager.GrantRoleCapabilities(ctx, interaction.GuildID, role.ID, request.Capabilities, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Created role `%s` and granted preset `%s`.", role.Name, request.Preset), nil
	case request.Group == "role" && request.Action == "assign":
		if roles == nil {
			return "", fmt.Errorf("discord role service is required")
		}
		if err := roles.GuildMemberRoleAdd(interaction.GuildID, request.UserID, request.RoleID); err != nil {
			return "", fmt.Errorf("discord role operation: %w", err)
		}
		return fmt.Sprintf("Assigned role `%s` to user `%s`.", request.RoleID, request.UserID), nil
	case request.Group == "role" && request.Action == "unassign":
		if roles == nil {
			return "", fmt.Errorf("discord role service is required")
		}
		if err := roles.GuildMemberRoleRemove(interaction.GuildID, request.UserID, request.RoleID); err != nil {
			return "", fmt.Errorf("discord role operation: %w", err)
		}
		return fmt.Sprintf("Unassigned role `%s` from user `%s`.", request.RoleID, request.UserID), nil
	case request.Group == "role" && request.Action == "grant":
		if err := manager.GrantRole(ctx, interaction.GuildID, request.RoleID, request.Capability, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Granted `%s` for role `%s`.", request.Capability, request.RoleID), nil
	case request.Group == "role" && request.Action == "revoke":
		if err := manager.RevokeRole(ctx, interaction.GuildID, request.RoleID, request.Capability); err != nil {
			return "", err
		}
		return fmt.Sprintf("Revoked `%s` for role `%s`.", request.Capability, request.RoleID), nil
	case request.Group == "role" && request.Action == "grant-preset":
		if err := manager.GrantRoleCapabilities(ctx, interaction.GuildID, request.RoleID, request.Capabilities, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Granted preset `%s` for role `%s`.", request.Preset, request.RoleID), nil
	case request.Group == "role" && request.Action == "revoke-preset":
		if err := manager.RevokeRoleCapabilities(ctx, interaction.GuildID, request.RoleID, request.Capabilities); err != nil {
			return "", err
		}
		return fmt.Sprintf("Revoked preset `%s` for role `%s`.", request.Preset, request.RoleID), nil
	case request.Group == "user" && request.Action == "grant":
		if err := manager.GrantUser(ctx, interaction.GuildID, request.UserID, request.Capability, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Granted `%s` for user `%s`.", request.Capability, request.UserID), nil
	case request.Group == "user" && request.Action == "revoke":
		if err := manager.RevokeUser(ctx, interaction.GuildID, request.UserID, request.Capability); err != nil {
			return "", err
		}
		return fmt.Sprintf("Revoked `%s` for user `%s`.", request.Capability, request.UserID), nil
	default:
		return "", fmt.Errorf("unsupported permissions action")
	}
}

type permissionRequest struct {
	Group        string
	Action       string
	RoleID       string
	RoleName     string
	UserID       string
	Preset       string
	Capability   capability.Capability
	Capabilities []capability.Capability
}

func parsePermissionRequest(interaction Interaction) (permissionRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return permissionRequest{}, fmt.Errorf("Permissions can only be changed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return permissionRequest{}, fmt.Errorf("Choose one permissions group.")
	}
	group := interaction.Options[0]
	if len(group.Options) != 1 {
		return permissionRequest{Group: group.Name}, fmt.Errorf("Choose one permissions action.")
	}
	action := group.Options[0]
	request := permissionRequest{Group: group.Name, Action: action.Name}

	switch request.Group {
	case "role":
		return parseRolePermissionRequest(request, action.Options)
	case "user":
		return parseUserPermissionRequest(request, action.Options)
	default:
		return request, fmt.Errorf("Unsupported permissions group.")
	}
}

func parseRolePermissionRequest(request permissionRequest, options []InteractionOption) (permissionRequest, error) {
	switch request.Action {
	case "create":
		request.RoleName = strings.TrimSpace(optionByName(options, "name"))
		if request.RoleName == "" {
			return request, fmt.Errorf("Role name is required.")
		}
		if len(request.RoleName) > 100 {
			return request, fmt.Errorf("Role name is too long.")
		}
		return parsePreset(request, options)
	case "assign", "unassign":
		request.RoleID = optionByName(options, "role")
		request.UserID = optionByName(options, "user")
		return validateRoleUserTargets(request)
	case "grant", "revoke":
		request.RoleID = optionByName(options, "role")
		if strings.TrimSpace(request.RoleID) == "" {
			return request, fmt.Errorf("Role is required.")
		}
		return parseCapability(request, options)
	case "grant-preset", "revoke-preset":
		request.RoleID = optionByName(options, "role")
		if strings.TrimSpace(request.RoleID) == "" {
			return request, fmt.Errorf("Role is required.")
		}
		return parsePreset(request, options)
	default:
		return request, fmt.Errorf("Unsupported role permissions action.")
	}
}

func parseUserPermissionRequest(request permissionRequest, options []InteractionOption) (permissionRequest, error) {
	switch request.Action {
	case "grant", "revoke":
		request.UserID = optionByName(options, "user")
		if strings.TrimSpace(request.UserID) == "" {
			return request, fmt.Errorf("User is required.")
		}
		return parseCapability(request, options)
	default:
		return request, fmt.Errorf("Unsupported user permissions action.")
	}
}

func parseCapability(request permissionRequest, options []InteractionOption) (permissionRequest, error) {
	capabilityValue := optionByName(options, "capability")
	capabilityName, err := capability.Normalize(capabilityValue)
	if err != nil {
		return request, err
	}
	request.Capability = capabilityName
	return request, nil
}

func parsePreset(request permissionRequest, options []InteractionOption) (permissionRequest, error) {
	preset := optionByName(options, "preset")
	caps, ok := capability.PresetCapabilities(preset)
	if !ok {
		return request, fmt.Errorf("Unknown permission preset.")
	}
	request.Preset = preset
	request.Capabilities = caps
	return request, nil
}

func validateRoleUserTargets(request permissionRequest) (permissionRequest, error) {
	if strings.TrimSpace(request.RoleID) == "" {
		return request, fmt.Errorf("Role is required.")
	}
	if strings.TrimSpace(request.UserID) == "" {
		return request, fmt.Errorf("User is required.")
	}
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

func cleanPermissionError(request permissionRequest, err error) string {
	if request.Group == "role" && (request.Action == "create" || request.Action == "assign" || request.Action == "unassign") {
		return "Could not update Discord role. Check Gigi has Manage Roles and role hierarchy is high enough."
	}
	return "Permission change failed."
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
		"group":   request.Group,
		"action":  request.Action,
	}
	if request.Capability != "" {
		metadata["capability"] = string(request.Capability)
	}
	if request.Preset != "" {
		metadata["preset"] = request.Preset
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
